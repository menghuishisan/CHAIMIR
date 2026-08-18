// db maintenance 文件定义跨租户维护事务的模块角色与表权限单一真相源。
package db

import "strings"

// MaintenancePolicy 描述一个模块维护角色及其唯一允许访问的表集合。
type MaintenancePolicy struct {
	Module string
	Role   string
	Tables []string
}

var maintenancePolicies = []MaintenancePolicy{
	{Module: "identity", Role: "chaimir_maintenance_identity", Tables: []string{
		"platform_admin", "platform_auth_session", "tenant", "tenant_provision_outbox", "tenant_application",
		"department", "major", "class", "account", "account_role", "account_profile", "auth_session",
		"sms_code", "activation_code", "sso_config", "import_preview", "import_batch", "audit_log",
	}},
	{Module: "sandbox", Role: "chaimir_maintenance_sandbox", Tables: []string{
		"runtime", "runtime_image", "tool", "sandbox", "sandbox_tool", "sandbox_event", "sandbox_recycle_outbox", "tenant_quota",
	}},
	{Module: "judge", Role: "chaimir_maintenance_judge", Tables: []string{
		"judger", "judge_task", "judge_result", "judge_event_outbox", "submission_fingerprint",
	}},
	{Module: "sim", Role: "chaimir_maintenance_sim", Tables: []string{
		"sim_package", "sim_package_review", "sim_session", "sim_action_log", "sim_checkpoint", "sim_share",
	}},
	{Module: "teaching", Role: "chaimir_maintenance_teaching", Tables: []string{
		"course", "chapter", "lesson", "course_member", "assignment", "assignment_item", "submission",
		"submission_judge_outbox", "submission_draft", "lesson_progress", "discussion_post", "announcement",
		"course_review", "grade_weight", "course_grade", "course_grade_export_request", "teaching_grade_event_outbox",
	}},
	{Module: "experiment", Role: "chaimir_maintenance_experiment", Tables: []string{
		"experiment", "experiment_group", "group_member", "experiment_instance", "checkpoint_result",
		"experiment_report", "experiment_score_outbox",
	}},
	{Module: "contest", Role: "chaimir_maintenance_contest", Tables: []string{
		"contest", "contest_problem", "team", "team_member", "solve_submission", "battle_entry", "battle_match",
		"ladder_rank", "contest_ladder_snapshot", "cheat_record", "vuln_source", "vuln_problem",
	}},
	{Module: "notify", Role: "chaimir_maintenance_notify", Tables: []string{
		"notification", "notification_template", "notification_preference", "system_announcement", "announcement_read",
	}},
	{Module: "grade", Role: "chaimir_maintenance_grade", Tables: []string{
		"grade_level_config", "semester", "grade_review", "grade_lock_outbox", "student_semester_grade",
		"grade_appeal", "academic_warning", "transcript_record",
	}},
	{Module: "admin", Role: "chaimir_maintenance_admin", Tables: []string{
		"audit_export_request",
	}},
}

// MaintenancePolicies 返回模块维护策略副本,供部署期授权与测试共同校验。
func MaintenancePolicies() []MaintenancePolicy {
	out := make([]MaintenancePolicy, len(maintenancePolicies))
	for index, policy := range maintenancePolicies {
		out[index] = policy
		out[index].Tables = append([]string(nil), policy.Tables...)
	}
	return out
}

// MaintenancePolicyForModule 返回规范化模块名对应的维护策略副本。
func MaintenancePolicyForModule(module string) (MaintenancePolicy, bool) {
	module = strings.TrimSpace(module)
	for _, policy := range maintenancePolicies {
		if policy.Module == module {
			policy.Tables = append([]string(nil), policy.Tables...)
			return policy, true
		}
	}
	return MaintenancePolicy{}, false
}
