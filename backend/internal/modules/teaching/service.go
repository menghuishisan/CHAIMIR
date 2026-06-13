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
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// Service 承载 teaching 模块业务编排,依赖 repo 接口和跨模块 contracts。
type Service struct {
	store     Store
	ids       snowflake.Generator
	audit     audit.Writer
	content   contracts.ContentReadService
	judge     contracts.JudgeService
	bus       eventbus.Bus
	transfers transferService
	storage   objectStorage
	files     fileService
	cfg       config.TeachingConfig
}

// objectStorage 描述 M6 导出产物写入统一对象存储所需能力。
type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	BucketReport() string
}

// fileService 描述 M6 复用统一文件服务规划对象路径所需能力。
type fileService interface {
	PlanUpload(req storage.PlanUploadRequest) (storage.UploadPlan, error)
}

// transferService 描述 M6 调用统一导入导出中心所需能力。
type transferService interface {
	CreateTask(context.Context, transfer.NewTaskRequest) (transfer.Task, error)
	CompleteTask(context.Context, int64, int64, transfer.CompleteTaskRequest) (transfer.Task, error)
}

// ServiceDeps 是 teaching service 的装配依赖集合。
type ServiceDeps struct {
	Store       Store
	IDs         snowflake.Generator
	Audit       audit.Writer
	Content     contracts.ContentReadService
	Judge       contracts.JudgeService
	Bus         eventbus.Bus
	Transfers   transferService
	Storage     *storage.Storage
	Objects     objectStorage
	FileService fileService
	Config      config.TeachingConfig
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
	if deps.Config.CourseGradesMaxRows <= 0 || deps.Config.JudgeOutboxBatchSize <= 0 || deps.Config.GradeExportBatchSize <= 0 {
		return nil, fmt.Errorf("teaching service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, audit: deps.Audit, content: deps.Content, judge: deps.Judge, bus: deps.Bus, transfers: deps.Transfers, storage: objects, files: deps.FileService, cfg: deps.Config}, nil
}

// currentIdentity 读取租户账号身份。
func currentIdentity(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

// currentServiceTenant 读取内部服务租户边界。
func currentServiceTenant(ctx context.Context) (int64, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || !id.IsSystem {
		return 0, apperr.ErrServiceUnauthorized
	}
	return id.TenantID, nil
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
