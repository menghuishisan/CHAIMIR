// 路由列定义：集中维护四端资源页表格列、列优先级和用户向标题。

import type { DataColumn, DataRow } from '../types'
import { formatFileSize } from '../../utils'

export function courseColumns(): DataColumn[] {
  return [
    { key: 'name', title: '课程名称', priority: 'primary' },
    { key: 'semester', title: '学期', priority: 'secondary' },
    { key: 'credits', title: '学分', align: 'end' },
    { key: 'status', title: '状态' },
    { key: 'updated_at', title: '更新时间', priority: 'optional' },
  ]
}

export function experimentColumns(): DataColumn[] {
  return [
    { key: 'name', title: '实验名称', priority: 'primary' },
    { key: 'collab_mode', title: '协作模式' },
    { key: 'wizard_step', title: '编排步骤', align: 'end' },
    { key: 'status', title: '状态' },
    { key: 'updated_at', title: '更新时间', priority: 'optional' },
  ]
}

export function contestColumns(): DataColumn[] {
  return [
    { key: 'name', title: '竞赛名称', priority: 'primary' },
    { key: 'mode', title: '赛制' },
    { key: 'status', title: '状态' },
    { key: 'start_at', title: '开始时间' },
    { key: 'end_at', title: '结束时间', priority: 'optional' },
  ]
}

export function contestRecordColumns(): DataColumn[] {
  return [
    { key: 'contest_name', title: '竞赛名称', priority: 'primary' },
    { key: 'score', title: '得分', align: 'end' },
    { key: 'rank', title: '排名', align: 'end' },
    { key: 'contest_status', title: '竞赛状态' },
  ]
}

export function simPackageColumns(): DataColumn[] {
  return [
    { key: 'name', title: '仿真包', priority: 'primary' },
    { key: 'code', title: '编码' },
    { key: 'version', title: '版本' },
    { key: 'category', title: '分类' },
    { key: 'status', title: '状态' },
  ]
}

export function gradeSummaryColumns(): DataColumn[] {
  return [
    { key: 'student_id', title: '学生', priority: 'primary' },
    { key: 'semester_id', title: '学期' },
    { key: 'total_credits', title: '学分', align: 'end' },
    { key: 'gpa', title: '学期绩点', align: 'end' },
    { key: 'cumulative_gpa', title: '累计绩点', align: 'end' },
  ]
}

export function contentColumns(): DataColumn[] {
  return [
    { key: 'title', title: '标题', priority: 'primary' },
    { key: 'code', title: '编码' },
    { key: 'version', title: '版本' },
    { key: 'difficulty', title: '难度', align: 'end' },
    { key: 'status', title: '状态' },
  ]
}

export function paperColumns(): DataColumn[] {
  return [
    { key: 'name', title: '试卷名称', priority: 'primary' },
    { key: 'gen_mode', title: '组卷方式' },
    { key: 'created_at', title: '创建时间' },
    { key: 'updated_at', title: '更新时间', priority: 'optional' },
  ]
}

export function judgeTaskColumns(): DataColumn[] {
  return [
    { key: 'task_id', title: '任务编号', priority: 'primary' },
    { key: 'status', title: '状态' },
    { key: 'submitter_id', title: '提交人' },
    { key: 'source_ref', title: '来源' },
  ]
}

export function appealColumns(): DataColumn[] {
  return [
    { key: 'course_id', title: '课程', priority: 'primary' },
    { key: 'student_id', title: '学生' },
    { key: 'status', title: '状态' },
    { key: 'created_at', title: '提交时间' },
  ]
}

export function simReviewColumns(): DataColumn[] {
  return [
    { key: 'package_id', title: '仿真包', priority: 'primary' },
    { key: 'submitter_id', title: '提交人' },
    { key: 'result', title: '审核结果' },
    { key: 'created_at', title: '提交时间' },
  ]
}

export function accountColumns(): DataColumn[] {
  return [
    { key: 'name', title: '姓名', priority: 'primary' },
    { key: 'no', title: '学工号' },
    { key: 'phone_masked', title: '手机号' },
    { key: 'status', title: '状态' },
    { key: 'created_at', title: '创建时间', priority: 'optional' },
  ]
}

export function orgColumns(): DataColumn[] {
  return [
    { key: 'name', title: '院系名称', priority: 'primary' },
    { key: 'code', title: '编码' },
    { key: 'updated_at', title: '更新时间' },
  ]
}

export function dashboardColumns(): DataColumn[] {
  return [
    { key: 'account_count', title: '账号总数', align: 'end' },
    { key: 'course_count', title: '课程数量', align: 'end' },
    { key: 'experiment_count', title: '实验数量', align: 'end' },
    { key: 'contest_count', title: '竞赛数量', align: 'end' },
    { key: 'generated_at', title: '生成时间' },
  ]
}

export function gradeReviewColumns(): DataColumn[] {
  return [
    { key: 'course_id', title: '课程', priority: 'primary' },
    { key: 'submitter_id', title: '提交人' },
    { key: 'status', title: '状态' },
    { key: 'is_locked', title: '是否锁定' },
    { key: 'submitted_at', title: '提交时间' },
  ]
}

export function warningColumns(): DataColumn[] {
  return [
    { key: 'student_id', title: '学生', priority: 'primary' },
    { key: 'semester_id', title: '学期' },
    { key: 'type', title: '预警类型' },
    { key: 'status', title: '状态' },
    { key: 'created_at', title: '创建时间' },
  ]
}

export function tenantConfigColumns(): DataColumn[] {
  return [
    { key: 'name', title: '学校名称', priority: 'primary' },
    { key: 'code', title: '租户编码' },
    { key: 'status', title: '状态' },
    { key: 'auth_mode', title: '认证方式' },
    { key: 'enable_activation_code', title: '启用激活码' },
  ]
}

export function auditColumns(): DataColumn[] {
  return [
    { key: 'action', title: '操作', priority: 'primary' },
    { key: 'actor_id', title: '操作者' },
    { key: 'target_type', title: '对象类型' },
    { key: 'trace_id', title: '编号' },
    { key: 'created_at', title: '时间' },
  ]
}

export function tenantColumns(): DataColumn[] {
  return [
    { key: 'name', title: '学校名称', priority: 'primary' },
    { key: 'code', title: '编码' },
    { key: 'status', title: '状态' },
    { key: 'deploy_mode', title: '部署形态' },
    { key: 'expire_at', title: '到期时间' },
  ]
}

export function applicationColumns(): DataColumn[] {
  return [
    { key: 'school_name', title: '学校名称', priority: 'primary' },
    { key: 'contact_name', title: '联系人' },
    { key: 'contact_phone', title: '联系电话' },
    { key: 'status', title: '状态' },
    { key: 'submitted_at', title: '提交时间' },
  ]
}

export function runtimeColumns(): DataColumn[] {
  return [
    { key: 'name', title: '运行时', priority: 'primary' },
    { key: 'code', title: '编码' },
    { key: 'eco', title: '生态' },
    { key: 'adapter_level', title: '适配等级', align: 'end' },
    { key: 'status', title: '状态' },
  ]
}

export function toolColumns(): DataColumn[] {
  return [
    { key: 'name', title: '工具名称', priority: 'primary' },
    { key: 'code', title: '编码' },
    { key: 'kind', title: '类型' },
    { key: 'eco_tags', title: '生态标签' },
    { key: 'status', title: '状态' },
  ]
}

export function judgerColumns(): DataColumn[] {
  return [
    { key: 'name', title: '判题器', priority: 'primary' },
    { key: 'code', title: '编码' },
    { key: 'type', title: '类型' },
    { key: 'runtime_required', title: '需要运行时' },
    { key: 'status', title: '状态' },
  ]
}

export function vulnProblemColumns(): DataColumn[] {
  return [
    { key: 'title', title: '漏洞题', priority: 'primary' },
    { key: 'external_ref', title: '外部编号' },
    { key: 'level', title: '等级', align: 'end' },
    { key: 'prevalidate_status', title: '预验证' },
    { key: 'status', title: '状态' },
  ]
}

export function alertColumns(): DataColumn[] {
  return [
    { key: 'message', title: '告警内容', priority: 'primary' },
    { key: 'level', title: '级别' },
    { key: 'status', title: '状态' },
    { key: 'triggered_at', title: '触发时间' },
    { key: 'handled_at', title: '处理时间', priority: 'optional' },
  ]
}

export function monitoringColumns(): DataColumn[] {
  return [
    { key: 'name', title: '面板名称', priority: 'primary' },
    { key: 'url', title: '入口地址' },
  ]
}

export function backupColumns(): DataColumn[] {
  return [
    { key: 'id', title: '备份编号', priority: 'primary' },
    { key: 'type', title: '类型' },
    { key: 'size_bytes', title: '大小', align: 'end' },
    { key: 'status', title: '状态' },
    { key: 'started_at', title: '开始时间' },
  ]
}

export function notificationColumns(): DataColumn[] {
  return [
    { key: 'title', title: '标题', priority: 'primary' },
    { key: 'type', title: '类型' },
    { key: 'is_read', title: '已读' },
    { key: 'created_at', title: '时间' },
  ]
}

export function configColumns(): DataColumn[] {
  return [
    { key: 'key', title: '配置键', priority: 'primary' },
    { key: 'scope', title: '作用范围' },
    { key: 'version', title: '版本', align: 'end' },
    { key: 'updated_by', title: '更新人' },
    { key: 'updated_at', title: '更新时间' },
  ]
}

export function sessionColumns(): DataColumn[] {
  return [
    { key: 'device_info', title: '设备', priority: 'primary' },
    { key: 'ip', title: '网络地址' },
    { key: 'status', title: '状态' },
    { key: 'expire_at', title: '过期时间' },
    { key: 'created_at', title: '登录时间' },
  ]
}

export function outlineColumns(): DataColumn[] {
  return [
    { key: 'course', title: '课程', priority: 'primary' },
    { key: 'chapters', title: '章节' },
    { key: 'lessons', title: '课时' },
    { key: 'progress', title: '学习进度' },
  ]
}

export function lessonColumns(): DataColumn[] {
  return [
    { key: 'title', title: '课时标题', priority: 'primary' },
    { key: 'content_type', title: '内容类型' },
    { key: 'content_ref', title: '内容引用' },
    { key: 'updated_at', title: '更新时间' },
  ]
}

export function assignmentColumns(): DataColumn[] {
  return [
    { key: 'assignment', title: '作业', priority: 'primary' },
    { key: 'items', title: '题目列表' },
    { key: 'status', title: '状态' },
    { key: 'updated_at', title: '更新时间' },
  ]
}

export function submissionColumns(): DataColumn[] {
  return [
    { key: 'id', title: '提交编号', priority: 'primary' },
    { key: 'student_id', title: '学生' },
    { key: 'attempt_no', title: '次数', align: 'end' },
    { key: 'final_score', title: '最终分', align: 'end' },
    { key: 'status', title: '状态' },
    { key: 'submitted_at', title: '提交时间' },
  ]
}

export function reportColumns(): DataColumn[] {
  return [
    { key: 'id', title: '报告编号', priority: 'primary' },
    { key: 'instance_id', title: '实例' },
    { key: 'student_id', title: '学生' },
    { key: 'manual_score', title: '人工分', align: 'end' },
    { key: 'status', title: '状态' },
  ]
}

export function contestProblemColumns(): DataColumn[] {
  return [
    { key: 'id', title: '题目编号', priority: 'primary' },
    { key: 'item_code', title: '题目编码' },
    { key: 'item_version', title: '版本' },
    { key: 'score', title: '分值', align: 'end' },
    { key: 'seq', title: '顺序', align: 'end' },
  ]
}

export function transcriptColumns(): DataColumn[] {
  return [
    { key: 'id', title: '成绩单编号', priority: 'primary' },
    { key: 'student_id', title: '学生' },
    { key: 'scope', title: '范围' },
    { key: 'pdf_ref', title: '文件授权' },
    { key: 'created_at', title: '生成时间' },
  ]
}

export function chapterColumns(): DataColumn[] {
  return [
    { key: 'title', title: '章节', priority: 'primary' },
    { key: 'sort', title: '顺序', align: 'end' },
    { key: 'updated_at', title: '更新时间' },
  ]
}

export function memberColumns(): DataColumn[] {
  return [
    { key: 'student_id', title: '学生', priority: 'primary' },
    { key: 'course_id', title: '课程' },
    { key: 'status', title: '状态' },
    { key: 'joined_at', title: '加入时间' },
  ]
}

export function cheatColumns(): DataColumn[] {
  return [
    { key: 'team_id', title: '队伍', priority: 'primary' },
    { key: 'type', title: '类型' },
    { key: 'action', title: '处理动作' },
    { key: 'created_at', title: '时间' },
  ]
}

export function vulnSourceColumns(): DataColumn[] {
  return [
    { key: 'name', title: '漏洞源', priority: 'primary' },
    { key: 'type', title: '类型' },
    { key: 'default_level', title: '默认等级', align: 'end' },
    { key: 'enabled', title: '启用' },
    { key: 'last_sync_at', title: '最近同步' },
  ]
}

export function importBatchColumns(): DataColumn[] {
  return [
    { key: 'id', title: '批次编号', priority: 'primary' },
    { key: 'target_type', title: '账号类型' },
    { key: 'success_count', title: '成功数', align: 'end' },
    { key: 'failed_count', title: '失败数', align: 'end' },
    { key: 'created_at', title: '创建时间' },
  ]
}

export function levelConfigColumns(): DataColumn[] {
  return [
    { key: 'name', title: '配置名称', priority: 'primary' },
    { key: 'mapping', title: '等级映射' },
    { key: 'warning_rules', title: '预警规则' },
    { key: 'is_default', title: '默认' },
    { key: 'updated_at', title: '更新时间' },
  ]
}

export function ssoColumns(): DataColumn[] {
  return [
    { key: 'type', title: '认证类型', priority: 'primary' },
    { key: 'match_field', title: '匹配字段' },
    { key: 'enabled', title: '启用' },
  ]
}

export function statisticsColumns(): DataColumn[] {
  return [
    { key: 'date', title: '日期', priority: 'primary' },
    { key: 'metric', title: '指标' },
    { key: 'value', title: '数值', align: 'end' },
  ]
}

export function runtimeImageColumns(): DataColumn[] {
  return [
    { key: 'image_url', title: '镜像地址', priority: 'primary' },
    { key: 'version', title: '版本' },
    { key: 'digest', title: '摘要' },
    { key: 'is_default', title: '默认' },
  ]
}

export function quotaColumns(): DataColumn[] {
  return [
    { key: 'tenant_id', title: '租户', priority: 'primary' },
    { key: 'max_concurrent_sandbox', title: '并发沙箱', align: 'end' },
    { key: 'max_cpu', title: 'CPU', align: 'end' },
    { key: 'max_memory_mb', title: '内存', align: 'end' },
    { key: 'idle_timeout_min', title: '空闲超时', align: 'end' },
  ]
}

export function announcementColumns(): DataColumn[] {
  return [
    { key: 'title', title: '公告标题', priority: 'primary' },
    { key: 'scope', title: '范围' },
    { key: 'is_read', title: '已读' },
    { key: 'created_at', title: '发布时间' },
  ]
}

/**
 * normalizeObject 的后置修正：备份大小使用用户可读单位。
 */
export function normalizeBackupSize(row: DataRow): DataRow {
  const size = Number(row.size_bytes)
  return Number.isFinite(size) ? { ...row, size_bytes: formatFileSize(size) } : row
}
