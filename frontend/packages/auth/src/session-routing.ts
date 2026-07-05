// 会话路由：统一认证成功后的登录态保存和单入口角色路径跳转规则。

import type { ChaimirApi, LoginResponse } from '@chaimir/api-client'
import { readFrontendConfig, saveSession, saveStoredUser } from '@chaimir/shared'
import type { FormValues, LoginMode } from './types'
import { optionalNumberOf, valueOf } from './form-state'

/**
 * loginByMode 将三种学校用户登录方式映射到后端已有接口。
 */
export async function loginByMode(api: ChaimirApi, mode: LoginMode, values: FormValues): Promise<LoginResponse> {
  if (mode === 'no') {
    return api.identity.loginNo({
      tenant_code: valueOf(values, 'tenant_code'),
      no: valueOf(values, 'no'),
      password: valueOf(values, 'password'),
    })
  }
  if (mode === 'sms') {
    return api.identity.loginSMS({
      phone: valueOf(values, 'phone'),
      code: valueOf(values, 'code'),
      tenant_id: optionalNumberOf(values, 'tenant_id'),
    })
  }
  return api.identity.loginPhone({
    phone: valueOf(values, 'phone'),
    password: valueOf(values, 'password'),
    tenant_id: optionalNumberOf(values, 'tenant_id'),
  })
}

/**
 * handleLoginResponse 保存登录态，并按后端角色直接进入对应端第一个功能页。
 */
export function handleLoginResponse(response: LoginResponse, config: ReturnType<typeof readFrontendConfig>): void {
  if (response.need_select_tenant) {
    return
  }
  saveSession(response.access_token, response.refresh_token)
  if (response.account) {
    saveStoredUser(response.account)
  }
  if (response.must_change_pwd) {
    window.location.hash = '#change-pwd'
    return
  }
  const target = resolveRoleEntryPath(response, config)
  window.location.assign(target)
}

/**
 * resolveRoleEntryPath 根据 M1 角色编码选择单入口下的角色路径。
 */
function resolveRoleEntryPath(response: LoginResponse, config: ReturnType<typeof readFrontendConfig>): string {
  const roles = response.account?.roles ?? []
  const identity = response.account?.base_identity
  if (roles.includes(1) || identity === 1) return config.roleEntryPaths.platformAdmin
  if (roles.includes(2) || identity === 2) return config.roleEntryPaths.schoolAdmin
  if (roles.includes(3) || identity === 3) return config.roleEntryPaths.teacher
  return config.roleEntryPaths.student
}
