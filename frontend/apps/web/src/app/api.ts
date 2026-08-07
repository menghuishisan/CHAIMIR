// 应用级 API 入口：集中创建后端 SDK 实例，统一鉴权与未登录处理。
// trace_id 由后端在错误信封中签发，随 ApiError 直达展示处（前端不生成、不转存、不回传），
// 因此这里不注入任何 X-Trace-Id 请求头 —— 由前端回灌的编号在运维侧查不到，等于假报障线索。

import { createApi } from '@chaimir/api-client'
import { clearLoginTokens, getStoredAccessToken, persistRefreshedTokens } from '../utils/authSession'
import { loginPathForPath } from '../utils/roleRouting'
import { appConfig } from './config'

/**
 * api 是 apps/web 调用后端的唯一 SDK 实例。
 */
export const api = createApi({
  baseURL: appConfig.apiBaseURL,
  wsBaseURL: appConfig.wsBaseURL,
  timeout: appConfig.apiTimeoutMs,
  getToken: getStoredAccessToken,
  hasRefreshSession: () => true,
  onTokensRefreshed: persistRefreshedTokens,
  onUnauthorized: () => {
    clearLoginTokens()
    // 认证域内的 401 由页面自行呈现（改密页会话过期等），不打断当前表单
    if (window.location.pathname.startsWith('/auth/')) return
    // 按当前所在角色区回对应登录入口：平台管理员不落到学校登录页
    window.location.assign(loginPathForPath(window.location.pathname))
  },
})
