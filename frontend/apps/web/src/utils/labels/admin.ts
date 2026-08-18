// admin labels 文件维护 M9 管理后台枚举与审计动作的用户向文案。

import {
  AlertLevel,
  AlertStatus,
  AuditActorRole,
  BackupStatus,
  BackupType,
} from '@chaimir/api-client'

const ALERT_STATUS_LABELS: Record<AlertStatus, string> = {
  [AlertStatus.PENDING]: '待处理',
  [AlertStatus.HANDLED]: '已处理',
  [AlertStatus.IGNORED]: '已忽略',
}

/** alertStatusLabel 返回告警处理状态文案。 */
export function alertStatusLabel(status: AlertStatus): string {
  return ALERT_STATUS_LABELS[status]
}

/** 告警级别文案与后端 1 提示、2 警告、3 严重、4 紧急的闭集契约一致。 */
const ALERT_LEVEL_LABELS: Record<AlertLevel, string> = {
  [AlertLevel.NOTICE]: '提示',
  [AlertLevel.WARNING]: '警告',
  [AlertLevel.SEVERE]: '严重',
  [AlertLevel.CRITICAL]: '紧急',
}

/** alertLevelLabel 返回告警级别文案。 */
export function alertLevelLabel(level: AlertLevel): string {
  return ALERT_LEVEL_LABELS[level]
}

const BACKUP_TYPE_LABELS: Record<BackupType, string> = {
  [BackupType.FULL]: '全量备份',
}

/** backupTypeLabel 返回备份类型文案。 */
export function backupTypeLabel(type: BackupType): string {
  return BACKUP_TYPE_LABELS[type]
}

const BACKUP_STATUS_LABELS: Record<BackupStatus, string> = {
  [BackupStatus.RUNNING]: '备份中',
  [BackupStatus.SUCCEEDED]: '备份成功',
  [BackupStatus.FAILED]: '备份失败',
}

/** backupStatusLabel 返回备份状态文案。 */
export function backupStatusLabel(status: BackupStatus): string {
  return BACKUP_STATUS_LABELS[status]
}

const ACTOR_ROLE_LABELS: Record<AuditActorRole, string> = {
  [AuditActorRole.PLATFORM_ADMIN]: '平台管理员',
  [AuditActorRole.SCHOOL_ADMIN]: '学校管理员',
  [AuditActorRole.TEACHER]: '教师',
  [AuditActorRole.STUDENT]: '学生',
  [AuditActorRole.SYSTEM]: '系统',
}

/** auditActorRoleLabel 返回审计操作者角色文案。 */
export function auditActorRoleLabel(role: AuditActorRole): string {
  return ACTOR_ROLE_LABELS[role]
}

/**
 * 审计动作文案。action 是后端各模块自取的开放字符串(如 teaching.assignment.submit),
 * 界面必须显示中文动作名而不是内部标识(旧前端直出 auth.login 是 P0 违规)。
 * 未登记的动作按「模块 · 操作」拆解显示,仍不暴露原始点分标识。
 */
const AUDIT_ACTION_LABELS: Record<string, string> = {
  'auth.login': '登录',
  'auth.logout': '退出登录',
  'auth.password.reset': '重置密码',
  'auth.activate': '激活账号',
  'account.create': '创建账号',
  'account.update': '修改账号资料',
  'account.disable': '停用账号',
  'account.enable': '启用账号',
  'account.archive': '归档账号',
  'account.restore': '恢复账号',
  'account.cancel': '注销账号',
  'account.force_logout': '强制下线',
  'account.reset_password': '重置账号密码',
  'account.grant_admin': '授予管理员',
  'account.revoke_admin': '撤销管理员',
  'account.import': '批量导入账号',
  'org.import': '导入组织架构',
  'tenant.config.update': '修改学校配置',
  'tenant.sso.upsert': '配置统一认证',
  'teaching.course.create': '创建课程',
  'teaching.course.publish': '发布课程',
  'teaching.course.archive': '归档课程',
  'teaching.assignment.submit': '提交作业',
  'teaching.assignment.grade': '批改作业',
  'teaching.grade.override': '人工调整成绩',
  'contest.team.join': '加入竞赛队伍',
  'contest.team.lock': '锁定竞赛队伍',
  'contest.cheat.record': '记录违规处理',
  'grade.review.submit': '提交成绩审核',
  'grade.review.approve': '通过成绩审核',
  'grade.review.reject': '驳回成绩审核',
  'grade.review.unlock': '解锁成绩审核',
  'grade.appeal.create': '提交成绩申诉',
  'grade.appeal.accept': '受理成绩申诉',
  'grade.appeal.reject': '驳回成绩申诉',
  'grade.transcript.download': '下载成绩单',
  'content.attachment.upload': '上传题库附件',
  'content.attachment.download': '下载题库附件',
  'sim.review.approve': '通过仿真包审核',
  'sim.review.reject': '退回仿真包',
  'admin.config.update': '修改系统配置',
  'admin.config.rollback': '回滚系统配置',
  'admin.alert.handle': '处理告警',
  'platform.tenant.update': '修改学校信息',
  'platform.application.approve': '通过入驻申请',
  'platform.application.reject': '驳回入驻申请',
}

/** 审计动作里的模块段文案:未登记动作时用它拼出可读说明。 */
const AUDIT_MODULE_LABELS: Record<string, string> = {
  auth: '登录认证',
  account: '账号',
  org: '组织架构',
  tenant: '学校配置',
  teaching: '教学',
  experiment: '实验',
  contest: '竞赛',
  content: '题库',
  sim: '仿真',
  judge: '判题',
  grade: '成绩',
  notify: '通知',
  admin: '系统管理',
  platform: '平台',
  sandbox: '沙箱',
}

/** auditActionLabel 返回审计动作的中文说明,未登记动作按模块拼出可读名。 */
export function auditActionLabel(action: string): string {
  const known = AUDIT_ACTION_LABELS[action]
  if (known) return known
  const [moduleKey] = action.split('.')
  const moduleName = AUDIT_MODULE_LABELS[moduleKey]
  return moduleName ? `${moduleName}操作` : '系统操作'
}

/**
 * 审计对象类型文案。target_type 也是开放字符串,同样不直出。
 */
const AUDIT_TARGET_LABELS: Record<string, string> = {
  account: '账号',
  tenant: '学校',
  course: '课程',
  assignment: '作业',
  submission: '作业提交',
  experiment: '实验',
  contest: '竞赛',
  'contest.team': '竞赛队伍',
  'grade.review': '成绩审核',
  'grade.appeal': '成绩申诉',
  'grade.transcript': '成绩单',
  'content.item': '题目',
  'sim.package': '仿真包',
  config: '系统配置',
  alert: '告警',
  session: '登录会话',
}

/** auditTargetTypeLabel 返回审计对象类型文案。 */
export function auditTargetTypeLabel(targetType: string): string {
  return AUDIT_TARGET_LABELS[targetType] ?? '系统对象'
}

/**
 * 看板指标的用户向名称。metrics 是后端按日聚合写入的开放对象,
 * 只呈现已登记的键 —— 未登记键不猜语义,不把内部键名抛到界面上。
 */
const STATISTICS_METRIC_LABELS: Record<string, string> = {
  account_count: '账号总数',
  active_account_count: '活跃账号',
  new_account_count: '新增账号',
  course_count: '课程数',
  active_course_count: '进行中课程',
  experiment_count: '实验数',
  active_instance_count: '活跃实验环境',
  contest_count: '竞赛数',
  active_contest_count: '进行中竞赛',
  active_sandbox_count: '活跃沙箱',
  submission_count: '作业提交数',
  judge_task_count: '判题任务数',
  tenant_count: '学校数',
  learning_duration_sec: '累计学习时长',
}

/** statisticsMetricLabel 返回统计指标名称;未登记键返回 undefined 由调用方跳过。 */
export function statisticsMetricLabel(key: string): string | undefined {
  return STATISTICS_METRIC_LABELS[key]
}

/**
 * 系统配置项的用户向名称与说明。
 * 配置值是 JSONB,界面按已登记的配置项渲染显式表单,不给裸 JSON 文本域。
 */
const CONFIG_KEY_LABELS: Record<string, { name: string; description: string }> = {
  'sandbox.default_quota': {
    name: '沙箱默认配额',
    description: '新学校开通时的沙箱并发与资源上限',
  },
  'judge.concurrency': { name: '判题并发数', description: '同时执行的判题任务上限' },
  'notify.retention_days': { name: '站内信保留天数', description: '超过后自动清理站内消息' },
  'transfer.max_attempts': { name: '导入导出重试次数', description: '任务失败后的最大重试次数' },
  'grade.summary_cache_ttl': { name: '成绩概览缓存时长', description: '学业概览的缓存有效期(秒)' },
}

/** configKeyLabel 返回配置项名称;未登记的键显示原键名(平台管理员能理解技术键)。 */
export function configKeyLabel(key: string): string {
  return CONFIG_KEY_LABELS[key]?.name ?? key
}

/** configKeyDescription 返回配置项说明;未登记返回 undefined。 */
export function configKeyDescription(key: string): string | undefined {
  return CONFIG_KEY_LABELS[key]?.description
}

/**
 * 告警指标的用户向名称。metric 是后端开放字符串,规则表单从已登记指标里选,
 * 不让管理员手写指标名(写错要等到告警不触发才发现)。
 */
const ALERT_METRIC_LABELS: Record<string, string> = {
  'sandbox.active_count': '活跃实验环境数',
  'sandbox.failed_rate': '实验环境创建失败率',
  'judge.queue_length': '待评分任务数量',
  'judge.failed_rate': '自动评分失败率',
  'account.login_failed_count': '登录失败次数',
  'transfer.failed_count': '导入导出失败数',
  'grade.warning_count': '学业预警数',
}

/** alertMetricLabel 返回告警指标名称;未登记指标显示原值(便于排查配置)。 */
export function alertMetricLabel(metric: string): string {
  return ALERT_METRIC_LABELS[metric] ?? metric
}

/**
 * 告警条件的结构化键。condition 是 JSONB,规则表单按「比较方式 + 阈值 + 持续时间」
 * 三个显式字段组装,不给裸 JSON。
 */
const ALERT_CONDITION_OPERATOR_LABELS: Record<string, string> = {
  gt: '大于',
  gte: '大于或等于',
  lt: '小于',
  lte: '小于或等于',
}

/** alertConditionOperatorLabel 返回比较方式文案。 */
export function alertConditionOperatorLabel(operator: string): string {
  return ALERT_CONDITION_OPERATOR_LABELS[operator] ?? operator
}
