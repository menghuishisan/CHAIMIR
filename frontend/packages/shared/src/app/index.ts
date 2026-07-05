// 四端应用共享入口：导出应用壳、配置、类型、路由、存储和错误工具。

export { ChaimirApp } from './ChaimirApp'
export { readFrontendConfig } from './config'
export { toUserFacingError } from './errors'
export { clearSession, getAccessToken, getRefreshToken, getStoredUser, getTraceId, saveSession, saveStoredUser } from './storage'
export { parseHashRoute, routeHref } from './router'
export type * from './types'
