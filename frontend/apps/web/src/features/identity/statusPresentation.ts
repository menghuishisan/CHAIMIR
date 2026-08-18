// identity statusPresentation 文件维护身份与租户状态的设计系统语义色。

import type { StatusTone } from '@chaimir/ui'
import { AccountStatus, ApplicationStatus, ClassStatus, SessionStatus, TenantStatus } from '@chaimir/api-client'

/** CLASS_STATUS_TONES 供组织页面统一呈现班级状态。 */
export const CLASS_STATUS_TONES: Record<ClassStatus, StatusTone> = {
  [ClassStatus.ACTIVE]: 'success',
  [ClassStatus.ARCHIVED]: 'neutral',
}

const ACCOUNT_STATUS_TONES: Record<AccountStatus, StatusTone> = {
  [AccountStatus.PENDING]: 'warning',
  [AccountStatus.ACTIVE]: 'success',
  [AccountStatus.DISABLED]: 'danger',
  [AccountStatus.ARCHIVED]: 'neutral',
  [AccountStatus.CANCELLED]: 'neutral',
}

/** accountStatusTone 返回账号状态语义色。 */
export function accountStatusTone(status: AccountStatus): StatusTone {
  return ACCOUNT_STATUS_TONES[status]
}

const SESSION_STATUS_TONES: Record<SessionStatus, StatusTone> = {
  [SessionStatus.ACTIVE]: 'success',
  [SessionStatus.REVOKED]: 'neutral',
}

/** sessionStatusTone 返回登录会话状态语义色。 */
export function sessionStatusTone(status: SessionStatus): StatusTone {
  return SESSION_STATUS_TONES[status]
}

const TENANT_STATUS_TONES: Record<TenantStatus, StatusTone> = {
  [TenantStatus.ACTIVE]: 'success',
  [TenantStatus.DISABLED]: 'danger',
  [TenantStatus.EXPIRED]: 'warning',
}

/** tenantStatusTone 返回学校状态语义色。 */
export function tenantStatusTone(status: TenantStatus): StatusTone {
  return TENANT_STATUS_TONES[status]
}

const APPLICATION_STATUS_TONES: Record<ApplicationStatus, StatusTone> = {
  [ApplicationStatus.PENDING]: 'warning',
  [ApplicationStatus.APPROVED]: 'success',
  [ApplicationStatus.REJECTED]: 'neutral',
}

/** applicationStatusTone 返回入驻申请状态语义色。 */
export function applicationStatusTone(status: ApplicationStatus): StatusTone {
  return APPLICATION_STATUS_TONES[status]
}
