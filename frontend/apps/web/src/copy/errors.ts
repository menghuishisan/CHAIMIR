// 错误文案：集中维护应用级错误标题、兜底提示和操作标签。

import { API_ERROR_MESSAGES } from '@chaimir/api-client'

export const ERROR_TITLES = {
  ROUTE_LOAD: '暂时无法加载',
  APP_CRASH: '页面暂时不可用',
} as const

export const ERROR_MESSAGES = {
  HTTP_FALLBACK: API_ERROR_MESSAGES.HTTP_FALLBACK,
  NETWORK_FALLBACK: API_ERROR_MESSAGES.NETWORK_FALLBACK,
  BUSINESS_FALLBACK: API_ERROR_MESSAGES.BUSINESS_FALLBACK,
  APP_CRASH: '页面遇到异常，请重新加载后再试',
  BOOTSTRAP_CRASH: '页面加载失败，请刷新后重试',
} as const

export const REALTIME_ERROR_MESSAGES = {
  INVALID_URL: '实时连接地址无效',
  NOT_CONNECTED: '实时连接尚未建立',
} as const

export const ERROR_ACTION_LABELS = {
  RELOAD: '重新加载',
} as const

export type ErrorTitleKey = keyof typeof ERROR_TITLES
