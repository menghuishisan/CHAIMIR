// identity labels 文件维护账号、租户、认证、组织和申请状态文案。

import {
  AccountStatus,
  ApplicationStatus,
  AuthMode,
  DeployMode,
  ImportBatchStatus,
  SessionStatus,
  SsoMatchField,
  SsoType,
  TenantStatus,
  UserRole,
} from '@chaimir/api-client'
import { labelFromMap } from './map'

/** tenantApplicationSchoolTypeLabel 返回入驻机构类型文案。 */
export function tenantApplicationSchoolTypeLabel(type: number): string {
  return labelFromMap(type, { 1: '本科院校', 2: '高职高专', 3: '其他教育机构' }, '暂无法识别')
}

/** accountRoleLabel 按优先级返回账号角色文案。 */
export function accountRoleLabel(roles: UserRole[]): string {
  if (roles.includes(UserRole.PLATFORM_ADMIN)) return '平台管理员'
  if (roles.includes(UserRole.SCHOOL_ADMIN)) return '学校管理员'
  if (roles.includes(UserRole.TEACHER)) return '教师'
  if (roles.includes(UserRole.STUDENT)) return '学生'
  return '未分配'
}

/** baseIdentityLabel 返回账号基础身份文案。 */
export function baseIdentityLabel(identity: number): string {
  return labelFromMap(identity, { 1: '学生', 2: '教师' }, '未识别的基础身份')
}

/** accountStatusLabel 返回账号生命周期状态文案。 */
export function accountStatusLabel(status: AccountStatus): string {
  return labelFromMap(status, {
    [AccountStatus.PENDING]: '待激活',
    [AccountStatus.ACTIVE]: '正常',
    [AccountStatus.DISABLED]: '已停用',
    [AccountStatus.ARCHIVED]: '已归档',
    [AccountStatus.CANCELLED]: '已注销',
  }, '未知')
}

/** sessionStatusLabel 返回登录会话状态文案。 */
export function sessionStatusLabel(status: SessionStatus): string {
  return labelFromMap(status, { [SessionStatus.ACTIVE]: '有效', [SessionStatus.REVOKED]: '已失效' }, '未知')
}

/** importBatchStatusLabel 返回账号导入批次状态文案。 */
export function importBatchStatusLabel(status: ImportBatchStatus): string {
  return labelFromMap(status, {
    [ImportBatchStatus.PROCESSING]: '处理中',
    [ImportBatchStatus.COMPLETED]: '已完成',
    [ImportBatchStatus.FAILED]: '处理失败',
  }, '未知')
}

/** ssoTypeLabel 返回统一认证类型文案。 */
export function ssoTypeLabel(type: SsoType): string {
  return labelFromMap(type, { [SsoType.CAS]: 'CAS', [SsoType.LDAP]: 'LDAP' }, '未识别的认证类型')
}

/** ssoMatchFieldLabel 返回统一认证匹配字段文案。 */
export function ssoMatchFieldLabel(field: SsoMatchField): string {
  return labelFromMap(field, { [SsoMatchField.NO]: '学号工号', [SsoMatchField.PHONE]: '手机号' }, '未识别的匹配字段')
}

/** classStatusLabel 返回班级状态文案。 */
export function classStatusLabel(status: number): string {
  return labelFromMap(status, { 1: '正常', 2: '已归档' }, '未识别的班级状态')
}

/** tenantStatusLabel 返回租户运行状态文案。 */
export function tenantStatusLabel(status: TenantStatus): string {
  return labelFromMap(status, {
    [TenantStatus.ACTIVE]: '运营中',
    [TenantStatus.DISABLED]: '已停用',
    [TenantStatus.EXPIRED]: '已到期',
  }, String(status))
}

/** deployModeLabel 返回租户部署形态文案。 */
export function deployModeLabel(mode: DeployMode): string {
  return labelFromMap(mode, { [DeployMode.SAAS]: '平台 SaaS', [DeployMode.SCHOOL]: '学校私有化' }, '未知')
}

/** authModeLabel 返回租户认证模式文案。 */
export function authModeLabel(mode: AuthMode): string {
  return labelFromMap(mode, {
    [AuthMode.LOCAL]: '本地账号',
    [AuthMode.CAS]: 'CAS 单点登录',
    [AuthMode.LDAP]: 'LDAP 目录认证',
  }, '未知')
}

/** tenantApplicationStatusLabel 返回入驻申请状态文案。 */
export function tenantApplicationStatusLabel(status: ApplicationStatus): string {
  return labelFromMap(status, {
    [ApplicationStatus.PENDING]: '待审核',
    [ApplicationStatus.APPROVED]: '已通过',
    [ApplicationStatus.REJECTED]: '已驳回',
  }, '未知状态')
}
