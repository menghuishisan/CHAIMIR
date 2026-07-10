// 应用级 API 入口：集中创建后端 SDK 实例，统一鉴权、trace id 和未登录处理。

import { createApi } from '@chaimir/api-client'
import { ACCESS_TOKEN_KEY } from '../utils/authSession'
import { appConfig } from './config'

const TRACE_ID_KEY = 'chaimir.trace_id'

/**
 * getBrowserItem 从浏览器存储读取调用后端所需的短字符串。
 */
function getBrowserItem(key: string): string | null {
  if (typeof window === 'undefined') {
    return null
  }
  return window.localStorage.getItem(key) || window.sessionStorage.getItem(key)
}

/**
 * api 是 apps/web 调用后端的唯一 SDK 实例。
 */
export const api = createApi({
  baseURL: appConfig.apiBaseURL,
  wsBaseURL: appConfig.wsBaseURL,
  timeout: appConfig.apiTimeoutMs,
  getToken: () => getBrowserItem(ACCESS_TOKEN_KEY),
  getTraceId: () => getBrowserItem(TRACE_ID_KEY),
  onUnauthorized: () => {
    window.dispatchEvent(new CustomEvent('chaimir:unauthorized'))
  },
})
