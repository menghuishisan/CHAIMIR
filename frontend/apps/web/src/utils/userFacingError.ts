// userFacingError.ts 把捕获到的异常收敛为可直接展示给终端用户的自然语言提示。
// 边界规则(规范 §8 错误暴露分层):只有 API 客户端产出的 ApiError 携带用户向文案,
// 其余异常(组件运行时错误、类型错误等)的 message 是开发向文本,一律换成调用方给的用户向文案。

import { ApiError } from '@chaimir/api-client'

/**
 * RESOURCE_LOAD_FAILED_MESSAGE 是读取失败时的用户向文案。
 * 用于「异常不是后端签发的业务错误」这一种情况(组件内运行时异常),
 * 此时不存在后端 message 与 trace_id,只能由前端表达。
 */
export const RESOURCE_LOAD_FAILED_MESSAGE = '暂时无法获取数据，请稍后重试'

/**
 * userFacingErrorMessage 返回用户向错误，并在后端签发 trace id 时附上报障编号。
 */
export function userFacingErrorMessage(error: unknown, defaultMessage: string): string {
  if (!(error instanceof ApiError)) return defaultMessage
  const traceId = error.traceId
  return traceId ? `${error.message} 如需帮助，请提供编号 ${traceId}。` : error.message
}

/**
 * traceIdOf 取出后端签发的报障编号；只认后端 trace_id，前端不生成替代编号
 * （前端随机编号在运维侧查不到，展示它等于把用户挡在报障流程之外）。
 */
export function traceIdOf(error: unknown): string | undefined {
  return error instanceof ApiError ? error.traceId : undefined
}
