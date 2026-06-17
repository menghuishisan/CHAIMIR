// sim service 文件定义服务依赖注入和通用业务编排,不接收数据库连接。
package sim

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"
)

const (
	simModuleName         = "sim"
	simBundleResourceType = "package-bundle"
	shareCodeLength       = 18
)

// objectStorage 描述 M4 写入仿真包 bundle 所需的对象存储能力。
type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	BucketCode() string
}

// fileService 描述 M4 复用统一文件服务所需能力。
type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
	IssueDownloadGrant(req storage.IssueDownloadGrantRequest) (string, storage.DownloadGrant, error)
}

// BackendAdapter 是 M4 自有后端计算适配器,不得调用 M2 模块内部实现。
type BackendAdapter interface {
	// Serve 在已鉴权的 WebSocket 上执行后端计算协议。
	Serve(ctx context.Context, session SessionWithPackage, conn BackendConn) error
	// Release 回收指定后端计算会话占用的适配器资源。
	Release(ctx context.Context, session SessionWithPackage) error
}

// BackendConn 是 compute=backend 适配器可使用的受控连接能力。
type BackendConn interface {
	ReadJSON(v any) error
	SendJSON(v any) error
}

// BackendRegistry 保存 compute=backend 可用适配器。
type BackendRegistry map[string]BackendAdapter

// Service 承载 sim 模块业务编排,依赖 repo 接口和平台横切能力。
type Service struct {
	store    Store
	ids      snowflake.Generator
	upload   config.UploadConfig
	storage  objectStorage
	files    fileService
	audit    audit.Writer
	identity contracts.IdentityService
	wsHub    *ws.Hub
	backends BackendRegistry
}

// ServiceDeps 是 sim service 的装配依赖集合。
type ServiceDeps struct {
	Store           Store
	IDs             snowflake.Generator
	Upload          config.UploadConfig
	Storage         *storage.Storage
	FileService     storage.Service
	Audit           audit.Writer
	Identity        contracts.IdentityService
	WSHub           *ws.Hub
	BackendAdapters BackendRegistry
}

// NewService 构造 sim 服务,不接收数据库连接,由装配层传入 Store。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("sim service 缺少 store")
	}
	if deps.IDs == nil {
		return nil, fmt.Errorf("sim service 缺少 ID 生成器")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("sim service 缺少统一对象存储")
	}
	if deps.FileService.DownloadGrantTTL <= 0 {
		return nil, fmt.Errorf("sim service 缺少统一文件服务配置")
	}
	if deps.Audit == nil {
		return nil, fmt.Errorf("sim service 缺少审计写入器")
	}
	if deps.Identity == nil {
		return nil, fmt.Errorf("sim service 缺少身份读取契约")
	}
	if deps.BackendAdapters == nil {
		deps.BackendAdapters = BackendRegistry{}
	}
	return &Service{store: deps.Store, ids: deps.IDs, upload: deps.Upload, storage: deps.Storage, files: deps.FileService, audit: deps.Audit, identity: deps.Identity, wsHub: deps.WSHub, backends: deps.BackendAdapters}, nil
}

// IssueBundleDownloadGrant 为已上架仿真包签发短时下载授权,让对象下载走统一文件服务边界。
func (s *Service) IssueBundleDownloadGrant(ctx context.Context, accountID int64, code, version string) (BundleDownloadGrantDTO, error) {
	if accountID <= 0 {
		return BundleDownloadGrantDTO{}, apperr.ErrUnauthorized
	}
	pkg, err := s.loadPackage(ctx, code, version)
	if err != nil {
		return BundleDownloadGrantDTO{}, err
	}
	if pkg.Status != PackageStatusPublished {
		return BundleDownloadGrantDTO{}, apperr.ErrSimPackageUnavailable
	}
	ref, err := storage.ParseObjectRef(pkg.BundleKey)
	if err != nil {
		return BundleDownloadGrantDTO{}, apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	ownerTenantID, err := tenantIDFromBundleKey(ref.Key)
	if err != nil {
		return BundleDownloadGrantDTO{}, err
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:     ownerTenantID,
		AccountID:    accountID,
		ObjectRef:    pkg.BundleKey,
		Module:       simModuleName,
		ResourceType: simBundleResourceType,
		ResourceID:   ids.Format(pkg.ID),
	})
	if err != nil {
		return BundleDownloadGrantDTO{}, apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	return BundleDownloadGrantDTO{Token: token, BundleHash: pkg.BundleHash, ExpiresAt: grant.ExpiresAt.Format(time.RFC3339)}, nil
}

// tenantIDFromBundleKey 从统一对象 key 的首段解析上传租户,用于全局包的下载授权边界。
func tenantIDFromBundleKey(key string) (int64, error) {
	parts := strings.Split(strings.TrimSpace(key), "/")
	if len(parts) < 4 || parts[1] != simModuleName || parts[2] != simBundleResourceType {
		return 0, apperr.ErrSimBundleUnreadable.WithCause(fmt.Errorf("仿真包对象 key 结构异常: key=%q", key))
	}
	tenantID, ok := ids.Parse(parts[0])
	if !ok {
		return 0, apperr.ErrSimBundleUnreadable.WithCause(fmt.Errorf("仿真包对象 key 租户段异常: key=%q", key))
	}
	return tenantID, nil
}

// storeBundle 通过统一文件服务执行扫描并规划对象引用。
func (s *Service) storeBundle(ctx context.Context, tenantID, accountID, packageID int64, input BundleInput, req SubmitPackageRequest, compute int16) (string, string, ValidationReport, InteractionSchema, CodeTraceAudit, error) {
	limits := upload.ArchiveLimits{MaxFiles: s.upload.SimBundleMaxFiles, MaxUnpackedBytes: s.upload.SimBundleMaxUnpackedBytes}
	bundleHash, staticScan, manifest, err := analyzeBundle(input, limits)
	if err != nil {
		return "", "", ValidationReport{}, InteractionSchema{}, CodeTraceAudit{}, err
	}
	report := ValidationReport{BundleHash: bundleHash, MetadataValidation: ValidationStatus{Status: validationPassed}, StaticScan: staticScan}
	if staticScan.Status != validationPassed {
		return "", bundleHash, report, InteractionSchema{}, CodeTraceAudit{}, apperr.ErrSimPackageValidationFailed
	}
	if err := validateBundleManifestMatchesRequest(manifest, req, compute); err != nil {
		return "", bundleHash, report, InteractionSchema{}, CodeTraceAudit{}, err
	}
	plan, err := s.planBundleObject(ctx, tenantID, accountID, packageID, input)
	if err != nil {
		return "", bundleHash, report, InteractionSchema{}, CodeTraceAudit{}, err
	}
	return plan.ObjectRef, bundleHash, report, manifest.InteractionSchema, manifest.CodeTrace, nil
}

// planBundleObject 规划 bundle 对象引用,调用方在数据库侧前置校验后再执行实际上传。
func (s *Service) planBundleObject(ctx context.Context, tenantID, accountID, packageID int64, input BundleInput) (storage.UploadPlan, error) {
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:        tenantID,
		AccountID:       accountID,
		Module:          simModuleName,
		ResourceType:    simBundleResourceType,
		ResourceID:      ids.Format(packageID),
		FileName:        input.FileName,
		ContentType:     input.ContentType,
		Size:            int64(len(input.Data)),
		MaxBytes:        s.upload.SimBundleMaxBytes,
		ExpectedBucket:  s.storage.BucketCode(),
		AllowedFileName: true,
		Content:         input.Data,
		KindValidator:   simBundleKind,
		ScanPolicy:      upload.ScanPolicy{Required: s.upload.VirusScanRequired},
	})
	if err != nil {
		return storage.UploadPlan{}, apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	return plan, nil
}

// uploadBundleObject 写入已规划的 bundle 对象。
func (s *Service) uploadBundleObject(ctx context.Context, plan storage.UploadPlan, input BundleInput) error {
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(input.Data), int64(len(input.Data)), input.ContentType); err != nil {
		return apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	return nil
}

// uploadPlannedBundle 解析统一对象引用并写入已被数据库接受的 bundle。
func (s *Service) uploadPlannedBundle(ctx context.Context, objectRef string, input BundleInput) error {
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		return apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	return s.uploadBundleObject(ctx, storage.UploadPlan{Bucket: ref.Bucket, Key: ref.Key}, input)
}

// simBundleKind 校验仿真包只能是 ZIP/TAR 归档。
func simBundleKind(fileName, _ string, content []byte) bool {
	_, err := upload.DetectArchiveFormat(fileName, content)
	return err == nil
}

// newShareCode 生成不可从 session_id 推导的全局分享码。
func newShareCode() (string, error) {
	return pkgcrypto.RandomToken(shareCodeLength)
}

// trimMapStrings 清理动态报告 details,避免无意义空 key/value 入库。
func trimMapStrings(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		key := strings.TrimSpace(k)
		value := strings.TrimSpace(v)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

// lookupError 保留仓储层已经归类好的应用错误,无记录时走 not found,其他底层错误走查询失败。
func lookupError(err error, notFound, queryFailed *apperr.Error) error {
	if err == nil {
		return nil
	}
	if ae, ok := apperr.As(err); ok {
		return ae
	}
	if isNoRows(err) && notFound != nil {
		return notFound.WithCause(err)
	}
	if queryFailed != nil {
		return queryFailed.WithCause(err)
	}
	if notFound != nil {
		return notFound.WithCause(err)
	}
	return apperr.ErrInternal.WithCause(err)
}
