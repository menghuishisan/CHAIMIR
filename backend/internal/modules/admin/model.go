// admin model 文件定义 M9 管理后台内部模型和响应视图。
package admin

import (
	"time"

	"chaimir/internal/platform/ids"
)

// MonitoringPanel 是外接监控系统的安全嵌入入口。
type MonitoringPanel struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// DashboardDTO 是平台和学校看板聚合输出。
type DashboardDTO struct {
	Scope                 int16          `json:"scope"`
	TenantID              ids.ID         `json:"tenant_id,omitempty"`
	TenantCount           int64          `json:"tenant_count,omitempty"`
	AccountCount          int64          `json:"account_count"`
	TeacherCount          int64          `json:"teacher_count"`
	StudentCount          int64          `json:"student_count"`
	ActiveAccountCount    int64          `json:"active_account_count"`
	CourseCount           int64          `json:"course_count"`
	ActiveCourseCount     int64          `json:"active_course_count"`
	ExperimentCount       int64          `json:"experiment_count"`
	ActiveInstanceCount   int64          `json:"active_instance_count"`
	ContestCount          int64          `json:"contest_count"`
	ActiveContestCount    int64          `json:"active_contest_count"`
	ActiveSandboxCount    int64          `json:"active_sandbox_count"`
	PendingApplyCount     int64          `json:"pending_apply_count,omitempty"`
	ResourceQuotaSnapshot map[string]any `json:"resource_quota_snapshot,omitempty"`
	GeneratedAt           time.Time      `json:"generated_at"`
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
