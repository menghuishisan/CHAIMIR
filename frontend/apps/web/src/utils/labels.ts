// labels 文件集中维护前端页面的枚举与状态文案，避免页面内重复定义展示映射。

import {
  AccountStatus,
  AssignmentStatus,
  AlertStatus,
  ApplicationStatus,
  AuthMode,
  BackupStatus,
  BackupType,
  BattleRole,
  ContentDifficulty,
  ContentStatus,
  ContentType,
  ContentVisibility,
  ContestMode,
  ContestStatus,
  CourseStatus,
  CourseType,
  DeployMode,
  ExperimentCollabMode,
  ExperimentStatus,
  GradingMode,
  GradeAppealStatus,
  GradeReviewStatus,
  GradeWarningType,
  ImagePrepullStatus,
  ImportBatchStatus,
  JoinMode,
  JudgerStatus,
  JudgerType,
  LatePolicy,
  LessonContentType,
  MatchMode,
  PaperMode,
  RuntimeSelftestStatus,
  RuntimeStatus,
  SandboxStatus,
  SandboxToolKind,
  SessionStatus,
  SsoMatchField,
  SsoType,
  TeachingDifficulty,
  TeamMode,
  TenantStatus,
  ToolStatus,
  UserRole,
  VulnLevel,
  VulnRuntimeMode,
  SIM_COMPUTE,
  SIM_REVIEW_RESULT,
  type SandboxQuota,
  type SimCompute,
  type SimReviewResult,
  type TransferTask,
} from '@chaimir/api-client'

/**
 * labelFromMap 按字符串键读取文案，缺省时暴露未识别值，避免掩盖前后端枚举不一致。
 */
function labelFromMap(value: unknown, labels: Record<string, string>, fallback: string): string {
  const key = String(value)
  return Object.prototype.hasOwnProperty.call(labels, key) ? labels[key] : `${fallback}（${key}）`
}

/**
 * courseTypeLabel 返回课程类型文案。
 */
export function courseTypeLabel(type: CourseType): string {
  return labelFromMap(type, {
    [CourseType.THEORY]: '理论课',
    [CourseType.LAB]: '实验课',
    [CourseType.MIXED]: '混合课',
    [CourseType.PROJECT]: '项目课',
  }, '未识别的课程类型')
}

/**
 * teachingDifficultyLabel 返回教学课程难度文案。
 */
export function teachingDifficultyLabel(difficulty: TeachingDifficulty): string {
  return labelFromMap(difficulty, {
    [TeachingDifficulty.INTRO]: '入门',
    [TeachingDifficulty.ADVANCED]: '进阶',
    [TeachingDifficulty.EXPERT]: '高阶',
    [TeachingDifficulty.RESEARCH]: '研究型',
  }, '未识别的课程难度')
}

/**
 * courseStatusLabel 返回课程状态的教师端展示文案。
 */
export function courseStatusLabel(status: CourseStatus): string {
  return labelFromMap(status, {
    [CourseStatus.DRAFT]: '草稿',
    [CourseStatus.PUBLISHED]: '已发布',
    [CourseStatus.RUNNING]: '进行中',
    [CourseStatus.ENDED]: '已结课',
    [CourseStatus.ARCHIVED]: '已归档',
  }, '未知')
}

/**
 * assignmentStatusLabel 返回作业发布状态文案。
 */
export function assignmentStatusLabel(status: AssignmentStatus): string {
  return labelFromMap(status, {
    [AssignmentStatus.DRAFT]: '草稿',
    [AssignmentStatus.PUBLISHED]: '已发布',
  }, '未知')
}

/**
 * joinModeLabel 返回课程成员加入方式文案。
 */
export function joinModeLabel(mode: JoinMode): string {
  return labelFromMap(mode, {
    [JoinMode.INVITE]: '邀请码加入',
    [JoinMode.TEACHER]: '教师添加',
  }, '未知')
}

/**
 * contestStatusLabel 返回竞赛生命周期状态文案。
 */
export function contestStatusLabel(status: ContestStatus, draftLabel = '草稿'): string {
  return labelFromMap(status, {
    [ContestStatus.DRAFT]: draftLabel,
    [ContestStatus.SIGNUP]: '报名中',
    [ContestStatus.RUNNING]: '进行中',
    [ContestStatus.FROZEN]: '封榜中',
    [ContestStatus.ENDED]: '已结束',
    [ContestStatus.ARCHIVED]: '已归档',
  }, draftLabel)
}

/**
 * contestModeLabel 返回竞赛模式文案。
 */
export function contestModeLabel(mode: ContestMode): string {
  return labelFromMap(mode, {
    [ContestMode.SOLVE]: '解题赛',
    [ContestMode.BATTLE]: '对抗赛',
  }, '未识别的竞赛模式')
}

/**
 * teamModeLabel 返回竞赛组队方式文案。
 */
export function teamModeLabel(mode: TeamMode): string {
  return labelFromMap(mode, {
    [TeamMode.SOLO]: '个人参赛',
    [TeamMode.GROUP]: '团队参赛',
  }, '未识别的组队方式')
}

/**
 * matchModeLabel 返回对抗赛匹配模式文案。
 */
export function matchModeLabel(mode: MatchMode): string {
  return labelFromMap(mode, {
    [MatchMode.ROUND_ROBIN]: '循环赛',
    [MatchMode.ELO]: '积分匹配',
  }, '未识别的对局模式')
}

/**
 * battleRoleLabel 返回对抗赛参战角色文案。
 */
export function battleRoleLabel(role: BattleRole): string {
  return labelFromMap(role, {
    [BattleRole.ATTACK]: '攻击',
    [BattleRole.DEFENSE]: '防守',
    [BattleRole.STRATEGY]: '策略',
  }, '未识别的参战角色')
}

/**
 * vulnLevelLabel 返回漏洞题等级文案。
 */
export function vulnLevelLabel(level: VulnLevel): string {
  return labelFromMap(level, {
    [VulnLevel.A]: 'A',
    [VulnLevel.B]: 'B',
    [VulnLevel.C]: 'C',
  }, '未识别的漏洞等级')
}

/**
 * vulnRuntimeModeLabel 返回漏洞题运行方式文案。
 */
export function vulnRuntimeModeLabel(mode: VulnRuntimeMode): string {
  return labelFromMap(mode, {
    [VulnRuntimeMode.ISOLATED]: '隔离环境',
    [VulnRuntimeMode.FORKED]: '主网分叉',
  }, '未识别的运行方式')
}

/**
 * experimentStatusLabel 返回实验编排发布状态文案。
 */
export function experimentStatusLabel(status: ExperimentStatus): string {
  return labelFromMap(status, {
    [ExperimentStatus.PUBLISHED]: '已发布',
    [ExperimentStatus.UNPUBLISHED]: '已下架',
  }, '草稿')
}

/**
 * experimentCollabModeLabel 返回实验协作模式文案。
 */
export function experimentCollabModeLabel(mode: ExperimentCollabMode): string {
  return labelFromMap(mode, {
    [ExperimentCollabMode.SOLO]: '个人实验',
    [ExperimentCollabMode.GROUP]: '小组实验',
  }, '未识别的协作模式')
}

/**
 * gradeReviewStatusLabel 返回成绩审核状态文案。
 */
export function gradeReviewStatusLabel(status: GradeReviewStatus): string {
  return labelFromMap(status, {
    [GradeReviewStatus.PENDING]: '待审核',
    [GradeReviewStatus.APPROVED]: '已通过',
    [GradeReviewStatus.REJECTED]: '已驳回',
  }, '未知')
}

/**
 * gradeAppealStatusLabel 返回成绩申诉状态文案。
 */
export function gradeAppealStatusLabel(status: GradeAppealStatus): string {
  return labelFromMap(status, {
    [GradeAppealStatus.PENDING]: '待处理',
    [GradeAppealStatus.ACCEPTED]: '已受理',
    [GradeAppealStatus.COMPLETED]: '已完成',
    [GradeAppealStatus.REJECTED]: '已驳回',
  }, '未知')
}

/**
 * gradeWarningTypeLabel 返回学业预警类型文案。
 */
export function gradeWarningTypeLabel(type: GradeWarningType): string {
  return labelFromMap(type, {
    [GradeWarningType.FAILED_COURSE]: '课程未通过',
    [GradeWarningType.LOW_GPA]: '绩点偏低',
  }, '学业预警')
}

/**
 * gradeWarningStatusLabel 返回学业预警确认状态文案。
 */
export function gradeWarningStatusLabel(status: number): string {
  return labelFromMap(status, {
    1: '待确认',
    2: '已确认',
  }, '未知')
}

/**
 * gradeWarningDetailLabel 将预警详情对象转为用户向说明。
 */
export function gradeWarningDetailLabel(detail?: Record<string, unknown>): string {
  if (!detail || Object.keys(detail).length === 0) return '已记录预警触发条件'
  const labels: Record<string, string> = {
    fail_count: '未通过课程数',
    min_gpa: '最低绩点要求',
    gpa: '当前绩点',
    course_id: '课程编号',
  }
  return Object.entries(detail)
    .map(([key, value]) => `${labels[key] || '触发参数'}：${String(value)}`)
    .join('，')
}

/**
 * contentTypeLabel 返回内容中心资源类型文案。
 */
export function contentTypeLabel(type: ContentType): string {
  return labelFromMap(type, {
    [ContentType.EXPERIMENT_TEMPLATE]: '实验模板',
    [ContentType.CONTEST_PROBLEM]: '竞赛题',
    [ContentType.THEORY_QUESTION]: '理论题',
  }, '资源')
}

/**
 * contentDifficultyLabel 返回内容难度文案。
 */
export function contentDifficultyLabel(value: ContentDifficulty): string {
  return labelFromMap(value, {
    [ContentDifficulty.INTRO]: '入门',
    [ContentDifficulty.BASIC]: '基础',
    [ContentDifficulty.ADVANCED]: '进阶',
    [ContentDifficulty.CHALLENGE]: '挑战',
  }, '未知')
}

/**
 * contentVisibilityLabel 返回内容可见范围文案。
 */
export function contentVisibilityLabel(value: ContentVisibility): string {
  return labelFromMap(value, {
    [ContentVisibility.PRIVATE]: '仅自己可见',
    [ContentVisibility.TENANT]: '校内可见',
    [ContentVisibility.SHARED]: '共享库可见',
  }, '未识别的可见范围')
}

/**
 * contentStatusLabel 返回内容发布状态文案。
 */
export function contentStatusLabel(status: ContentStatus): string {
  return labelFromMap(status, {
    [ContentStatus.DRAFT]: '草稿',
    [ContentStatus.PUBLISHED]: '已发布',
    [ContentStatus.DEPRECATED]: '已停用',
  }, '未识别的内容状态')
}

/**
 * paperModeLabel 返回试卷组卷模式文案。
 */
export function paperModeLabel(mode: PaperMode): string {
  return labelFromMap(mode, {
    [PaperMode.MANUAL]: '手动组卷',
    [PaperMode.RANDOM]: '规则组卷',
  }, '未识别的组卷模式')
}

/**
 * lessonContentTypeLabel 返回课程小节内容类型文案。
 */
export function lessonContentTypeLabel(type: LessonContentType): string {
  return labelFromMap(type, {
    [LessonContentType.VIDEO]: '视频',
    [LessonContentType.MARKDOWN]: '图文',
    [LessonContentType.ATTACHMENT]: '附件',
    [LessonContentType.EXPERIMENT]: '实验',
    [LessonContentType.SIMULATION]: '仿真',
  }, '未识别的内容类型')
}

/**
 * latePolicyLabel 返回作业逾期策略文案。
 */
export function latePolicyLabel(policy: LatePolicy): string {
  return labelFromMap(policy, {
    [LatePolicy.REJECT]: '不允许逾期提交',
    [LatePolicy.PENALIZE]: '逾期扣分',
    [LatePolicy.NO_PENALTY]: '允许补交',
  }, '未识别的逾期策略')
}

/**
 * gradingModeLabel 返回作业评分方式文案。
 */
export function gradingModeLabel(mode: GradingMode): string {
  return labelFromMap(mode, {
    [GradingMode.AUTO]: '自动评分',
    [GradingMode.MANUAL]: '人工评分',
  }, '未识别的评分方式')
}

/**
 * transferTaskStatusLabel 返回导入导出任务状态文案。
 */
export function transferTaskStatusLabel(status: TransferTask['status']): string {
  return labelFromMap(status, {
    pending: '等待处理',
    running: '处理中',
    retrying: '准备重试',
    succeeded: '已完成',
    failed: '处理失败',
  }, '未知')
}

/**
 * accountRoleLabel 按优先级返回账号角色文案。
 */
export function accountRoleLabel(roles: UserRole[]): string {
  if (roles.includes(UserRole.PLATFORM_ADMIN)) return '平台管理员'
  if (roles.includes(UserRole.SCHOOL_ADMIN)) return '学校管理员'
  if (roles.includes(UserRole.TEACHER)) return '教师'
  if (roles.includes(UserRole.STUDENT)) return '学生'
  return '未分配'
}

/**
 * baseIdentityLabel 返回账号基础身份文案。
 */
export function baseIdentityLabel(identity: number): string {
  return labelFromMap(identity, {
    1: '学生',
    2: '教师',
  }, '未识别的基础身份')
}

/**
 * accountStatusLabel 返回账号生命周期状态文案。
 */
export function accountStatusLabel(status: AccountStatus): string {
  return labelFromMap(status, {
    [AccountStatus.PENDING]: '待激活',
    [AccountStatus.ACTIVE]: '正常',
    [AccountStatus.DISABLED]: '已停用',
    [AccountStatus.ARCHIVED]: '已归档',
    [AccountStatus.CANCELLED]: '已注销',
  }, '未知')
}

/**
 * sessionStatusLabel 返回登录会话状态文案。
 */
export function sessionStatusLabel(status: SessionStatus): string {
  return labelFromMap(status, {
    [SessionStatus.ACTIVE]: '有效',
    [SessionStatus.REVOKED]: '已失效',
  }, '未知')
}

/**
 * importBatchStatusLabel 返回账号导入批次状态文案。
 */
export function importBatchStatusLabel(status: ImportBatchStatus): string {
  return labelFromMap(status, {
    [ImportBatchStatus.PROCESSING]: '处理中',
    [ImportBatchStatus.COMPLETED]: '已完成',
    [ImportBatchStatus.FAILED]: '处理失败',
  }, '未知')
}

/**
 * ssoTypeLabel 返回统一认证类型文案。
 */
export function ssoTypeLabel(type: SsoType): string {
  return labelFromMap(type, {
    [SsoType.CAS]: 'CAS',
    [SsoType.LDAP]: 'LDAP',
  }, '未识别的认证类型')
}

/**
 * ssoMatchFieldLabel 返回统一认证匹配字段文案。
 */
export function ssoMatchFieldLabel(field: SsoMatchField): string {
  return labelFromMap(field, {
    [SsoMatchField.NO]: '学号工号',
    [SsoMatchField.PHONE]: '手机号',
  }, '未识别的匹配字段')
}

/**
 * classStatusLabel 返回班级状态文案。
 */
export function classStatusLabel(status: number): string {
  return labelFromMap(status, {
    1: '正常',
    2: '已归档',
  }, '未识别的班级状态')
}

/**
 * announcementScopeLabel 返回公告覆盖范围文案。
 */
export function announcementScopeLabel(scope: number): string {
  return labelFromMap(scope, {
    1: '全平台可见',
    2: '全校可见',
    3: '按角色可见',
  }, '未识别的公告范围')
}

/**
 * tenantStatusLabel 返回租户运行状态文案。
 */
export function tenantStatusLabel(status: TenantStatus): string {
  return labelFromMap(status, {
    [TenantStatus.ACTIVE]: '运营中',
    [TenantStatus.DISABLED]: '已停用',
    [TenantStatus.EXPIRED]: '已到期',
  }, String(status))
}

/**
 * deployModeLabel 返回租户部署形态文案。
 */
export function deployModeLabel(mode: DeployMode): string {
  return labelFromMap(mode, {
    [DeployMode.SAAS]: '平台 SaaS',
    [DeployMode.SCHOOL]: '学校私有化',
  }, '未知')
}

/**
 * authModeLabel 返回租户认证模式文案。
 */
export function authModeLabel(mode: AuthMode): string {
  return labelFromMap(mode, {
    [AuthMode.LOCAL]: '本地账号',
    [AuthMode.CAS]: 'CAS 单点登录',
    [AuthMode.LDAP]: 'LDAP 目录认证',
  }, '未知')
}

/**
 * tenantApplicationStatusLabel 返回入驻申请状态文案。
 */
export function tenantApplicationStatusLabel(status: ApplicationStatus): string {
  return labelFromMap(status, {
    [ApplicationStatus.PENDING]: '待审核',
    [ApplicationStatus.APPROVED]: '已通过',
    [ApplicationStatus.REJECTED]: '已驳回',
  }, '未知状态')
}

/**
 * alertStatusLabel 返回告警事件处理状态文案。
 */
export function alertStatusLabel(status: AlertStatus): string {
  return labelFromMap(status, {
    [AlertStatus.PENDING]: '待处理',
    [AlertStatus.HANDLED]: '已处理',
    [AlertStatus.IGNORED]: '已忽略',
  }, String(status))
}

/**
 * simComputeLabel 返回仿真包运行方式文案。
 */
export function simComputeLabel(compute: SimCompute): string {
  return labelFromMap(compute, {
    [SIM_COMPUTE.FRONTEND]: '前端仿真',
    [SIM_COMPUTE.BACKEND]: '后端仿真',
  }, '未识别的运行方式')
}

/**
 * simReviewResultLabel 返回仿真包审核结果文案。
 */
export function simReviewResultLabel(result: SimReviewResult): string {
  return labelFromMap(result, {
    [SIM_REVIEW_RESULT.PENDING]: '待审核',
    [SIM_REVIEW_RESULT.APPROVED]: '已通过',
    [SIM_REVIEW_RESULT.REJECTED]: '已退回',
  }, '未识别的审核结果')
}

/**
 * systemConfigLabel 返回平台配置键的管理端文案。
 */
export function systemConfigLabel(key: string): string {
  return labelFromMap(key, {
    maintenance_mode: '平台维护模式',
    oss: '对象存储配置',
    smtp: '邮件发送配置',
  }, key)
}

type SandboxQuotaField = keyof Omit<SandboxQuota, 'tenant_id' | 'active_sandbox_count'>

/**
 * sandboxQuotaFieldLabels 定义租户沙箱配额字段在表单中的展示文案。
 */
export const sandboxQuotaFieldLabels: Record<SandboxQuotaField, string> = {
  max_concurrent_sandbox: '最大并发沙箱数',
  max_cpu: '最大 CPU 毫核',
  max_memory_mb: '最大内存 MB',
  idle_timeout_min: '空闲回收分钟',
  max_lifetime_min: '最长运行分钟',
  max_keepalive_min: '最长保活分钟',
  max_snapshot_retention_min: '快照保留分钟',
}

/**
 * sandboxToolKindLabel 返回沙箱工具类型文案。
 */
export function sandboxToolKindLabel(kind: SandboxToolKind): string {
  return labelFromMap(kind, {
    [SandboxToolKind.BUILTIN]: '内置工具',
    [SandboxToolKind.TERMINAL]: '终端',
    [SandboxToolKind.WEB_EMBED]: '网页工具',
    [SandboxToolKind.COMMAND]: '受控命令',
  }, '未知')
}

/**
 * toolStatusLabel 返回沙箱工具可用状态文案。
 */
export function toolStatusLabel(status: ToolStatus): string {
  return labelFromMap(status, {
    [ToolStatus.AVAILABLE]: '可用',
    [ToolStatus.DISABLED]: '已停用',
  }, '未知')
}

/**
 * runtimeStatusLabel 返回链运行时状态文案。
 */
export function runtimeStatusLabel(status: RuntimeStatus): string {
  return labelFromMap(status, {
    [RuntimeStatus.AVAILABLE]: '可用',
    [RuntimeStatus.ONBOARDING]: '接入中',
    [RuntimeStatus.DISABLED]: '已停用',
  }, '未知')
}

/**
 * runtimeSelftestStatusLabel 返回链运行时自检状态文案。
 */
export function runtimeSelftestStatusLabel(status: RuntimeSelftestStatus): string {
  return labelFromMap(status, {
    [RuntimeSelftestStatus.PENDING]: '待自检',
    [RuntimeSelftestStatus.PASSED]: '已通过',
    [RuntimeSelftestStatus.FAILED]: '未通过',
  }, '未知')
}

/**
 * sandboxStatusLabel 返回沙箱实例状态文案。
 */
export function sandboxStatusLabel(status: SandboxStatus): string {
  return labelFromMap(status, {
    [SandboxStatus.CREATING]: '创建中',
    [SandboxStatus.RUNNING]: '运行中',
    [SandboxStatus.PAUSED]: '已暂停',
    [SandboxStatus.RECYCLING]: '回收中',
    [SandboxStatus.DESTROYED]: '已销毁',
    [SandboxStatus.FAILED]: '启动失败',
    [SandboxStatus.READY]: '就绪',
    [SandboxStatus.IDLE]: '空闲',
  }, '未知')
}

/**
 * imagePrepullStatusLabel 返回镜像预拉取状态文案。
 */
export function imagePrepullStatusLabel(status: ImagePrepullStatus): string {
  return labelFromMap(status, {
    [ImagePrepullStatus.PENDING]: '等待中',
    [ImagePrepullStatus.SUCCEEDED]: '已完成',
    [ImagePrepullStatus.FAILED]: '失败',
    [ImagePrepullStatus.RUNNING]: '进行中',
  }, '未知')
}

/**
 * backupTypeLabel 返回备份类型文案。
 */
export function backupTypeLabel(type: BackupType): string {
  return labelFromMap(type, {
    [BackupType.FULL]: '全量备份',
  }, '未知')
}

/**
 * backupStatusLabel 返回备份任务状态文案。
 */
export function backupStatusLabel(status: BackupStatus): string {
  return labelFromMap(status, {
    [BackupStatus.RUNNING]: '进行中',
    [BackupStatus.SUCCEEDED]: '已完成',
    [BackupStatus.FAILED]: '失败',
  }, '未知')
}

/**
 * judgerTypeLabel 返回判题器类型文案。
 */
export function judgerTypeLabel(type: JudgerType): string {
  return labelFromMap(type, {
    [JudgerType.TESTCASE]: '测试用例',
    [JudgerType.ONCHAIN_ASSERT]: '链上断言',
    [JudgerType.FLAG]: 'Flag 校验',
    [JudgerType.STATIC_SCAN]: '静态扫描',
    [JudgerType.SIM_CHECKPOINT]: '仿真检查点',
    [JudgerType.MANUAL]: '人工评分',
  }, '未知')
}

/**
 * judgerStatusLabel 返回判题器启用状态文案。
 */
export function judgerStatusLabel(status: JudgerStatus): string {
  return labelFromMap(status, {
    [JudgerStatus.AVAILABLE]: '可用',
    [JudgerStatus.DISABLED]: '已停用',
  }, '未知')
}
