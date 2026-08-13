// teaching service 文件定义 M6 服务依赖注入和通用业务编排,不接收数据库连接。
package teaching

import (
	"context"
	"fmt"
	"io"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/transfer"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

const (
	// teachingModuleName 是 M6 在统一文件服务对象路径与授权边界里的模块段。
	teachingModuleName = "teaching"
	// lessonMaterialResourceType 与 courseCoverResourceType 是 M6 两类对象的资源段。
	lessonMaterialResourceType = "material"
	courseCoverResourceType    = "cover"
)

// Service 承载 teaching 模块业务编排,依赖 repo 接口和跨模块 contracts。
type Service struct {
	store              Store
	ids                snowflake.Generator
	audit              audit.Writer
	identity           contracts.IdentityService
	content            contracts.ContentReadService
	judge              contracts.JudgeService
	bus                eventbus.Bus
	transfers          transferService
	storage            objectStorage
	files              fileService
	cfg                config.TeachingConfig
	materialMaxBytes   int64
	coverMaxBytes      int64
	materialScanPolicy upload.ScanPolicy
}

// objectStorage 描述 M6 导出产物写入统一对象存储所需能力。
type objectStorage interface {
	Delete(ctx context.Context, bucket, key string) error
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) (int64, error)
	BucketAttach() string
	BucketReport() string
}

// fileService 描述 M6 复用统一文件服务规划对象路径与签发下载授权所需能力。
type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
	IssueDownloadGrant(req storage.IssueDownloadGrantRequest) (string, storage.DownloadGrant, error)
}

// transferService 描述 M6 调用统一导入导出中心所需能力。
type transferService interface {
	CreateTask(context.Context, transfer.NewTaskRequest) (transfer.Task, error)
	CompleteTask(context.Context, int64, int64, transfer.CompleteTaskRequest) (transfer.Task, error)
}

// ServiceDeps 是 teaching service 的装配依赖集合。
type ServiceDeps struct {
	Store                  Store
	IDs                    snowflake.Generator
	Audit                  audit.Writer
	Identity               contracts.IdentityService
	Content                contracts.ContentReadService
	Judge                  contracts.JudgeService
	Bus                    eventbus.Bus
	Transfers              transferService
	Storage                *storage.Storage
	Objects                objectStorage
	FileService            fileService
	Config                 config.TeachingConfig
	CourseMaterialMaxBytes int64
	CourseCoverMaxBytes    int64
	MaterialScanPolicy     upload.ScanPolicy
}

// NewService 构造 teaching 服务,不接收数据库连接,由装配层传入 Store。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("teaching service 缺少 store")
	}
	if deps.IDs == nil {
		return nil, fmt.Errorf("teaching service 缺少 ID 生成器")
	}
	if deps.Audit == nil {
		return nil, fmt.Errorf("teaching service 缺少审计写入器")
	}
	if deps.Identity == nil {
		return nil, fmt.Errorf("teaching service 缺少 identity 契约")
	}
	if deps.Content == nil {
		return nil, fmt.Errorf("teaching service 缺少 content 契约")
	}
	if deps.Judge == nil {
		return nil, fmt.Errorf("teaching service 缺少 judge 契约")
	}
	if deps.Bus == nil {
		return nil, fmt.Errorf("teaching service 缺少事件总线")
	}
	objects := deps.Objects
	if objects == nil {
		objects = deps.Storage
	}
	if deps.Transfers == nil || objects == nil || deps.FileService == nil {
		return nil, fmt.Errorf("teaching service 缺少统一导入导出或文件服务依赖")
	}
	if deps.Config.CourseGradesMaxRows <= 0 || deps.Config.JudgeOutboxBatchSize <= 0 || deps.Config.GradeEventOutboxBatchSize <= 0 || deps.Config.GradeEventOutboxStaleMs <= 0 || deps.Config.GradeExportBatchSize <= 0 || deps.CourseMaterialMaxBytes <= 0 || deps.CourseCoverMaxBytes <= 0 {
		return nil, fmt.Errorf("teaching service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, audit: deps.Audit, identity: deps.Identity, content: deps.Content, judge: deps.Judge, bus: deps.Bus, transfers: deps.Transfers, storage: objects, files: deps.FileService, cfg: deps.Config, materialMaxBytes: deps.CourseMaterialMaxBytes, coverMaxBytes: deps.CourseCoverMaxBytes, materialScanPolicy: deps.MaterialScanPolicy}, nil
}

// currentIdentity 读取租户账号身份。
func currentIdentity(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

// mapCourseError 将数据库未命中归一为课程不存在。
func mapCourseError(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return apperr.ErrTeachingCourseNotFound
	}
	return apperr.ErrTeachingCourseInvalid.WithCause(err)
}

// mapAssignmentError 将数据库未命中归一为作业不存在。
func mapAssignmentError(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return apperr.ErrTeachingAssignmentNotFound
	}
	return apperr.ErrTeachingAssignmentInvalid.WithCause(err)
}

// mapGradeError 将数据库未命中或锁定写失败归一为成绩错误。
func mapGradeError(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return apperr.ErrTeachingGradeLocked
	}
	return apperr.ErrTeachingGradeInvalid.WithCause(err)
}
