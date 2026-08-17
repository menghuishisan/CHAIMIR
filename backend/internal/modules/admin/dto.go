// admin dto 文件定义 M9 HTTP 请求和响应结构。
package admin

import (
	"time"

	"chaimir/internal/platform/ids"
)

// MonitoringPanel 是外接监控系统的安全嵌入入口响应。
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

// ConfigDTO 表示系统配置响应。
type ConfigDTO struct {
	ID        ids.ID         `json:"id"`
	Scope     int16          `json:"scope"`
	TenantID  ids.ID         `json:"tenant_id,omitempty"`
	Key       string         `json:"key"`
	Value     map[string]any `json:"value"`
	Version   int32          `json:"version"`
	UpdatedBy ids.ID         `json:"updated_by"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// ConfigUpdateRequest 是配置更新和回滚请求。
type ConfigUpdateRequest struct {
	Scope       int16          `json:"scope"`
	TenantID    ids.ID         `json:"tenant_id,omitempty"`
	Value       map[string]any `json:"value"`
	Version     int32          `json:"version"`
	ChangeLogID ids.ID         `json:"change_log_id,omitempty"`
}

// ConfigRollbackRequest 是配置回滚请求,只携带回滚所需的历史记录和当前版本。
type ConfigRollbackRequest struct {
	Scope       int16  `json:"scope"`
	TenantID    ids.ID `json:"tenant_id,omitempty"`
	Version     int32  `json:"version"`
	ChangeLogID ids.ID `json:"change_log_id,omitempty"`
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

// ConfigChangeLogDTO 表示配置变更历史响应。
type ConfigChangeLogDTO struct {
	ID         ids.ID         `json:"id"`
	ConfigID   ids.ID         `json:"config_id"`
	TenantID   ids.ID         `json:"tenant_id,omitempty"`
	OldValue   map[string]any `json:"old_value"`
	NewValue   map[string]any `json:"new_value"`
	OperatorID ids.ID         `json:"operator_id"`
	CreatedAt  string         `json:"created_at"`
}

// AlertRuleDTO 表示告警规则响应。
type AlertRuleDTO struct {
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
	FinishedAt string `json:"finished_at,omitempty"`
}

type AuditLogEntryDTO struct {
	ID         ids.ID `json:"id"`
	TenantID   ids.ID `json:"tenant_id,omitempty"`
	ActorID    ids.ID `json:"actor_id"`
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
}
