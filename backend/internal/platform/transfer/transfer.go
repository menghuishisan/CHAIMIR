// transfer 提供统一导入导出中心的通用任务模型、重试语义和下载中心边界。
package transfer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
)

// Status 表示统一导入导出任务的生命周期状态。
type Status string

const (
	// StatusPending 表示任务已创建但尚未执行。
	StatusPending Status = "pending"
	// StatusRunning 表示任务正在执行。
	StatusRunning Status = "running"
	// StatusRetrying 表示任务失败后等待下一次重试。
	StatusRetrying Status = "retrying"
	// StatusSucceeded 表示任务成功完成并产生可下载产物。
	StatusSucceeded Status = "succeeded"
	// StatusFailed 表示任务已重试耗尽并进入最终失败态。
	StatusFailed Status = "failed"
)

// Channel 表示统一导入导出中心处理的通道类型。
type Channel string

const (
	// ChannelImport 表示导入任务。
	ChannelImport Channel = "import"
	// ChannelExport 表示导出任务。
	ChannelExport Channel = "export"
)

// Config 描述统一导入导出中心的重试和下载授权边界。
type Config struct {
	MaxAttempts      int
	RetryDelay       time.Duration
	DownloadGrantTTL time.Duration
}

// Artifact 表示任务成功后产出的统一文件服务对象引用。
type Artifact struct {
	ObjectRef   string
	Size        int64
	ContentType string
	FileName    string
}

// Task 表示统一导入导出中心在服务端持久化和流转的任务快照。
type Task struct {
	TaskID           int64
	TenantID         int64
	AccountID        int64
	Channel          Channel
	Subject          string
	Status           Status
	ContentType      string
	FileName         string
	AttemptCount     int
	MaxAttempts      int
	LastError        string
	Artifact         Artifact
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      time.Time
	NextAttemptAfter time.Time
}

// Manager 负责统一导入导出中心的通用状态流转和下载授权编排。
type Manager struct {
	Config            Config
	StorageSigningKey string
}

// NewTaskRequest 描述创建统一导入导出任务所需的通用字段。
type NewTaskRequest struct {
	TaskID      int64
	TenantID    int64
	AccountID   int64
	Channel     Channel
	Subject     string
	FileName    string
	ContentType string
}

// CompleteTaskRequest 描述任务成功完成后需要登记的产物信息。
type CompleteTaskRequest struct {
	ObjectRef string
	Size      int64
}

// NewTask 创建统一 pending 任务快照,为后续模块化执行器提供同一状态机起点。
func (m Manager) NewTask(req NewTaskRequest) (Task, error) {
	if err := m.validateConfig(); err != nil {
		return Task{}, err
	}
	if req.TaskID <= 0 {
		return Task{}, fmt.Errorf("导入导出任务缺少 task_id")
	}
	if req.TenantID < 0 || req.AccountID <= 0 {
		return Task{}, fmt.Errorf("导入导出任务缺少租户或账号边界")
	}
	if err := validateChannel(req.Channel); err != nil {
		return Task{}, err
	}
	if strings.TrimSpace(req.Subject) == "" {
		return Task{}, fmt.Errorf("导入导出任务缺少 subject")
	}
	now := timex.Now()
	return Task{
		TaskID:       req.TaskID,
		TenantID:     req.TenantID,
		AccountID:    req.AccountID,
		Channel:      req.Channel,
		Subject:      strings.TrimSpace(req.Subject),
		Status:       StatusPending,
		ContentType:  strings.TrimSpace(req.ContentType),
		FileName:     strings.TrimSpace(req.FileName),
		MaxAttempts:  m.Config.MaxAttempts,
		CreatedAt:    now,
		UpdatedAt:    now,
		AttemptCount: 0,
	}, nil
}

// CompleteTask 把任务推进到 succeeded,并登记统一文件服务对象引用作为下载中心产物。
func (m Manager) CompleteTask(task Task, req CompleteTaskRequest) (Task, error) {
	if err := m.validateConfig(); err != nil {
		return Task{}, err
	}
	if strings.TrimSpace(req.ObjectRef) == "" {
		return Task{}, fmt.Errorf("导入导出任务缺少产物对象引用")
	}
	if req.Size <= 0 {
		return Task{}, fmt.Errorf("导入导出任务产物大小必须大于 0")
	}
	task.Status = StatusSucceeded
	task.LastError = ""
	task.Artifact = Artifact{
		ObjectRef:   strings.TrimSpace(req.ObjectRef),
		Size:        req.Size,
		ContentType: task.ContentType,
		FileName:    task.FileName,
	}
	task.CompletedAt = timex.Now()
	task.UpdatedAt = task.CompletedAt
	task.NextAttemptAfter = time.Time{}
	return task, nil
}

// FailTask 按统一重试策略推进任务状态,耗尽前进入 retrying,耗尽后进入 failed。
func (m Manager) FailTask(task Task, cause error, now time.Time) (Task, error) {
	if err := m.validateConfig(); err != nil {
		return Task{}, err
	}
	if cause == nil {
		return Task{}, fmt.Errorf("导入导出任务失败原因不能为空")
	}
	if now.IsZero() {
		now = timex.Now()
	} else {
		now = timex.UTC(now)
	}
	task.AttemptCount++
	task.LastError = strings.TrimSpace(cause.Error())
	task.UpdatedAt = now
	if task.AttemptCount < task.MaxAttempts {
		task.Status = StatusRetrying
		task.NextAttemptAfter = now.Add(m.Config.RetryDelay)
		return task, nil
	}
	task.Status = StatusFailed
	task.CompletedAt = now
	task.NextAttemptAfter = time.Time{}
	return task, nil
}

// BuildDownloadGrant 为已完成任务的产物签发统一文件服务短时下载授权,供下载中心复用。
func (m Manager) BuildDownloadGrant(task Task, now time.Time) (string, storage.DownloadGrant, error) {
	if err := m.validateConfig(); err != nil {
		return "", storage.DownloadGrant{}, err
	}
	if task.Status != StatusSucceeded {
		return "", storage.DownloadGrant{}, fmt.Errorf("仅已完成任务可签发下载授权")
	}
	if strings.TrimSpace(task.Artifact.ObjectRef) == "" {
		return "", storage.DownloadGrant{}, fmt.Errorf("任务缺少产物对象引用")
	}
	if strings.TrimSpace(m.StorageSigningKey) == "" {
		return "", storage.DownloadGrant{}, fmt.Errorf("统一导入导出中心缺少文件服务签名密钥")
	}
	if now.IsZero() {
		now = timex.Now()
	} else {
		now = timex.UTC(now)
	}
	service := storage.Service{
		SigningKey:       m.StorageSigningKey,
		DownloadGrantTTL: m.Config.DownloadGrantTTL,
	}
	return service.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:           task.TenantID,
		AccountID:          task.AccountID,
		AllowPlatformScope: task.TenantID == 0,
		ObjectRef:          task.Artifact.ObjectRef,
		Module:             "transfer",
		ResourceType:       string(task.Channel),
		ResourceID:         strconv.FormatInt(task.TaskID, 10),
		ExpiresAt:          now.Add(m.Config.DownloadGrantTTL),
	})
}

// validateConfig 校验统一导入导出中心的全局运行边界。
func (m Manager) validateConfig() error {
	if m.Config.MaxAttempts <= 0 {
		return fmt.Errorf("统一导入导出中心最大尝试次数必须大于 0")
	}
	if m.Config.RetryDelay <= 0 {
		return fmt.Errorf("统一导入导出中心重试间隔必须大于 0")
	}
	if m.Config.DownloadGrantTTL <= 0 {
		return fmt.Errorf("统一导入导出中心下载授权 TTL 必须大于 0")
	}
	return nil
}

// validateChannel 限制统一导入导出中心只接收导入和导出两类通道。
func validateChannel(channel Channel) error {
	switch channel {
	case ChannelImport, ChannelExport:
		return nil
	default:
		return fmt.Errorf("导入导出任务通道非法")
	}
}
