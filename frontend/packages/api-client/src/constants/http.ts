// HTTP 契约常量：维护前端与后端 API 网关路径对齐的基础值。

/**
 * 后端统一 API 基础路径，与总 API 文档的 /api/v1 保持一致。
 */
export const API_BASE_PATH = '/api/v1'

/**
 * API 客户端在后端无 message 或网络无响应时使用的兜底文案。
 * 页面仍负责标题、布局、trace_id 展示和交互动作。
 */
export const API_ERROR_MESSAGES = {
  HTTP_FALLBACK: '当前操作暂时没有完成，请稍后重试',
  NETWORK_FALLBACK: '网络连接暂时不可用，请检查网络后重试',
  BUSINESS_FALLBACK: '操作失败',
} as const
