// HTTP 契约常量：维护前端与后端 API 网关路径对齐的基础值。

/**
 * 后端统一 API 基础路径，与总 API 文档的 /api/v1 保持一致。
 */
export const API_BASE_PATH = '/api/v1'

/**
 * 后端统一信封的成功码（对齐 backend/pkg/apperr/codes.go 的 CodeOK）。
 */
export const API_SUCCESS_CODE = '0'

/**
 * 后端统一信封中前端需要据以决定交互的平台错误码（对齐 backend/pkg/apperr/codes.go）。
 * 业务面 HTTP 状态码恒为 200，登录态判定只认 code，不看状态码。
 */
export const API_ERROR_CODES = {
  UNAUTHORIZED: '11001',
} as const

/**
 * API 客户端在拿不到后端信封时使用的文案。
 * 网络层失败（断网、DNS、超时、连接被拒）根本没有响应体，后端 message 无从存在，
 * 这两条是该场景下唯一的用户向表达，不是对后端文案的兜底替换。
 * 页面仍负责标题、布局、trace_id 展示和交互动作。
 */
export const API_TRANSPORT_ERROR_MESSAGES = {
  /** 有响应但响应体不是平台信封（网关自身返回的错误页等） */
  MALFORMED_RESPONSE: '当前操作暂时没有完成，请稍后重试',
  /** 请求未拿到任何响应 */
  NETWORK: '网络连接暂时不可用，请检查网络后重试',
} as const
