// M9 DTO 定义:隔离 HTTP 请求/响应、服务聚合结果与数据访问视图。
package admin

import (
	"time"

	"chaimir/internal/contracts"
)

// DashboardDTO 是平台/学校运营看板聚合结果。
type DashboardDTO struct {
	Scope      int16                     `json:"scope"`
	TenantID   string                    `json:"tenant_id,omitempty"`
	Identity   contracts.IdentityStats   `json:"identity"`
	Sandbox    contracts.SandboxStats    `json:"sandbox"`
	Teaching   contracts.TeachingStats   `json:"teaching"`
	Experiment contracts.ExperimentStats `json:"experiment"`
	Contest    contracts.ContestStats    `json:"contest"`
}

// StatisticDTO 是 M9 周期统计快照视图。
type StatisticDTO struct {
	ID        string         `json:"id"`
	Scope     int16          `json:"scope"`
	TenantID  string         `json:"tenant_id,omitempty"`
	StatDate  string         `json:"stat_date"`
	Metrics   map[string]any `json:"metrics"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

// ConfigDTO 是系统配置视图,敏感值返回前会脱敏。
type ConfigDTO struct {
	ID        string         `json:"id"`
	Scope     int16          `json:"scope"`
	TenantID  string         `json:"tenant_id,omitempty"`
	Key       string         `json:"key"`
	Value     map[string]any `json:"value"`
	Version   int32          `json:"version"`
	UpdatedBy string         `json:"updated_by,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
}

// ConfigUpdateRequest 是配置更新请求,version 用于乐观锁。
type ConfigUpdateRequest struct {
	Scope   int16          `json:"scope"`
	Value   map[string]any `json:"value"`
	Version int32          `json:"version"`
}

// ConfigRollbackRequest 是配置回退请求,version 仍用于当前配置乐观锁。
type ConfigRollbackRequest struct {
	Scope     int16  `json:"scope"`
	HistoryID string `json:"history_id"`
	Version   int32  `json:"version"`
}

// ConfigChangeLogDTO 是配置变更历史视图。
type ConfigChangeLogDTO struct {
	ID         string         `json:"id"`
	ConfigID   string         `json:"config_id"`
	TenantID   string         `json:"tenant_id,omitempty"`
	OldValue   map[string]any `json:"old_value"`
	NewValue   map[string]any `json:"new_value"`
	OperatorID string         `json:"operator_id"`
	CreatedAt  time.Time      `json:"created_at,omitempty"`
}

// AlertRuleRequest 是业务级告警规则创建请求。
type AlertRuleRequest struct {
	Scope     int16          `json:"scope"`
	Name      string         `json:"name"`
	Metric    string         `json:"metric"`
	Condition map[string]any `json:"condition"`
	Level     int16          `json:"level"`
	Enabled   bool           `json:"enabled"`
}

// AlertRulePatchRequest 是业务级告警规则更新请求。
type AlertRulePatchRequest struct {
	Name      string         `json:"name"`
	Metric    string         `json:"metric"`
	Condition map[string]any `json:"condition"`
	Level     int16          `json:"level"`
	Enabled   bool           `json:"enabled"`
}

// AlertRuleDTO 是业务级告警规则视图。
type AlertRuleDTO struct {
	ID        string         `json:"id"`
	Scope     int16          `json:"scope"`
	TenantID  string         `json:"tenant_id,omitempty"`
	Name      string         `json:"name"`
	Metric    string         `json:"metric"`
	Condition map[string]any `json:"condition"`
	Level     int16          `json:"level"`
	Enabled   bool           `json:"enabled"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
}

// AlertEventDTO 是业务级告警事件视图。
type AlertEventDTO struct {
	ID          string    `json:"id"`
	RuleID      string    `json:"rule_id"`
	TenantID    string    `json:"tenant_id,omitempty"`
	Level       int16     `json:"level"`
	Message     string    `json:"message"`
	Status      int16     `json:"status"`
	HandlerID   string    `json:"handler_id,omitempty"`
	TriggeredAt time.Time `json:"triggered_at,omitempty"`
	HandledAt   time.Time `json:"handled_at,omitempty"`
}

// AlertHandleRequest 是处理或忽略告警事件的请求。
type AlertHandleRequest struct {
	Status int16 `json:"status"`
}

// MonitoringPanelDTO 是外接监控面板嵌入入口。
type MonitoringPanelDTO struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Scope int16  `json:"scope"`
}

// BackupRecordDTO 是备份记录视图。
type BackupRecordDTO struct {
	ID         string    `json:"id"`
	Type       int16     `json:"type"`
	StorageRef string    `json:"storage_ref"`
	SizeBytes  int64     `json:"size_bytes"`
	Status     int16     `json:"status"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

// BackupTriggerRequest 是 M9 记录备份触发请求的参数。
type BackupTriggerRequest struct {
	Type       int16  `json:"type"`
	StorageRef string `json:"storage_ref"`
}

// ApplicationApproveRequest 是平台管理员通过入驻申请的请求。
type ApplicationApproveRequest struct {
	TenantCode string `json:"tenant_code"`
	AdminPhone string `json:"admin_phone"`
	AdminName  string `json:"admin_name"`
}

// ApplicationRejectRequest 是平台管理员驳回入驻申请的请求。
type ApplicationRejectRequest struct {
	Reason string `json:"reason"`
}
