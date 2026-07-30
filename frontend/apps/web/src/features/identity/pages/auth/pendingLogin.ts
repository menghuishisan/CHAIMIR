// pendingLogin 在「手机号登录 → 多学校选择」两步之间以内存暂存登录凭证。
// 安全约束:凭证(含密码/验证码)绝不进入路由 state、localStorage 或任何持久化存储
// ——路由 state 会被浏览器写入历史记录存储。刷新即失效,由选择页引导重新登录。

import type { TenantOption } from '@chaimir/api-client'

export interface PendingTenantLogin {
  tenants: TenantOption[]
  remember: boolean
  returnPath?: string
  method:
    | { type: 'phone'; phone: string; password: string }
    | { type: 'sms'; phone: string; code: string }
}

let pending: PendingTenantLogin | null = null

/** 登录页在收到 need_select_tenant 后暂存凭证 */
export function setPendingTenantLogin(value: PendingTenantLogin): void {
  pending = value
}

/** 多学校选择页读取暂存凭证(不清除:选择失败可重试) */
export function getPendingTenantLogin(): PendingTenantLogin | null {
  return pending
}

/** 登录完成或返回登录页时清除,凭证不在内存中久留 */
export function clearPendingTenantLogin(): void {
  pending = null
}
