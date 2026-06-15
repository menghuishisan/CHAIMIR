// sim service 文件定义服务依赖注入和通用业务编排,不接收数据库连接。
package sim

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

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

// objectStorage 描述 M4 读取和写入仿真包 bundle 所需的对象存储能力。
type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
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
	Serve(ctx context.Context, session SessionWithPackage, conn *ws.Conn) error
	// Release 回收指定后端计算会话占用的适配器资源。
	Release(ctx context.Context, session SessionWithPackage) error
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
	if deps.BackendAdapters == nil {
		deps.BackendAdapters = BackendRegistry{}
	}
	return &Service{store: deps.Store, ids: deps.IDs, upload: deps.Upload, storage: deps.Storage, files: deps.FileService, audit: deps.Audit, wsHub: deps.WSHub, backends: deps.BackendAdapters}, nil
}

// storeBundle 通过统一文件服务规划对象路径、执行扫描并写入对象存储。
func (s *Service) storeBundle(ctx context.Context, tenantID, accountID, packageID int64, input BundleInput) (string, string, ValidationReport, error) {
	limits := upload.ArchiveLimits{MaxFiles: s.upload.SimBundleMaxFiles, MaxUnpackedBytes: s.upload.SimBundleMaxUnpackedBytes}
	bundleHash, staticScan, err := analyzeBundle(input, limits)
	if err != nil {
		return "", "", ValidationReport{}, err
	}
	report := ValidationReport{BundleHash: bundleHash, MetadataValidation: ValidationStatus{Status: validationPassed}, StaticScan: staticScan}
	if staticScan.Status != validationPassed {
		return "", bundleHash, report, apperr.ErrSimPackageValidationFailed
	}
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
		return "", bundleHash, report, apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(input.Data), int64(len(input.Data)), input.ContentType); err != nil {
		return "", bundleHash, report, apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	return plan.ObjectRef, bundleHash, report, nil
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
