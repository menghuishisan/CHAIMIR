// API 客户端核心：封装后端统一信封、鉴权头、trace_id 透传和用户向错误。
// 后端业务面 HTTP 状态码恒为 200(docs/总-API接口总览.md §2 统一响应体),
// 一切业务结果——含登录失效——都在信封的 code 里,因此本文件只按 code 判定,
// axios 的 error 分支只剩「拿不到响应体」的传输层失败。

import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'
import { API_BASE_PATH, API_ERROR_CODES, API_SUCCESS_CODE, API_TRANSPORT_ERROR_MESSAGES } from './constants'

export interface ApiConfig {
  baseURL: string
  wsBaseURL?: string
  /** 请求超时毫秒数:由装配层从部署环境变量读入,SDK 不设默认阈值 */
  timeout: number
  getToken?: () => string | null
  getRefreshToken?: () => string | null
  onTokensRefreshed?: (tokens: TokenRefreshResponse) => void
  onUnauthorized?: () => void
}

export interface ApiResponse<T = unknown> {
  /** 后端信封业务码，成功恒为 '0'（对齐 apperr.CodeOK） */
  code: string
  /** 用户向文案，后端每个错误码都带，前端不改写 */
  message: string
  data?: T
  trace_id?: string
}

/**
 * ApiError 是 API 客户端唯一对外抛出的错误类型。
 * 用具体类而不是结构化对象，是为了让调用方能用 instanceof 精确区分
 * 「本客户端产出的用户向错误」与「组件内的运行时异常」——
 * 后者的 message 是开发向文本，不允许展示给用户（§8 错误暴露分层）。
 */
export class ApiError extends Error {
  /** 后端业务码，页面据此决定交互（跳转/重试/提示）；传输层失败没有业务码 */
  readonly code?: string
  /** 后端签发的报障编号，前端不生成替代值 */
  readonly traceId?: string

  constructor(message: string, code?: string, traceId?: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.traceId = traceId
  }
}

export interface TokenRefreshResponse {
  access_token: string
  refresh_token: string
  must_change_pwd?: boolean
}

interface RetriableRequestConfig extends InternalAxiosRequestConfig {
  chaimirRetried?: boolean
}

export class ApiClient {
  private client: AxiosInstance
  private config: ApiConfig
  private refreshPromise: Promise<void> | null = null

  /**
   * constructor 创建绑定后端统一 API 根路径的 Axios 客户端。
   */
  constructor(config: ApiConfig) {
    this.config = {
      ...config,
      baseURL: normalizeBaseURL(config.baseURL),
      wsBaseURL: config.wsBaseURL ? normalizeBaseURL(config.wsBaseURL) : undefined,
    }
    this.client = axios.create({
      baseURL: this.config.baseURL,
      timeout: this.config.timeout,
      headers: {
        'Content-Type': 'application/json',
      },
    })

    this.setupInterceptors()
  }

  /**
   * setupInterceptors 注入认证、trace_id 和后端统一信封解析逻辑。
   */
  private setupInterceptors() {
    // 请求拦截：注入 Token
    this.client.interceptors.request.use(
      (config: InternalAxiosRequestConfig) => {
        const token = this.config.getToken?.()
        if (token && config.headers) {
          config.headers.Authorization = `Bearer ${token}`
        }
        return config
      },
      (error) => {
        return Promise.reject(transformTransportError(error))
      }
    )

    // 响应拦截：业务结果全在信封的 code 里，按 code 判定
    this.client.interceptors.response.use(
      async (response) => {
        const requestConfig = response.config as RetriableRequestConfig

        // 统一文件服务下载流不使用 JSON 信封(API 总览 §统一文件服务)，按原样透出。
        // 同时带出 Content-Disposition：保存文件名的唯一来源是后端响应头，
        // 拦截器是能拿到响应头的唯一位置，故在此打包给 getAttachment。
        if (requestConfig.responseType === 'blob') {
          return { blob: response.data, disposition: response.headers['content-disposition'] } as never
        }

        // 边界校验：不是平台信封的 200 响应来自网关而非后端，按传输层失败处理
        if (!isEnvelope(response.data)) {
          return Promise.reject(new ApiError(API_TRANSPORT_ERROR_MESSAGES.MALFORMED_RESPONSE))
        }

        const apiResponse = response.data
        if (apiResponse.code === API_SUCCESS_CODE) {
          return (apiResponse.data !== undefined ? apiResponse.data : apiResponse) as never
        }

        // 登录态失效：用 Refresh Token 轮转一次并重放原请求
        if (this.shouldRefresh(apiResponse, requestConfig)) {
          requestConfig.chaimirRetried = true
          try {
            await this.refreshAccessToken()
          } catch {
            // 轮转失败已在 performTokenRefresh 内触发未登录处理，此处只回用户向错误
            return Promise.reject(transformApiError(apiResponse))
          }
          return this.client.request(requestConfig) as never
        }

        if (apiResponse.code === API_ERROR_CODES.UNAUTHORIZED) {
          this.config.onUnauthorized?.()
        }
        return Promise.reject(transformApiError(apiResponse))
      },
      (error: AxiosError) => Promise.reject(transformTransportError(error))
    )
  }

  /** shouldRefresh 只对后端明确的登录失效错误触发一次令牌轮转。 */
  private shouldRefresh(response: ApiResponse, requestConfig: RetriableRequestConfig): boolean {
    return response.code === API_ERROR_CODES.UNAUTHORIZED
      && Boolean(this.config.getRefreshToken?.())
      && !requestConfig.chaimirRetried
      && !requestConfig.url?.endsWith('/auth/refresh')
  }

  /** refreshAccessToken 单飞轮转 Refresh Token，并让并发失败请求等待同一结果。 */
  private refreshAccessToken(): Promise<void> {
    if (this.refreshPromise) return this.refreshPromise

    this.refreshPromise = this.performTokenRefresh().finally(() => {
      this.refreshPromise = null
    })
    return this.refreshPromise
  }

  /** performTokenRefresh 使用独立客户端完成轮转，避免刷新请求进入业务响应拦截器。 */
  private async performTokenRefresh(): Promise<void> {
    const refreshToken = this.config.getRefreshToken?.()
    if (!refreshToken) {
      this.config.onUnauthorized?.()
      throw new Error('refresh token unavailable')
    }

    try {
      const response = await axios.post<ApiResponse<TokenRefreshResponse>>(
        `${this.config.baseURL}/auth/refresh`,
        { refresh_token: refreshToken },
        { timeout: this.config.timeout, headers: { 'Content-Type': 'application/json' } },
      )
      const envelope = response.data
      if (!isEnvelope(envelope) || envelope.code !== API_SUCCESS_CODE
        || !envelope.data?.access_token || !envelope.data.refresh_token) {
        throw new Error('refresh response invalid')
      }
      this.config.onTokensRefreshed?.(envelope.data)
    } catch (error) {
      this.config.onUnauthorized?.()
      throw error
    }
  }

  // === HTTP 方法 ===

  /**
   * get 发送 GET 请求并返回后端信封中的 data 字段。
   */
  async get<T = unknown>(url: string, params?: object): Promise<T> {
    return this.client.get<unknown, T>(url, { params })
  }

  /**
   * post 发送 POST 请求并返回后端信封中的 data 字段。
   */
  async post<T = unknown>(url: string, data?: unknown): Promise<T> {
    return this.client.post<unknown, T>(url, data)
  }

  /**
   * put 发送 PUT 请求并返回后端信封中的 data 字段。
   */
  async put<T = unknown>(url: string, data?: unknown): Promise<T> {
    return this.client.put<unknown, T>(url, data)
  }

  /**
   * patch 发送 PATCH 请求并返回后端信封中的 data 字段。
   */
  async patch<T = unknown>(url: string, data?: unknown): Promise<T> {
    return this.client.patch<unknown, T>(url, data)
  }

  /**
   * delete 发送 DELETE 请求并返回后端信封中的 data 字段。
   */
  async delete<T = unknown>(url: string): Promise<T> {
    return this.client.delete<unknown, T>(url)
  }

  // === URL 构造 ===

  /**
   * 返回规范化后的 HTTP API 根地址，供 iframe 工具入口等浏览器原生能力使用。
   */
  public baseURL(): string {
    return normalizeBaseURL(this.config.baseURL)
  }

  /**
   * 基于后端 HTTP 根地址生成同源 WebSocket 入口地址。
   */
  public wsURL(path: string, query?: Record<string, string | undefined>): string {
    const wsBaseURL = toWebSocketBaseURL(this.config.wsBaseURL || this.baseURL())
    return `${wsBaseURL}${normalizePath(path)}${queryString(query)}`
  }

  /**
   * 基于 API 根地址推导同源根路径 WebSocket,用于后端 M10 的 /api/ws。
   */
  public rootWsURL(path: string, query?: Record<string, string | undefined>): string {
    const baseURL = this.config.wsBaseURL || this.baseURL()
    const apiRoot = API_BASE_PATH
    const originBase = baseURL.endsWith(apiRoot) ? baseURL.slice(0, -apiRoot.length) : baseURL
    const wsBaseURL = toWebSocketBaseURL(originBase)
    return `${wsBaseURL}${normalizePath(path)}${queryString(query)}`
  }

  /**
   * 基于后端 HTTP 根地址生成可直接交给浏览器的绝对地址。
   * 用于 <video src> 这类由浏览器自身发起请求、拿不到 Axios 拦截器的场景;
   * 鉴权由授权令牌承载(见统一文件服务 mode=stream),不在此拼接对象存储地址。
   */
  public absoluteURL(path: string, query?: Record<string, string | undefined>): string {
    return `${this.baseURL()}${normalizePath(path)}${queryString(query)}`
  }

  /**
   * 基于后端 HTTP 根地址生成浏览器工具代理入口地址。
   */
  public browserURL(path: string, query?: Record<string, string | undefined>): string {
    return `${this.baseURL()}${normalizePath(path)}${this.browserTokenQuery(query)}`
  }

  /**
   * 构造浏览器工具代理入口使用的一次性 token 查询参数。
   */
  public browserTokenQuery(extra?: Record<string, string | undefined>): string {
    const params = new URLSearchParams()
    for (const [key, value] of Object.entries(extra || {})) {
      if (value) {
        params.set(key, value)
      }
    }
    const token = this.config.getToken?.()
    if (token) {
      params.set('token', token)
    }
    const query = params.toString()
    return query ? `?${query}` : ''
  }

  // === 文件上传 ===

  /**
   * upload 以默认 file 字段提交单文件上传。
   */
  async upload<T = unknown>(url: string, file: File, onProgress?: (progress: number) => void): Promise<T> {
    const formData = new FormData()
    formData.append('file', file)

    return this.client.post<unknown, T>(url, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total)
          onProgress(progress)
        }
      },
    })
  }

  /**
   * 提交 multipart 表单，用于后端要求多个元数据字段和指定文件字段名的上传接口。
   */
  async postFormData<T = unknown>(
    url: string,
    formData: FormData,
    onProgress?: (progress: number) => void
  ): Promise<T> {
    return this.client.post<unknown, T>(url, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total)
          onProgress(progress)
        }
      },
    })
  }

  /**
   * 以 PATCH 方式提交 multipart 表单。
   */
  async patchFormData<T = unknown>(
    url: string,
    formData: FormData,
    onProgress?: (progress: number) => void
  ): Promise<T> {
    return this.client.patch<unknown, T>(url, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total)
          onProgress(progress)
        }
      },
    })
  }

  /**
   * 获取附件响应:返回文件内容与后端在 Content-Disposition 中声明的保存文件名。
   * 全站所有文件下载共用这一条 —— 文件名的单一来源是后端响应头
   * (`httpx.WriteAttachment*` 恒写入并已做单段化与字符白名单),
   * 客户端不自造名字、不按多个业务字段依次取值。
   */
  async getAttachment(url: string, params?: object): Promise<AttachmentResponse> {
    const response = await this.client.get<unknown, AttachmentResult>(url, {
      params,
      responseType: 'blob',
    })
    return { blob: response.blob, fileName: attachmentFileName(response.disposition) }
  }

}

/**
 * AttachmentResponse 是附件下载的对外结果。
 */
export interface AttachmentResponse {
  blob: Blob
  fileName: string
}

/** AttachmentResult 是响应拦截器为附件请求透出的内部载荷。 */
interface AttachmentResult {
  blob: Blob
  disposition: unknown
}

/**
 * attachmentFileName 从 Content-Disposition 解析保存文件名。
 * 后端出口恒写 `attachment; filename="<单段 ASCII 名>"`,取不到即说明响应不来自后端
 * (网关错误页等),按传输层失败处理 —— 不编造文件名掩盖这一情况。
 */
function attachmentFileName(disposition: unknown): string {
  const matched = typeof disposition === 'string' ? /filename="([^"]+)"/.exec(disposition) : null
  if (!matched) {
    throw new ApiError(API_TRANSPORT_ERROR_MESSAGES.MALFORMED_RESPONSE)
  }
  return matched[1]
}

/**
 * isEnvelope 判断 200 响应体是否为平台统一信封。
 * 后端所有业务出口都写信封(见 internal/platform/response),拿到非信封的 200
 * 说明响应来自网关或代理而不是后端,不能当业务结果解读。
 */
function isEnvelope(data: unknown): data is ApiResponse {
  if (!data || typeof data !== 'object') return false
  const candidate = data as Record<string, unknown>
  return typeof candidate.code === 'string' && typeof candidate.message === 'string'
}

/**
 * transformTransportError 处理「拿不到后端信封」的传输层失败。
 * 后端业务面恒返回 200 信封,所以进入 axios error 分支只有两种情况:
 * 请求根本没拿到响应(断网/DNS/超时/连接被拒),或网关直接写了非 2xx 响应。
 * 这两种情况下后端 message 与 trace_id 都不存在,只能用客户端固定文案表达。
 */
function transformTransportError(error: AxiosError): ApiError {
  return new ApiError(
    error.response
      ? API_TRANSPORT_ERROR_MESSAGES.MALFORMED_RESPONSE
      : API_TRANSPORT_ERROR_MESSAGES.NETWORK,
  )
}

/**
 * transformApiError 把后端业务错误信封转换为统一 ApiError。
 * message 与 trace_id 由后端签发(apperr 每个错误码都带用户向文案),前端原样透出,
 * 不改写、不补编号(§8 错误暴露分层)。
 */
function transformApiError(response: ApiResponse): ApiError {
  return new ApiError(response.message, response.code, response.trace_id)
}

/**
 * normalizeBaseURL 去掉末尾斜杠,避免 URL 拼接时出现双斜杠。
 */
function normalizeBaseURL(baseURL: string): string {
  return baseURL.replace(/\/+$/, '')
}

/**
 * normalizePath 确保路径以单个斜杠开头。
 */
function normalizePath(path: string): string {
  const trimmed = path.trim()
  if (!trimmed) {
    return ''
  }
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`
}

/**
 * queryString 把调用方显式传入的查询参数拼接到 URL,不自动携带登录凭证。
 */
function queryString(extra?: Record<string, string | undefined>): string {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(extra || {})) {
    if (value) {
      params.set(key, value)
    }
  }
  const query = params.toString()
  return query ? `?${query}` : ''
}

/**
 * toWebSocketBaseURL 把 HTTP API 根地址转换为浏览器可直接连接的 WebSocket 根地址。
 */
function toWebSocketBaseURL(baseURL: string): string {
  if (!baseURL && typeof window !== 'undefined') {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}`
  }
  if (/^http:\/\//.test(baseURL)) {
    return baseURL.replace(/^http:/, 'ws:')
  }
  if (/^https:\/\//.test(baseURL)) {
    return baseURL.replace(/^https:/, 'wss:')
  }
  if (baseURL.startsWith('/') && typeof window !== 'undefined') {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}${baseURL}`
  }
  return baseURL
}
