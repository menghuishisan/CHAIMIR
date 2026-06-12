// admin dto 文件定义 M9 HTTP 请求和响应结构。
package admin

// ConfigUpdateRequest 是配置更新和回滚请求。
type ConfigUpdateRequest struct {
	Scope       int16          `json:"scope"`
	TenantID    int64          `json:"tenant_id,string,omitempty"`
	Value       map[string]any `json:"value"`
	Version     int32          `json:"version"`
	ChangeLogID int64          `json:"change_log_id,string,omitempty"`
}

// AlertRuleRequest 是告警规则创建和编辑请求。
type AlertRuleRequest struct {
	Scope     int16          `json:"scope"`
	TenantID  int64          `json:"tenant_id,string,omitempty"`
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

// BackupTriggerRequest 是手工触发备份记录请求。
type BackupTriggerRequest struct {
	Type int16 `json:"type"`
}

// ConfigChangeLogDTO 表示配置变更历史响应。
type ConfigChangeLogDTO struct {
	ID         int64          `json:"id,string"`
	ConfigID   int64          `json:"config_id,string"`
	TenantID   int64          `json:"tenant_id,omitempty,string"`
	OldValue   map[string]any `json:"old_value"`
	NewValue   map[string]any `json:"new_value"`
	OperatorID int64          `json:"operator_id,string"`
	CreatedAt  string         `json:"created_at"`
}

// AlertRuleDTO 表示告警规则响应。
type AlertRuleDTO struct {
	ID        int64          `json:"id,string"`
	Scope     int16          `json:"scope"`
	TenantID  int64          `json:"tenant_id,omitempty,string"`
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
	ID          int64  `json:"id,string"`
	RuleID      int64  `json:"rule_id,string"`
	TenantID    int64  `json:"tenant_id,omitempty,string"`
	Level       int16  `json:"level"`
	Message     string `json:"message"`
	Status      int16  `json:"status"`
	HandlerID   int64  `json:"handler_id,omitempty,string"`
	TriggeredAt string `json:"triggered_at"`
	HandledAt   string `json:"handled_at,omitempty"`
}

// StatisticsDTO 表示运营统计时间序列响应。
type StatisticsDTO struct {
	Scope    int16          `json:"scope"`
	TenantID int64          `json:"tenant_id,omitempty,string"`
	Date     string         `json:"date"`
	Metrics  map[string]any `json:"metrics"`
}

// BackupRecordDTO 表示备份记录响应。
type BackupRecordDTO struct {
	ID         int64  `json:"id,string"`
	Type       int16  `json:"type"`
	StorageRef string `json:"storage_ref"`
	SizeBytes  int64  `json:"size_bytes"`
	Status     int16  `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
}
