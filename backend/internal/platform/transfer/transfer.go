// transfer 提供统一导入导出中心的通用任务模型、重试语义和下载中心边界。
package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"chaimir/internal/platform/timex"
)

// LeaseArtifactFileName 为每次任务租约生成稳定且不暴露租约值的内部产物文件名。
// 任务资源前缀仍保持 taskID,因此下载授权契约不变;租约摘要避免过期 worker 与新 worker 竞写同一对象。
func LeaseArtifactFileName(fileName, leaseToken string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	leaseToken = strings.TrimSpace(leaseToken)
	if fileName == "" || fileName == "." || fileName == ".." || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") || filepath.Base(fileName) != fileName || leaseToken == "" {
		return "", fmt.Errorf("导入导出任务租约产物文件名参数非法")
	}
	digest := sha256.Sum256([]byte(leaseToken))
	ext := filepath.Ext(fileName)
	stem := strings.TrimSuffix(fileName, ext)
	return stem + "." + hex.EncodeToString(digest[:8]) + ext, nil
}

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

// Config 描述统一导入导出中心的重试边界。
type Config struct {
	MaxAttempts   int
	RetryDelay    time.Duration
	LeaseDuration time.Duration
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
	LeaseToken       string
	LeaseUntil       time.Time
}

// Manager 负责统一导入导出中心的通用状态流转。
type Manager struct {
	Config Config
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
	// 文件名是下载中心的契约字段:CompleteTask 把它复制给产物,客户端据此命名保存的文件。
	// 允许为空会让下载端拿到空文件名,只能靠客户端硬编码兜底,故在任务入口处拒绝。
	if strings.TrimSpace(req.FileName) == "" {
		return Task{}, fmt.Errorf("导入导出任务缺少 file_name")
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
	task.LeaseToken = ""
	task.LeaseUntil = time.Time{}
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
	task.LeaseToken = ""
	task.LeaseUntil = time.Time{}
	return task, nil
}

// ClaimTask 为模块 worker 生成本次执行的租约快照。尝试次数在领取时增加，进程崩溃后的过期租约也会消耗一次尝试。
func (m Manager) ClaimTask(task Task, leaseToken string, now time.Time) (Task, error) {
	if err := m.validateConfig(); err != nil {
		return Task{}, err
	}
	if strings.TrimSpace(leaseToken) == "" {
		return Task{}, fmt.Errorf("导入导出任务缺少租约令牌")
	}
	if now.IsZero() {
		now = timex.Now()
	} else {
		now = timex.UTC(now)
	}
	task.Status = StatusRunning
	task.AttemptCount++
	task.LastError = ""
	task.LeaseToken = strings.TrimSpace(leaseToken)
	task.LeaseUntil = now.Add(m.Config.LeaseDuration)
	task.NextAttemptAfter = time.Time{}
	task.UpdatedAt = now
	return task, nil
}

// validateConfig 校验统一导入导出中心的全局运行边界。
func (m Manager) validateConfig() error {
	if m.Config.MaxAttempts <= 0 {
		return fmt.Errorf("统一导入导出中心最大尝试次数必须大于 0")
	}
	if m.Config.RetryDelay <= 0 {
		return fmt.Errorf("统一导入导出中心重试间隔必须大于 0")
	}
	if m.Config.LeaseDuration <= 0 {
		return fmt.Errorf("统一导入导出中心任务租约时长必须大于 0")
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
