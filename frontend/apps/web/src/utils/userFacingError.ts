import { ApiError } from '@chaimir/api-client'

export const RESOURCE_LOAD_FAILED_MESSAGE = '暂时无法获取数据，请稍后重试'

const SENTENCE_END = /[。！？!?.]$/

export function traceHintText(traceId: string): string {
  return `如需帮助，请提供编号 ${traceId}。`
}

/** 仅 ApiError 的原始文案可以展示给用户。 */
export function userFacingErrorMessage(error: unknown, defaultMessage: string): string {
  if (!(error instanceof ApiError)) return defaultMessage
  const traceId = error.traceId
  if (!traceId) return error.message
  const message = SENTENCE_END.test(error.message) ? error.message : `${error.message}。`
  return `${message} ${traceHintText(traceId)}`
}

export function traceIdOf(error: unknown): string | undefined {
  return error instanceof ApiError ? error.traceId : undefined
}

/**
 * errorDiagnostics 生成可安全写入浏览器控制台的错误摘要。
 * 不保留 Axios 配置、响应体或异常 message,避免把令牌、内部地址和服务端细节带入日志。
 */
export function errorDiagnostics(error: unknown): Record<string, string | undefined> {
  if (error instanceof ApiError) {
    return { kind: 'api', code: error.code, trace_id: error.traceId }
  }
  return { kind: error instanceof Error ? error.name : typeof error }
}
