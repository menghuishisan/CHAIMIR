// identity labels 文件维护 identity 模块枚举值的用户向文案。

import type { StatusTone } from '@chaimir/ui'
import {
  AccountStatus,
  ApplicationStatus,
  AuthMode,
  BaseIdentity,
  DeployMode,
  SessionStatus,
  TenantStatus,
  UserRole,
} from '@chaimir/api-client'

/** 机构类型取值与后端 school_type 一致(1 本科 / 2 高职高专 / 3 其他),取值与文案在此单一登记。 */
const TENANT_APPLICATION_SCHOOL_TYPE_LABELS = {
  1: '本科院校',
  2: '高职高专',
  3: '其他教育机构',
} as const

export type TenantApplicationSchoolType = keyof typeof TENANT_APPLICATION_SCHOOL_TYPE_LABELS

/** TENANT_APPLICATION_SCHOOL_TYPES 供表单按登记顺序渲染选项,避免页面再抄一份取值清单。 */
export const TENANT_APPLICATION_SCHOOL_TYPES = [1, 2, 3] as const satisfies readonly TenantApplicationSchoolType[]

/** tenantApplicationSchoolTypeLabel 返回入驻机构类型文案。 */
export function tenantApplicationSchoolTypeLabel(type: TenantApplicationSchoolType): string {
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
function isKnownSchoolType(type: number): type is TenantApplicationSchoolType {
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

const ACCOUNT_STATUS_TONES: Record<AccountStatus, StatusTone> = {
  [AccountStatus.PENDING]: 'warning',
  [AccountStatus.ACTIVE]: 'success',
  [AccountStatus.DISABLED]: 'danger',
  [AccountStatus.ARCHIVED]: 'neutral',
  [AccountStatus.CANCELLED]: 'neutral',
}

/** accountStatusLabel 返回账号状态文案。 */
export function accountStatusLabel(status: AccountStatus): string {
  return ACCOUNT_STATUS_LABELS[status]
}

/** accountStatusTone 返回账号状态语义色。 */
export function accountStatusTone(status: AccountStatus): StatusTone {
  return ACCOUNT_STATUS_TONES[status]
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

const SESSION_STATUS_TONES: Record<SessionStatus, StatusTone> = {
  [SessionStatus.ACTIVE]: 'success',
  [SessionStatus.REVOKED]: 'neutral',
}

/** sessionStatusLabel 返回登录会话状态文案。 */
export function sessionStatusLabel(status: SessionStatus): string {
  return SESSION_STATUS_LABELS[status]
}

/** sessionStatusTone 返回登录会话状态语义色。 */
export function sessionStatusTone(status: SessionStatus): StatusTone {
  return SESSION_STATUS_TONES[status]
}

const TENANT_STATUS_LABELS: Record<TenantStatus, string> = {
  [TenantStatus.ACTIVE]: '正常使用',
  [TenantStatus.DISABLED]: '已停用',
  [TenantStatus.EXPIRED]: '已到期',
}

const TENANT_STATUS_TONES: Record<TenantStatus, StatusTone> = {
  [TenantStatus.ACTIVE]: 'success',
  [TenantStatus.DISABLED]: 'danger',
  [TenantStatus.EXPIRED]: 'warning',
}

/** tenantStatusLabel 返回学校状态文案。 */
export function tenantStatusLabel(status: TenantStatus): string {
  return TENANT_STATUS_LABELS[status]
}

/** tenantStatusTone 返回学校状态语义色。 */
export function tenantStatusTone(status: TenantStatus): StatusTone {
  return TENANT_STATUS_TONES[status]
}

/** TENANT_STATUSES 供状态调整表单按登记顺序渲染选项(后端只接受这三种)。 */
export const TENANT_STATUSES = [
  TenantStatus.ACTIVE,
  TenantStatus.DISABLED,
  TenantStatus.EXPIRED,
] as const

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

const APPLICATION_STATUS_TONES: Record<ApplicationStatus, StatusTone> = {
  [ApplicationStatus.PENDING]: 'warning',
  [ApplicationStatus.APPROVED]: 'success',
  [ApplicationStatus.REJECTED]: 'neutral',
}

/** applicationStatusLabel 返回入驻申请状态文案。 */
export function applicationStatusLabel(status: ApplicationStatus): string {
  return APPLICATION_STATUS_LABELS[status]
}

/** applicationStatusTone 返回入驻申请状态语义色。 */
export function applicationStatusTone(status: ApplicationStatus): StatusTone {
  return APPLICATION_STATUS_TONES[status]
}
