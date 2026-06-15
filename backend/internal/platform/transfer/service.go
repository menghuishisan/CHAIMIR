// transfer service 文件负责统一导入导出中心的生产级任务编排。
package transfer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// Service 是统一导入导出中心对模块和 HTTP 层暴露的生产服务。
type Service struct {
	store   Store
	ids     snowflake.Generator
	manager Manager
}

// ServiceDeps 描述统一导入导出中心服务的装配依赖。
type ServiceDeps struct {
	Store   Store
	IDs     snowflake.Generator
	Manager Manager
}

// TaskListQuery 描述 HTTP/API 查询任务列表的条件。
type TaskListQuery struct {
	TenantID  int64
	AccountID int64
	Channel   Channel
	Status    Status
	Page      int
	Size      int
}

// DownloadGrantDTO 是签发给前端的统一下载授权响应。
type DownloadGrantDTO struct {
	Token     string                `json:"token"`
	Grant     storage.DownloadGrant `json:"grant"`
	Task      TaskDTO               `json:"task"`
	ExpiresAt string                `json:"expires_at"`
}

// TaskDTO 是统一导入导出任务的用户向快照。
type TaskDTO struct {
	TaskID              string `json:"task_id"`
	Channel             string `json:"channel"`
	Subject             string `json:"subject"`
	Status              string `json:"status"`
	ContentType         string `json:"content_type,omitempty"`
	FileName            string `json:"file_name,omitempty"`
	AttemptCount        int    `json:"attempt_count"`
	MaxAttempts         int    `json:"max_attempts"`
	ArtifactSize        int64  `json:"artifact_size,omitempty"`
	ArtifactContentType string `json:"artifact_content_type,omitempty"`
	ArtifactFileName    string `json:"artifact_file_name,omitempty"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
	CompletedAt         string `json:"completed_at,omitempty"`
	NextAttemptAfter    string `json:"next_attempt_after,omitempty"`
}

// NewService 构造统一导入导出中心服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("transfer service 缺少 store")
	}
	if deps.IDs == nil {
		return nil, fmt.Errorf("transfer service 缺少 ID 生成器")
	}
	if err := deps.Manager.validateConfig(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(deps.Manager.StorageSigningKey) == "" {
		return nil, fmt.Errorf("transfer service 缺少下载签名密钥")
	}
	return &Service{store: deps.Store, ids: deps.IDs, manager: deps.Manager}, nil
}

// CreateTask 创建并持久化统一导入导出任务。
func (s *Service) CreateTask(ctx context.Context, req NewTaskRequest) (Task, error) {
	if req.TaskID == 0 {
		req.TaskID = s.ids.Generate()
	}
	task, err := s.manager.NewTask(req)
	if err != nil {
		return Task{}, apperr.ErrTransferTaskInvalid.WithCause(err)
	}
	created, err := s.store.CreateTask(ctx, task)
	if err != nil {
		return Task{}, err
	}
	return created, nil
}

// GetTask 读取单个任务。
func (s *Service) GetTask(ctx context.Context, tenantID, taskID int64) (Task, error) {
	if tenantID <= 0 || taskID <= 0 {
		return Task{}, apperr.ErrTransferTaskInvalid
	}
	return s.store.GetTask(ctx, tenantID, taskID)
}

// ListTasks 查询当前账号的导入导出任务。
func (s *Service) ListTasks(ctx context.Context, query TaskListQuery) ([]Task, int, int, error) {
	if query.TenantID <= 0 || query.AccountID <= 0 {
		return nil, 0, 0, apperr.ErrTransferTaskInvalid
	}
	if query.Channel != "" {
		if err := validateChannel(query.Channel); err != nil {
			return nil, 0, 0, apperr.ErrTransferTaskInvalid.WithCause(err)
		}
	}
	if query.Status != "" && !validStatus(query.Status) {
		return nil, 0, 0, apperr.ErrTransferTaskInvalid
	}
	page, size := pagex.Normalize(query.Page, query.Size)
	items, err := s.store.ListTasks(ctx, ListTasksQuery{
		TenantID:  query.TenantID,
		AccountID: query.AccountID,
		Channel:   query.Channel,
		Status:    query.Status,
		Limit:     int32(size),
		Offset:    int32((page - 1) * size),
	})
	if err != nil {
		return nil, 0, 0, err
	}
	return items, page, size, nil
}

// CompleteTask 完成任务并登记统一文件服务对象引用。
func (s *Service) CompleteTask(ctx context.Context, tenantID, taskID int64, req CompleteTaskRequest) (Task, error) {
	task, err := s.GetTask(ctx, tenantID, taskID)
	if err != nil {
		return Task{}, err
	}
	completed, err := s.manager.CompleteTask(task, req)
	if err != nil {
		return Task{}, apperr.ErrTransferTaskInvalid.WithCause(err)
	}
	return s.store.UpdateTask(ctx, completed)
}

// FailTask 按统一重试策略记录任务失败。
func (s *Service) FailTask(ctx context.Context, tenantID, taskID int64, cause error, now time.Time) (Task, error) {
	task, err := s.GetTask(ctx, tenantID, taskID)
	if err != nil {
		return Task{}, err
	}
	failed, err := s.manager.FailTask(task, cause, now)
	if err != nil {
		return Task{}, apperr.ErrTransferTaskInvalid.WithCause(err)
	}
	return s.store.UpdateTask(ctx, failed)
}

// BuildDownloadGrant 校验任务归属并签发统一文件服务下载授权。
func (s *Service) BuildDownloadGrant(ctx context.Context, tenantID, taskID, accountID int64, tenantAdmin bool) (DownloadGrantDTO, error) {
	task, err := s.GetTask(ctx, tenantID, taskID)
	if err != nil {
		return DownloadGrantDTO{}, err
	}
	if err := EnsureTaskOwner(task, tenantID, accountID, tenantAdmin); err != nil {
		return DownloadGrantDTO{}, err
	}
	if task.Status != StatusSucceeded || strings.TrimSpace(task.Artifact.ObjectRef) == "" {
		return DownloadGrantDTO{}, apperr.ErrTransferTaskNotDownloadable
	}
	token, grant, err := s.manager.BuildDownloadGrant(task, timex.Now())
	if err != nil {
		return DownloadGrantDTO{}, apperr.ErrTransferTaskNotDownloadable.WithCause(err)
	}
	return DownloadGrantDTO{Token: token, Grant: grant, Task: TaskToDTO(task), ExpiresAt: formatOptionalTime(grant.ExpiresAt)}, nil
}

// EnsureTaskOwner 校验任务访问者必须在同租户内,且只能读本人任务或由租户管理员读取。
func EnsureTaskOwner(task Task, tenantID, accountID int64, tenantAdmin bool) error {
	if task.TenantID != tenantID {
		return apperr.ErrTransferTaskForbidden
	}
	if task.AccountID != accountID && !tenantAdmin {
		return apperr.ErrTransferTaskForbidden
	}
	return nil
}

// TaskToDTO 把基础层任务快照转换成外部 JSON 安全 ID。
func TaskToDTO(task Task) TaskDTO {
	return TaskDTO{
		TaskID:              ids.Format(task.TaskID),
		Channel:             string(task.Channel),
		Subject:             task.Subject,
		Status:              string(task.Status),
		ContentType:         task.ContentType,
		FileName:            task.FileName,
		AttemptCount:        task.AttemptCount,
		MaxAttempts:         task.MaxAttempts,
		ArtifactSize:        task.Artifact.Size,
		ArtifactContentType: task.Artifact.ContentType,
		ArtifactFileName:    task.Artifact.FileName,
		CreatedAt:           formatOptionalTime(task.CreatedAt),
		UpdatedAt:           formatOptionalTime(task.UpdatedAt),
		CompletedAt:         formatOptionalTime(task.CompletedAt),
		NextAttemptAfter:    formatOptionalTime(task.NextAttemptAfter),
	}
}

// TasksToDTO 批量转换任务快照。
func TasksToDTO(tasks []Task) []TaskDTO {
	out := make([]TaskDTO, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, TaskToDTO(task))
	}
	return out
}

// validStatus 限制统一导入导出任务的公开过滤状态。
func validStatus(status Status) bool {
	switch status {
	case StatusPending, StatusRunning, StatusRetrying, StatusSucceeded, StatusFailed:
		return true
	default:
		return false
	}
}

// formatOptionalTime 统一输出 RFC3339 时间,零值字段省略为空字符串。
func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return timex.UTC(t).Format(time.RFC3339)
}
