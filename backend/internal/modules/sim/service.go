// sim service 文件定义服务依赖注入和通用业务编排,不接收数据库连接。
package sim

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

const (
	simModuleName         = "sim"
	simBundleResourceType = "package-bundle"
	shareCodeLength       = 18
)

// objectStorage 描述 M4 写入与读取仿真包 bundle 所需的对象存储能力。
type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	BucketCode() string
}

// fileService 描述 M4 复用统一文件服务所需能力。
type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
}

// BackendAdapter 是 M4 自有隔离执行适配器,不得调用 M2 模块内部实现。
type BackendAdapter interface {
	// Descriptor 返回前端可安全展示的能力信息,不得暴露镜像或集群细节。
	Descriptor() BackendAdapterDescriptor
	// ValidateConfig 在包进入审核前校验能力专属配置。
	ValidateConfig(config map[string]any) error
	// Serve 在已鉴权的 WebSocket 上执行隔离执行协议;bundle 为 nil 时表示算法固化在镜像内。
	// recorded 是会话已登记的用户操作,连接建立时先在容器内重放到那个位置。
	Serve(ctx context.Context, session SessionWithPackage, bundle *ExecutionBundle, recorded []Action, conn BackendConn) error
	// Preview 在隔离容器内做上架前预览:同 seed 双跑比对确定性,并渲出样例教学帧。
	Preview(ctx context.Context, pkg Package, bundle *ExecutionBundle, frameCount int) (PreviewResult, error)
	// Release 回收指定会话占用的隔离资源。
	Release(ctx context.Context, session SessionWithPackage) error
}

// ExecutionBundle 是投递给隔离容器的归档正文与校验信息。
//
// bundle 不经网络下发容器:计算 Pod 根文件系统只读且网络 deny-all,容器既写不下也取不到
// 对象存储正文;放开网络等于拆掉隔离边界。字节由后端取出后经 k8s exec stdin 推入,
// 容器内先校验 sha256 与 Hash 一致再按 Entry 装配(见 docs/总-镜像与容器设计.md §六之一)。
type ExecutionBundle struct {
	Data   []byte
	Hash   string
	Entry  string
	Format string
}

// PreviewResult 是隔离预览的产出:确定性结论与样例教学帧。
type PreviewResult struct {
	DeterminismPassed bool
	Detail            string
	Frames            []BackendSnapshot
}

// BackendConn 是隔离执行适配器可使用的受控连接能力。
//
// 刻意不暴露通用 ReadJSON/SendJSON:适配器只能读到已过交互白名单的受控命令,
// 只能写出协议校验过的教学帧,不得借连接把任意结构透传给浏览器。
type BackendConn interface {
	// ReadCommand 读取浏览器下发的下一条受控命令。
	ReadCommand() (BackendCommand, error)
	// RecordExecuted 在容器执行成功后登记这条命令的效果;时刻推进不入记录。
	RecordExecuted(command BackendCommand) error
	// SendFrame 推送一帧教学快照(首帧附带包自描述信息)。
	SendFrame(message BackendStreamMessage) error
}

// BackendRegistry 保存已装配的隔离执行能力。
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
	// packageRunnerCode 是扩展包通用运行器能力编号,服务端按作者类型自动绑定给教师/第三方包。
	packageRunnerCode string
	// previewFrameCount 是隔离预览渲制的样例帧数量,供平台管理员判断算法实现是否正确。
	previewFrameCount int
	// previewBatchSize 是隔离预览任务单轮认领的待审包数量上限。
	previewBatchSize int
	// maxIsolatedSessionsPerTenant 是单租户同时活跃的隔离执行会话上限;一个会话一个 Pod。
	maxIsolatedSessionsPerTenant int
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
	SimBackend      config.SimBackendConfig
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
		return nil, fmt.Errorf("sim service 缺少隔离执行能力注册表")
	}
	for code, adapter := range deps.BackendAdapters {
		if adapter == nil {
			return nil, fmt.Errorf("sim backend adapter %q 不能为空", code)
		}
		descriptor := adapter.Descriptor()
		if strings.TrimSpace(code) == "" || strings.TrimSpace(descriptor.Code) != strings.TrimSpace(code) || strings.TrimSpace(descriptor.Name) == "" || strings.TrimSpace(descriptor.Protocol) == "" {
			return nil, fmt.Errorf("sim backend adapter %q 描述无效", code)
		}
	}
	// 扩展包运行器必须真实注册:它承载全部教师/第三方包,缺了就等于扩展接入整条链路不可用,
	// 而那必须在启动时暴露,不能等到教师提交后才以运行时错误的形式出现。
	runnerCode := strings.TrimSpace(deps.SimBackend.PackageRunnerAdapterCode)
	if runnerCode == "" {
		return nil, fmt.Errorf("sim service 缺少 SIM_PACKAGE_RUNNER_ADAPTER_CODE")
	}
	if deps.BackendAdapters[runnerCode] == nil {
		return nil, fmt.Errorf("sim service 扩展包运行器能力 %q 未在 SIM_BACKEND_STDIO_ADAPTERS_JSON 中注册", runnerCode)
	}
	if deps.SimBackend.PreviewFrameCount <= 0 || deps.SimBackend.PreviewFrameCount > maxPreviewFrames {
		return nil, fmt.Errorf("SIM_PREVIEW_FRAME_COUNT 必须在 1 到 %d 之间", maxPreviewFrames)
	}
	if deps.SimBackend.PreviewBatchSize <= 0 {
		return nil, fmt.Errorf("SIM_PREVIEW_BATCH_SIZE 必须大于 0")
	}
	if deps.SimBackend.MaxConcurrentSessionsPerTenant <= 0 {
		return nil, fmt.Errorf("SIM_BACKEND_MAX_CONCURRENT_SESSIONS_PER_TENANT 必须大于 0")
	}
	return &Service{
		store: deps.Store, ids: deps.IDs, upload: deps.Upload, storage: deps.Storage,
		files: deps.FileService, audit: deps.Audit, identity: deps.Identity, wsHub: deps.WSHub,
		backends: deps.BackendAdapters, packageRunnerCode: runnerCode,
		previewFrameCount:            deps.SimBackend.PreviewFrameCount,
		previewBatchSize:             deps.SimBackend.PreviewBatchSize,
		maxIsolatedSessionsPerTenant: deps.SimBackend.MaxConcurrentSessionsPerTenant,
	}, nil
}

// ensurePackageRunnerAvailable 在提交与更新入口确认扩展包运行器仍然可用。
func (s *Service) ensurePackageRunnerAvailable() error {
	if s.backends[s.packageRunnerCode] == nil {
		return apperr.ErrSimBackendComputeUnavailable
	}
	return nil
}

// loadBundleForExecution 取出会话所引用包的归档正文,交隔离容器装配。
// 内置包不走这条路径:它们在浏览器 Worker 内按 code 装配,没有归档字节。
func (s *Service) loadBundleForExecution(ctx context.Context, session SessionWithPackage) (*ExecutionBundle, error) {
	if strings.TrimSpace(session.Entry) == "" {
		// 算法固化在镜像内的重计算能力没有归档,交 nil 表示"无需投递 bundle"。
		return nil, nil
	}
	ref, err := storage.ParseObjectRef(session.BundleKey)
	if err != nil {
		return nil, apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	reader, err := s.storage.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return nil, apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			logging.ErrorContext(ctx, "关闭仿真包对象读取器失败", closeErr.Error(), slog.String("object_key", ref.Key))
		}
	}()
	data, err := io.ReadAll(io.LimitReader(reader, s.upload.SimBundleMaxBytes+1))
	if err != nil {
		return nil, apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	if int64(len(data)) > s.upload.SimBundleMaxBytes {
		return nil, apperr.ErrSimBundleUnreadable.WithCause(fmt.Errorf("仿真包归档超过上传上限"))
	}
	format, err := bundleFormat(data)
	if err != nil {
		return nil, err
	}
	return &ExecutionBundle{Data: data, Hash: session.BundleHash, Entry: session.Entry, Format: format}, nil
}

// bundleFormat 判定归档格式,只接受与上传边界一致的 ZIP/TAR。
func bundleFormat(data []byte) (string, error) {
	detected, err := upload.DetectArchiveFormat("bundle", data)
	if err != nil {
		return "", apperr.ErrSimBundleUnreadable.WithCause(err)
	}
	switch detected {
	case upload.ArchiveFormatZIP:
		return "zip", nil
	case upload.ArchiveFormatTAR:
		return "tar", nil
	default:
		return "", apperr.ErrSimBundleUnreadable.WithCause(fmt.Errorf("仿真包归档格式不支持"))
	}
}

// storedBundle 汇总一次 bundle 上传规划的结果,避免多返回值堆叠到六个。
type storedBundle struct {
	ObjectRef         string
	BundleHash        string
	Entry             string
	Report            ValidationReport
	InteractionSchema InteractionSchema
	CodeTrace         CodeTraceAudit
}

// storeBundle 执行归档校验与静态扫描,并规划对象引用。
//
// 静态扫描命中不再阻断进入审核:它是给审核人的信号而非隔离边界(见 07 安全设计 §5),
// 真正的边界是隔离容器。命中项写进报告,由平台管理员在看样例帧时一并重点查看。
// 只有归档结构、manifest 协议或表单一致性问题才拒绝入库 —— 那些是包本身不可运行。
func (s *Service) storeBundle(ctx context.Context, tenantID, accountID, packageID int64, input BundleInput, req SubmitPackageRequest) (storedBundle, error) {
	limits := upload.ArchiveLimits{MaxFiles: s.upload.SimBundleMaxFiles, MaxUnpackedBytes: s.upload.SimBundleMaxUnpackedBytes}
	bundleHash, staticScan, manifest, err := analyzeBundle(input, limits)
	if err != nil {
		return storedBundle{}, err
	}
	report := ValidationReport{BundleHash: bundleHash, MetadataValidation: ValidationStatus{Status: validationPassed}, StaticScan: staticScan}
	if hasBlockingFindings(staticScan.Findings) {
		return storedBundle{BundleHash: bundleHash, Report: report}, apperr.ErrSimPackageValidationFailed
	}
	if err := validateBundleManifestMatchesRequest(manifest, req); err != nil {
		return storedBundle{BundleHash: bundleHash, Report: report}, err
	}
	plan, err := s.planBundleObject(ctx, tenantID, accountID, packageID, input)
	if err != nil {
		return storedBundle{BundleHash: bundleHash, Report: report}, err
	}
	return storedBundle{
		ObjectRef:         plan.ObjectRef,
		BundleHash:        bundleHash,
		Entry:             strings.TrimSpace(manifest.Meta.Entry),
		Report:            report,
		InteractionSchema: manifest.InteractionSchema,
		CodeTrace:         manifest.CodeTrace,
	}, nil
}

// hasBlockingFindings 判定扫描结果里是否存在"包本身不可运行"的问题。
// manifest: 前缀的命中来自协议解析(结构非法、入口缺失、模式越界),包不可装配故必须拒收;
// 其余命中是危险调用模式,只作为审核信号(容器隔离已覆盖其风险面)。
func hasBlockingFindings(findings []string) bool {
	for _, item := range findings {
		if strings.HasPrefix(item, "manifest:") {
			return true
		}
	}
	return false
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
