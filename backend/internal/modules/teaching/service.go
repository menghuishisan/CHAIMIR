// teaching service 文件定义 M6 服务依赖注入和通用业务编排,不接收数据库连接。
package teaching

import (
	"context"
	"fmt"
	"io"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
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
	identity  contracts.IdentityService
	judge     contracts.JudgeService
	bus       eventbus.Bus
	transfers transferService
	storage   objectStorage
	files     fileService
	auth      *auth.Manager
	cfg       config.TeachingConfig
}

// objectStorage 描述 M6 导出产物写入统一对象存储所需能力。
type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	BucketReport() string
}

// fileService 描述 M6 复用统一文件服务规划对象路径所需能力。
type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
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
	Identity    contracts.IdentityService
	Judge       contracts.JudgeService
	Bus         eventbus.Bus
	Transfers   transferService
	Storage     *storage.Storage
	Objects     objectStorage
	FileService fileService
	Auth        *auth.Manager
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
	if deps.Identity == nil {
		return nil, fmt.Errorf("teaching service 缺少 identity 契约")
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
	if deps.Auth == nil {
		return nil, fmt.Errorf("teaching service 缺少统一鉴权服务")
	}
	if deps.Config.CourseGradesMaxRows <= 0 || deps.Config.JudgeOutboxBatchSize <= 0 || deps.Config.GradeEventOutboxBatchSize <= 0 || deps.Config.GradeEventOutboxStaleMs <= 0 || deps.Config.GradeExportBatchSize <= 0 {
		return nil, fmt.Errorf("teaching service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, audit: deps.Audit, content: deps.Content, identity: deps.Identity, judge: deps.Judge, bus: deps.Bus, transfers: deps.Transfers, storage: objects, files: deps.FileService, auth: deps.Auth, cfg: deps.Config}, nil
}

// accountSummaries 批量读取教学列表需要的学生姓名与学号，避免逐行查询身份模块。
func (s *Service) accountSummaries(ctx context.Context, accountIDs []int64) (map[int64]contracts.AccountInfo, error) {
	unique := make([]int64, 0, len(accountIDs))
	seen := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID > 0 {
			if _, exists := seen[accountID]; !exists {
				seen[accountID] = struct{}{}
				unique = append(unique, accountID)
			}
		}
	}
	if len(unique) == 0 {
		return map[int64]contracts.AccountInfo{}, nil
	}
	rows, err := s.identity.BatchGetAccounts(ctx, unique)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]contracts.AccountInfo, len(rows))
	for _, row := range rows {
		out[row.AccountID] = row
	}
	for _, accountID := range unique {
		if _, exists := out[accountID]; !exists {
			return nil, apperr.ErrTeachingCourseInvalid.WithCause(fmt.Errorf("教学关联账号不存在: account_id=%d", accountID))
		}
	}
	return out, nil
}

// fillMemberSummaries 为课程成员响应补齐身份模块提供的授权摘要。
func (s *Service) fillMemberSummaries(ctx context.Context, items []MemberDTO) error {
	accountIDs := make([]int64, 0, len(items))
	for _, item := range items {
		accountIDs = append(accountIDs, item.StudentID.Int64())
	}
	summaries, err := s.accountSummaries(ctx, accountIDs)
	if err != nil {
		return err
	}
	for i := range items {
		info := summaries[items[i].StudentID.Int64()]
		items[i].StudentName, items[i].StudentNo = info.Name, info.No
	}
	return nil
}

// fillSubmissionSummaries 为作业提交响应补齐学生姓名和学号。
func (s *Service) fillSubmissionSummaries(ctx context.Context, items []SubmissionDTO) error {
	accountIDs := make([]int64, 0, len(items))
	for _, item := range items {
		accountIDs = append(accountIDs, item.StudentID.Int64())
	}
	summaries, err := s.accountSummaries(ctx, accountIDs)
	if err != nil {
		return err
	}
	for i := range items {
		info := summaries[items[i].StudentID.Int64()]
		items[i].StudentName, items[i].StudentNo = info.Name, info.No
	}
	return nil
}

// fillGradeSummaries 为课程成绩响应补齐学生姓名和学号。
func (s *Service) fillGradeSummaries(ctx context.Context, items []GradeDTO) error {
	accountIDs := make([]int64, 0, len(items))
	for _, item := range items {
		accountIDs = append(accountIDs, item.StudentID.Int64())
	}
	summaries, err := s.accountSummaries(ctx, accountIDs)
	if err != nil {
		return err
	}
	for i := range items {
		info := summaries[items[i].StudentID.Int64()]
		items[i].StudentName, items[i].StudentNo = info.Name, info.No
	}
	return nil
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
	if _, ok := apperr.As(err); ok {
		return err
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
	if _, ok := apperr.As(err); ok {
		return err
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
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.ErrTeachingGradeInvalid.WithCause(err)
}
