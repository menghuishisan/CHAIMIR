// identity labels 文件维护 identity 模块枚举值的用户向文案。

import {
  AccountStatus,
  ApplicationStatus,
  AuthMode,
  BaseIdentity,
  ClassStatus,
  DeployMode,
  SessionStatus,
  TenantStatus,
  UserRole,
  TENANT_MODULE,
  type TenantModule,
} from '@chaimir/api-client'

/** 班级状态用户向文案与语义色,供教师只读和校管维护页面共用。 */
export const CLASS_STATUS_LABELS: Record<ClassStatus, string> = {
  [ClassStatus.ACTIVE]: '在读',
  [ClassStatus.ARCHIVED]: '已归档',
}

/** 租户模块用户向文案与说明,供校管配置和平台只读详情共同使用。 */
export const TENANT_MODULE_LABELS: Record<TenantModule, string> = {
  [TENANT_MODULE.TEACHING]: '教学',
  [TENANT_MODULE.EXPERIMENT]: '实验',
  [TENANT_MODULE.CONTEST]: '竞赛',
  [TENANT_MODULE.SIM]: '仿真',
  [TENANT_MODULE.GRADE]: '成绩',
}

/** 租户模块说明文案,与模块名称共同供配置页面使用。 */
export const TENANT_MODULE_DESCRIPTIONS: Record<TenantModule, string> = {
  [TENANT_MODULE.TEACHING]: '课程、章节课时、作业与批改',
  [TENANT_MODULE.EXPERIMENT]: '实验编排与沙箱实验环境',
  [TENANT_MODULE.CONTEST]: '赛事组织、解题赛与对抗赛',
  [TENANT_MODULE.SIM]: '区块链协议与系统推演',
  [TENANT_MODULE.GRADE]: '成绩汇总与学业分析',
}

/** 机构类型取值与后端 school_type 一致(1 本科 / 2 高职高专 / 3 其他),取值与文案在此单一登记。 */
const TENANT_APPLICATION_SCHOOL_TYPE_LABELS = {
  1: '本科院校',
  2: '高职高专',
  3: '其他教育机构',
} as const

/** tenantApplicationSchoolTypeLabel 返回入驻机构类型文案。 */
export function tenantApplicationSchoolTypeLabel(
  type: keyof typeof TENANT_APPLICATION_SCHOOL_TYPE_LABELS,
): string {
  return TENANT_APPLICATION_SCHOOL_TYPE_LABELS[type]
}

/**
 * schoolTypeLabel 返回学校记录上的机构类型文案。
 * 申请与租户上的 school_type/type 是后端开放数字(申请表单只产出 1-3,历史数据可能有别的值),
 * 故读取侧宽松处理:未登记的值给通用名而不是裸数字。
 */
export function schoolTypeLabel(type: number): string {
  return isKnownSchoolType(type) ? TENANT_APPLICATION_SCHOOL_TYPE_LABELS[type] : '其他教育机构'
}

/** isKnownSchoolType 判断数字是否落在已登记的机构类型内。 */
function isKnownSchoolType(
  type: number,
): type is keyof typeof TENANT_APPLICATION_SCHOOL_TYPE_LABELS {
  return type in TENANT_APPLICATION_SCHOOL_TYPE_LABELS
}

/** 角色名称:全站统一口径,顶栏与个人中心共用。 */
const USER_ROLE_LABELS: Record<UserRole, string> = {
  [UserRole.PLATFORM_ADMIN]: '平台管理员',
  [UserRole.SCHOOL_ADMIN]: '学校管理员',
  [UserRole.TEACHER]: '教师',
  [UserRole.STUDENT]: '学生',
}

/** userRoleLabel 返回角色文案。 */
export function userRoleLabel(role: UserRole): string {
  return USER_ROLE_LABELS[role]
}

/** userRolesLabel 把多角色拼成一句用户向说明。 */
export function userRolesLabel(roles: UserRole[]): string {
  return roles.map(userRoleLabel).filter(Boolean).join(' · ')
}

const ACCOUNT_STATUS_LABELS: Record<AccountStatus, string> = {
  [AccountStatus.PENDING]: '待激活',
  [AccountStatus.ACTIVE]: '正常',
  [AccountStatus.DISABLED]: '已停用',
  [AccountStatus.ARCHIVED]: '已归档',
  [AccountStatus.CANCELLED]: '已注销',
}

/** accountStatusLabel 返回账号状态文案。 */
export function accountStatusLabel(status: AccountStatus): string {
  return ACCOUNT_STATUS_LABELS[status]
}

const BASE_IDENTITY_LABELS: Record<BaseIdentity, string> = {
  [BaseIdentity.STUDENT]: '学生',
  [BaseIdentity.TEACHER]: '教师',
}

/** baseIdentityLabel 返回基础身份文案(学号/工号的归属)。 */
export function baseIdentityLabel(identity: BaseIdentity): string {
  return BASE_IDENTITY_LABELS[identity]
}

const SESSION_STATUS_LABELS: Record<SessionStatus, string> = {
  [SessionStatus.ACTIVE]: '生效中',
  [SessionStatus.REVOKED]: '已退出',
}

/** sessionStatusLabel 返回登录会话状态文案。 */
export function sessionStatusLabel(status: SessionStatus): string {
  return SESSION_STATUS_LABELS[status]
}

const TENANT_STATUS_LABELS: Record<TenantStatus, string> = {
  [TenantStatus.ACTIVE]: '正常使用',
  [TenantStatus.DISABLED]: '已停用',
  [TenantStatus.EXPIRED]: '已到期',
}

/** tenantStatusLabel 返回学校状态文案。 */
export function tenantStatusLabel(status: TenantStatus): string {
  return TENANT_STATUS_LABELS[status]
}

const DEPLOY_MODE_LABELS: Record<DeployMode, string> = {
  [DeployMode.SAAS]: '平台托管',
  [DeployMode.SCHOOL]: '学校自建',
}

/** deployModeLabel 返回部署形态文案。 */
export function deployModeLabel(mode: DeployMode): string {
  return DEPLOY_MODE_LABELS[mode]
}

const AUTH_MODE_LABELS: Record<AuthMode, string> = {
  [AuthMode.LOCAL]: '平台账号密码',
  [AuthMode.CAS]: '学校统一认证(CAS)',
  [AuthMode.LDAP]: '学校目录服务(LDAP)',
}

/** authModeLabel 返回登录方式文案。 */
export function authModeLabel(mode: AuthMode): string {
  return AUTH_MODE_LABELS[mode]
}

const APPLICATION_STATUS_LABELS: Record<ApplicationStatus, string> = {
  [ApplicationStatus.PENDING]: '待审核',
  [ApplicationStatus.APPROVED]: '已开通',
  [ApplicationStatus.REJECTED]: '已驳回',
}

/** applicationStatusLabel 返回入驻申请状态文案。 */
export function applicationStatusLabel(status: ApplicationStatus): string {
  return APPLICATION_STATUS_LABELS[status]
}
