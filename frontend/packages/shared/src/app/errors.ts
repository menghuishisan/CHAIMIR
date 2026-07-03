// 前端错误处理：把后端错误信封转换为用户可理解的提示，不暴露内部细节。

import type { ApiError } from '@chaimir/api-client'

export interface UserFacingError {
  title: string
  message: string
  traceId?: string
}

export interface ToUserFacingErrorOptions {
  /** 是否允许展示本地显式抛出的用户向错误文案。渲染异常边界应关闭。 */
  allowPlainMessage?: boolean
}

/**
 * toUserFacingError 将未知错误收敛为页面可展示的用户向文案。
 */
export function toUserFacingError(error: unknown, options: ToUserFacingErrorOptions = {}): UserFacingError {
  const apiError = error as Partial<ApiError>
  const canUseMessage = options.allowPlainMessage !== false || Boolean(apiError.code || apiError.traceId || apiError.status)
  const message = canUseMessage && typeof apiError.message === 'string' && apiError.message.trim()
    ? apiError.message
    : '当前操作暂时没有完成，请稍后重试'

  return {
    title: '暂时无法加载',
    message,
    traceId: typeof apiError.traceId === 'string' ? apiError.traceId : undefined,
  }
}
