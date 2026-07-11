// userFacingError.ts 把 API 错误转换为可直接展示给终端用户的自然语言提示。

import type { ApiError } from '@chaimir/api-client'

/**
 * userFacingErrorMessage 返回用户向错误，并在后端提供 trace id 时附上报障编号。
 */
export function userFacingErrorMessage(error: unknown, fallback: string): string {
  const apiError = error as Partial<ApiError> | null
  const isSdkError = Boolean(apiError && typeof apiError === 'object' && ('code' in apiError || 'status' in apiError || 'traceId' in apiError))
  const message = isSdkError ? apiError?.message?.trim() || fallback : fallback
  const traceId = apiError?.traceId?.trim()
  return traceId ? `${message} 如需帮助，请提供编号 ${traceId}。` : message
}
