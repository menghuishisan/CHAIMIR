// 会话路由：统一认证成功后的登录态保存和四端跳转规则。

import type { ChaimirApi, LoginResponse } from '@chaimir/api-client'
import { readFrontendConfig, saveSession } from '@chaimir/shared'
import type { FormValues, LoginMode } from './types'
import { numberOf, valueOf } from './form-state'

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
      tenant_id: numberOf(values, 'tenant_id'),
    })
  }
  return api.identity.loginPhone({
    phone: valueOf(values, 'phone'),
    password: valueOf(values, 'password'),
    tenant_id: numberOf(values, 'tenant_id'),
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
  if (response.must_change_pwd) {
    window.location.hash = '#change-pwd'
    return
  }
  const target = resolveRoleUrl(response, config)
  window.location.assign(target)
}

/**
 * resolveRoleUrl 根据 M1 角色编码选择四端入口，平台管理员优先进入平台管理端。
 */
function resolveRoleUrl(response: LoginResponse, config: ReturnType<typeof readFrontendConfig>): string {
  const roles = response.account?.roles ?? []
  const identity = response.account?.base_identity
  if (roles.includes(1) || identity === 1) return config.roleAppUrls.platformAdmin
  if (roles.includes(2) || identity === 2) return config.roleAppUrls.schoolAdmin
  if (roles.includes(3) || identity === 3) return config.roleAppUrls.teacher
  return config.roleAppUrls.student
}
