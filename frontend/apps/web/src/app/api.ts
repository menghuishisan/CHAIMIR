import { createApi } from '@chaimir/api-client'
import { expireLoginSession, getStoredAccessToken, persistRefreshedTokens, rememberCurrentEntryPath } from '../utils/authSession'
import { loginPathForPath } from '../utils/roleRouting'
import { appConfig } from './config'

// 报障编号由服务端在响应中签发，应用不会自行生成。
export const api = createApi({
  baseURL: appConfig.apiBaseURL,
  wsBaseURL: appConfig.wsBaseURL,
  timeout: appConfig.apiTimeoutMs,
  getToken: getStoredAccessToken,
  hasRefreshSession: () => true,
  onTokensRefreshed: persistRefreshedTokens,
  onUnauthorized: () => {
    expireLoginSession()
    // 认证页自行提示会话失效，保留当前表单。
    if (window.location.pathname.startsWith('/auth/')) return
    rememberCurrentEntryPath()
    window.location.assign(loginPathForPath(window.location.pathname))
  },
})
