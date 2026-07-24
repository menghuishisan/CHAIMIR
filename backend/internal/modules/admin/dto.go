// admin dto 文件定义 M9 HTTP 请求和响应结构。
package admin

import "chaimir/internal/platform/ids"

// ConfigUpdateRequest 是配置更新和回滚请求。
type ConfigUpdateRequest struct {
	Scope       int16                 `json:"scope"`
	TenantID    ids.ID                `json:"tenant_id,omitempty"`
	Value       MaintenanceModeConfig `json:"value"`
	Version     int32                 `json:"version"`
	ChangeLogID ids.ID                `json:"change_log_id,omitempty"`
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
	Condition AlertCondition `json:"condition"`
	Level     int16          `json:"level"`
	Enabled   bool           `json:"enabled"`
}

// AlertCondition 定义业务告警的唯一阈值结构。
type AlertCondition struct {
	Operator        string  `json:"operator"`
	Threshold       float64 `json:"threshold"`
	DurationMinutes float64 `json:"duration_minutes"`
}

// AlertEventRequest 是告警处理请求。
type AlertEventRequest struct {
	Status int16 `json:"status"`
}

// ConfigChangeLogDTO 表示配置变更历史响应。
type ConfigChangeLogDTO struct {
	ID         ids.ID                `json:"id"`
	ConfigID   ids.ID                `json:"config_id"`
	TenantID   ids.ID                `json:"tenant_id,omitempty"`
	OldValue   MaintenanceModeConfig `json:"old_value"`
	NewValue   MaintenanceModeConfig `json:"new_value"`
	OperatorID ids.ID                `json:"operator_id"`
	CreatedAt  string                `json:"created_at"`
}

// MaintenanceModeConfig 定义平台维护开关的唯一配置结构。
type MaintenanceModeConfig struct {
	Enabled bool `json:"enabled"`
}

// AlertRuleDTO 表示告警规则响应。
type AlertRuleDTO struct {
	ID        ids.ID         `json:"id"`
	Scope     int16          `json:"scope"`
	TenantID  ids.ID         `json:"tenant_id,omitempty"`
	Name      string         `json:"name"`
	Metric    string         `json:"metric"`
	Condition AlertCondition `json:"condition"`
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
	TenantName  string `json:"tenant_name,omitempty"`
	Level       int16  `json:"level"`
	Message     string `json:"message"`
	Status      int16  `json:"status"`
	HandlerID   ids.ID `json:"handler_id,omitempty"`
	TriggeredAt string `json:"triggered_at"`
	HandledAt   string `json:"handled_at,omitempty"`
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

type TenantApplicationSummaryDTO struct {
	ApplicationID ids.ID `json:"application_id"`
	SchoolName    string `json:"school_name"`
	SchoolType    int16  `json:"school_type"`
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	ContactEmail  string `json:"contact_email"`
	Status        int16  `json:"status"`
	SubmittedAt   string `json:"submitted_at"`
	ReviewedAt    string `json:"reviewed_at,omitempty"`
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
