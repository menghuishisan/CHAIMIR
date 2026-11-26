// admin dto 文件定义 M9 HTTP 请求和响应结构。
package admin

import (
	"time"

	"chaimir/internal/platform/ids"
)

// MonitoringPanel 是受控外部监控入口，不含平台或监控凭据。
type MonitoringPanel struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ResourceQuotaSnapshotDTO 是看板资源配额的固定摘要结构。
type ResourceQuotaSnapshotDTO struct {
	MaxConcurrentSandbox int64 `json:"max_concurrent_sandbox"`
	MaxCPU               int64 `json:"max_cpu"`
	MaxMemoryMB          int64 `json:"max_memory_mb"`
}

// DashboardDTO 是平台和学校看板聚合输出。
type DashboardDTO struct {
	Scope                 int16                     `json:"scope"`
	TenantID              ids.ID                    `json:"tenant_id,omitempty"`
	TenantCount           int64                     `json:"tenant_count,omitempty"`
	AccountCount          int64                     `json:"account_count"`
	TeacherCount          int64                     `json:"teacher_count"`
	StudentCount          int64                     `json:"student_count"`
	ActiveAccountCount    int64                     `json:"active_account_count"`
	CourseCount           int64                     `json:"course_count"`
	ActiveCourseCount     int64                     `json:"active_course_count"`
	ExperimentCount       int64                     `json:"experiment_count"`
	ActiveInstanceCount   int64                     `json:"active_instance_count"`
	ContestCount          int64                     `json:"contest_count"`
	ActiveContestCount    int64                     `json:"active_contest_count"`
	ActiveSandboxCount    int64                     `json:"active_sandbox_count"`
	PendingApplyCount     int64                     `json:"pending_apply_count,omitempty"`
	ResourceQuotaSnapshot *ResourceQuotaSnapshotDTO `json:"resource_quota_snapshot,omitempty"`
	GeneratedAt           time.Time                 `json:"generated_at"`
}

// AlertRuleRequest 是告警规则创建和编辑请求。
type AlertRuleRequest struct {
	Scope     int16          `json:"scope"`
	TenantID  ids.ID         `json:"tenant_id,omitempty"`
	Name      string         `json:"name"`
	Metric    string         `json:"metric"`
	Condition map[string]any `json:"condition"`
	Level     int16          `json:"level"`
	Enabled   bool           `json:"enabled"`
}

// AlertEventRequest 是告警处理请求。
type AlertEventRequest struct {
	Status int16 `json:"status"`
}

// AlertRuleDTO 表示告警规则响应。
type AlertRuleDTO struct {
	revision  int64
	ID        ids.ID         `json:"id"`
	Scope     int16          `json:"scope"`
	TenantID  ids.ID         `json:"tenant_id,omitempty"`
	Name      string         `json:"name"`
	Metric    string         `json:"metric"`
	Condition map[string]any `json:"condition"`
	Level     int16          `json:"level"`
	Enabled   bool           `json:"enabled"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

// AlertEventDTO 表示告警事件响应。
type AlertEventDTO struct {
	RuleName    string `json:"rule_name,omitempty"`
	ID          ids.ID `json:"id"`
	RuleID      ids.ID `json:"rule_id"`
	TenantID    ids.ID `json:"tenant_id,omitempty"`
	Level       int16  `json:"level"`
	Message     string `json:"message"`
	Status      int16  `json:"status"`
	HandlerID   ids.ID `json:"handler_id,omitempty"`
	TriggeredAt string `json:"triggered_at"`
	HandledAt   string `json:"handled_at,omitempty"`
}

// AlertEventPushDTO 是告警处理完成后发送到告警 topic 的精确实时负载。
type AlertEventPushDTO struct {
	EventID   ids.ID `json:"event_id"`
	RuleID    ids.ID `json:"rule_id"`
	Level     int16  `json:"level"`
	Status    int16  `json:"status"`
	HandlerID ids.ID `json:"handler_id"`
}

// StatisticsDTO 表示运营统计时间序列响应。
type StatisticsDTO struct {
	Scope    int16          `json:"scope"`
	TenantID ids.ID         `json:"tenant_id,omitempty"`
	Date     string         `json:"date"`
	Metrics  map[string]any `json:"metrics"`
}

// BackupRecordDTO 表示备份记录响应。
type BackupRecordDTO struct {
	ID         ids.ID `json:"id"`
	Type       int16  `json:"type"`
	SizeBytes  int64  `json:"size_bytes"`
	Status     int16  `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

type AuditLogEntryDTO struct {
	ID       ids.ID `json:"id"`
	TenantID ids.ID `json:"tenant_id,omitempty"`
	// ActorID 使用文本类型，因为系统审计记录合法使用 actor_id=0，
	// 而 0 不是有效的正雪花 ID。
	ActorID string `json:"actor_id"`
	// ActorName 操作人姓名:学校身份解析本校账号,平台身份只解析平台管理员;系统记录与解析不到的省略
	ActorName string `json:"actor_name,omitempty"`
	// ActorNo 操作人学工号,只有学校账号才有
	ActorNo    string `json:"actor_no,omitempty"`
	ActorRole  int16  `json:"actor_role"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   ids.ID `json:"target_id,omitempty"`
	Detail     string `json:"detail,omitempty"`
	IP         string `json:"ip,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// BackupRecordCreate 是受控运维任务写入备份记录的内部请求。
type BackupRecordCreate struct {
	Type       int16
	StorageRef string
	SizeBytes  int64
	Status     int16
	StartedAt  time.Time
	FinishedAt time.Time
}

// AuditExportRequest 是 M9 持久化的可重放审计导出请求。
type AuditExportRequest struct {
	TransferTaskID int64
	TenantID       int64
	AccountID      int64
	QuerySnapshot  []byte
	NextCheckAt    time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
