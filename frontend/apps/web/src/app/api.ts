// 应用级 API 入口：集中创建后端 SDK 实例，统一鉴权、trace id 和未登录处理。

import { createApi } from '@chaimir/api-client'
import { clearLoginTokens, getStoredAccessToken, getStoredRefreshToken, persistRefreshedTokens } from '../utils/authSession'
import { appConfig } from './config'

const TRACE_ID_KEY = 'chaimir.trace_id'

/** getStoredTraceId 读取最近一次后端请求的报障追踪编号。 */
function getStoredTraceId(): string | null {
  if (typeof window === 'undefined') {
    return null
  }
  return window.localStorage.getItem(TRACE_ID_KEY) || window.sessionStorage.getItem(TRACE_ID_KEY)
}

/**
 * api 是 apps/web 调用后端的唯一 SDK 实例。
 */
export const api = createApi({
  baseURL: appConfig.apiBaseURL,
  wsBaseURL: appConfig.wsBaseURL,
  timeout: appConfig.apiTimeoutMs,
  getToken: getStoredAccessToken,
  getRefreshToken: getStoredRefreshToken,
  onTokensRefreshed: persistRefreshedTokens,
  getTraceId: getStoredTraceId,
  onUnauthorized: () => {
    clearLoginTokens()
    window.dispatchEvent(new CustomEvent('chaimir:unauthorized'))
    if (!window.location.pathname.startsWith('/auth/')) {
      window.location.assign('/auth/login')
    }
  },
})
