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
  // 登录与账号
  'auth.login': '登录',
  'auth.logout': '退出登录',
  'auth.refresh.replay': '登录凭证被重复使用,已强制下线',
  'account.create': '创建账号',
  'account.update': '修改账号资料',
  'account.status.update': '更新账号状态',
  'account.force_logout': '强制下线',
  'account.password.reset': '重置账号密码',
  'account.password.change': '修改自己的密码',
  'account.phone.change': '更换绑定手机',
  'account.admin.grant': '授予学校管理员',
  'account.admin.revoke': '撤销学校管理员',
  'account.import': '批量导入账号',
  'import.preview.discard': '放弃导入预览',
  'platform_admin.password.change': '平台管理员修改密码',
  // 组织架构
  'org.import': '导入组织架构',
  'org.department.create': '新建院系',
  'org.department.update': '修改院系',
  'org.department.delete': '删除院系',
  'org.major.create': '新建专业',
  'org.major.update': '修改专业',
  'org.major.delete': '删除专业',
  'org.class.create': '新建班级',
  'org.class.update': '修改班级',
  'org.class.delete': '删除班级',
  'org.class.promote': '班级升届',
  'org.class.archive': '整届归档',
  // 学校与入驻
  'tenant.config.update': '修改学校配置',
  'tenant.sso.update': '修改统一认证配置',
  'tenant.logo.upload': '上传学校标志',
  'tenant.logo.clear': '移除学校标志',
  'tenant.status.update': '更新学校状态',
  'tenant.application.approve': '通过入驻申请',
  'tenant.application.reject': '驳回入驻申请',
  'tenant.bootstrap': '开通学校',
  // 教学
  'teaching.course.create': '创建课程',
  'teaching.course.clone': '复制课程',
  'teaching.course.publish': '发布课程',
  'teaching.course.end': '结束课程',
  'teaching.course.archive': '归档课程',
  'teaching.course.share': '共享课程',
  'teaching.course.unshare': '取消共享课程',
  'teaching.course.cover.upload': '上传课程封面',
  'teaching.lesson.material.upload': '上传课时材料',
  'teaching.lesson.material.access': '查看课时材料',
  'teaching.assignment.create': '布置作业',
  'teaching.assignment.submit': '提交作业',
  'teaching.grade.override': '人工调整课程成绩',
  // 实验
  'experiment.create': '创建实验',
  'experiment.update': '修改实验',
  'experiment.publish': '发布实验',
  'experiment.unpublish': '撤回实验',
  'experiment.group.create': '创建实验小组',
  'experiment.group.member.upsert': '调整小组成员',
  'experiment.group.member.remove': '移出小组成员',
  'experiment.instance.create': '开始实验',
  'experiment.instance.finish': '完成实验',
  'experiment.report.submit': '提交实验报告',
  'experiment.report.grade': '批改实验报告',
  'experiment.report.download': '下载实验报告',
  // 竞赛
  'contest.create': '创建竞赛',
  'contest.update': '修改竞赛',
  'contest.publish': '发布竞赛',
  'contest.start': '开始竞赛',
  'contest.freeze': '封榜',
  'contest.end': '结束竞赛',
  'contest.archive': '归档竞赛',
  'contest.archive.auto': '自动归档竞赛',
  'contest.lifecycle': '竞赛按时间自动推进',
  'contest.problem.upsert': '调整竞赛赛题',
  'contest.signup': '报名竞赛',
  'contest.team.join': '加入竞赛队伍',
  'contest.team.lock': '锁定竞赛队伍',
  'contest.submit': '提交解题答案',
  'contest.battle.entry.submit': '提交参战程序',
  'contest.battle.finish': '对局结束结算',
  'contest.battle.replay.download_grant': '下载对局回放',
  'contest.cheat.record': '记录违规处理',
  'contest.vuln_source.upsert': '保存漏洞题源',
  'contest.vuln_source.sync': '同步漏洞题源',
  'contest.vuln_problem.import': '导入漏洞赛题',
  'contest.vuln_problem.prevalidate': '预验证漏洞赛题',
  'contest.vuln_problem.finalize': '确认漏洞赛题',
  // 题库
  'content.create': '新建题目',
  'content.update': '修改题目',
  'content.new_version': '题目出新版本',
  'content.clone': '复制题目',
  'content.publish': '发布题目',
  'content.deprecate': '停用题目',
  'content.delete': '删除题目',
  'content.share': '共享题目',
  'content.unshare': '取消共享题目',
  'content.system_import': '系统导入题目',
  'content.category.create': '新建题目分类',
  'content.category.update': '修改题目分类',
  'content.category.delete': '删除题目分类',
  'content.paper.create': '组卷',
  'content.paper.regenerate': '重新抽题组卷',
  'content.attachment.upload': '上传题库附件',
  'content.attachment.download': '下载题库附件',
  // 成绩
  'grade.review.submit': '报送成绩审核',
  'grade.review.approve': '通过成绩审核',
  'grade.review.reject': '退回成绩审核',
  'grade.review.unlock': '解锁成绩审核',
  'grade.appeal.create': '提交成绩申诉',
  'grade.appeal.accept': '受理成绩申诉',
  'grade.appeal.reject': '驳回成绩申诉',
  'grade.transcript.generate': '生成成绩单',
  'grade.transcript.download': '下载成绩单',
  // 判题
  'judge.submit': '提交判题',
  'judge.cancel': '取消判题',
  'judge.rejudge': '重新判题',
  'judge.rejudge.user': '申请重新判题',
  'judge.rejudge_batch': '整批重新判题',
  'judge.manual_score': '人工评分',
  'judge.judger.upsert': '登记判题器',
  'judge.judger.update': '修改判题器',
  'judge.judger.selftest': '判题器自检',
  'judge.catalog.reconcile': '校准判题器目录',
  // 实验环境与运行时
  'sandbox.create': '创建实验环境',
  'sandbox.pause': '暂停实验环境',
  'sandbox.resume': '恢复实验环境',
  'sandbox.resume.snapshot': '从快照恢复实验环境',
  'sandbox.recycle': '回收实验环境',
  'sandbox.runtime.upsert': '登记链运行时',
  'sandbox.runtime.update': '修改链运行时',
  'sandbox.runtime.selftest': '链运行时接入自检',
  'sandbox.image.register': '登记镜像版本',
  'sandbox.image.disable': '停用镜像版本',
  'sandbox.image.prepull': '预拉取镜像',
  'sandbox.tool.upsert': '保存实验工具',
  'sandbox.quota.upsert': '调整学校资源配额',
  // 仿真
  'sim.package.submit': '提交仿真场景',
  'sim.package.update': '更新仿真场景',
  'sim.package.approve': '通过仿真场景审核',
  'sim.package.reject': '退回仿真场景',
  'sim.package.archive': '下架仿真场景',
  'sim.package.republish': '重新上架仿真场景',
  'sim.session.create': '开始仿真',
  'sim.session.archive': '结束仿真',
  // 公告与系统
  'notify.announcement.publish': '发布公告',
  'admin.config.update': '修改系统配置',
  'admin.config.rollback': '回滚系统配置',
  'admin.alert.handle': '处理告警',
  'admin.alert_rule.create': '新建告警规则',
  'admin.alert_rule.update': '修改告警规则',
  'admin.audit.export': '导出审计日志',
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
  'identity.account': '账号',
  'identity.tenant': '学校',
  'identity.tenant_application': '入驻申请',
  'identity.department': '院系',
  'identity.major': '专业',
  'identity.class': '班级',
  'identity.import_batch': '导入批次',
  'identity.import_preview': '导入预览',
  'identity.sso_config': '统一认证配置',
  'identity.auth_session': '登录会话',
  'identity.platform_auth_session': '平台登录会话',
  'identity.platform_admin': '平台管理员',
  'teaching.course': '课程',
  'teaching.chapter': '课程章节',
  'teaching.lesson': '课时',
  'teaching.assignment': '作业',
  'teaching.submission': '作业提交',
  'teaching.progress': '学习进度',
  'teaching.discussion': '课程讨论',
  'teaching.announcement': '课程公告',
  'teaching.review': '课程评价',
  'teaching.grade': '课程成绩',
  experiment: '实验',
  experiment_instance: '实验记录',
  experiment_group: '实验小组',
  experiment_report: '实验报告',
  contest: '竞赛',
  contest_team: '竞赛队伍',
  solve_submission: '解题提交',
  battle_entry: '参战程序',
  battle_match: '对局',
  cheat_record: '违规记录',
  vuln_source: '漏洞题源',
  vuln_problem: '漏洞赛题',
  'content.item': '题目',
  'content.category': '题目分类',
  'content.paper': '试卷',
  'grade.review': '成绩审核',
  'grade.appeal': '成绩申诉',
  'grade.transcript': '成绩单',
  judge_task: '判题任务',
  judge_source: '判题来源',
  judger: '判题器',
  sandbox: '实验环境',
  runtime: '链运行时',
  runtime_image: '镜像版本',
  tool: '实验工具',
  tenant_quota: '资源配额',
  sim_package: '仿真场景',
  sim_session: '仿真记录',
  'notify.announcement': '公告',
  system_config: '系统配置',
  alert_event: '告警',
  alert_rule: '告警规则',
  audit_log: '审计日志',
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
  'sandbox.pending_seconds': '实验环境等待时间',
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
