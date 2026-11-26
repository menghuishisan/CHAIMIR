import axios, {
  type AxiosError,
  type AxiosInstance,
  type AxiosRequestConfig,
  type InternalAxiosRequestConfig,
} from 'axios'
import {
  API_BASE_PATH,
  API_ERROR_CODES,
  API_SUCCESS_CODE,
  API_TRANSPORT_ERROR_MESSAGES,
} from './constants'

export interface ApiConfig {
  baseURL: string
  wsBaseURL?: string
  /** 请求超时毫秒数:由装配层从部署环境变量读入,SDK 不设默认阈值 */
  timeout: number
  getToken?: () => string | null
  hasRefreshSession?: () => boolean
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
  must_change_pwd?: boolean
}

interface RetriableRequestConfig extends InternalAxiosRequestConfig {
  chaimirRetried?: boolean
}

export class ApiClient {
  private readonly client: AxiosInstance
  private readonly config: ApiConfig
  private refreshPromise: Promise<void> | null = null

  constructor(config: ApiConfig) {
    this.config = {
      ...config,
      baseURL: normalizeBaseURL(config.baseURL),
      wsBaseURL: config.wsBaseURL ? normalizeBaseURL(config.wsBaseURL) : undefined,
    }
    this.client = axios.create({
      baseURL: this.config.baseURL,
      timeout: this.config.timeout,
      withCredentials: true,
    })

    this.setupInterceptors()
  }

  private setupInterceptors() {
    this.client.interceptors.request.use(
      (config: InternalAxiosRequestConfig) => {
        const token = this.config.getToken?.()
        if (token && config.headers) {
          config.headers.Authorization = `Bearer ${token}`
        }
        if (config.headers) {
          // FormData 的 boundary 必须由浏览器生成;固定 JSON 头会让 multipart 请求缺少 boundary。
          if (typeof FormData !== 'undefined' && config.data instanceof FormData) {
            delete config.headers['Content-Type']
          } else if (!config.headers['Content-Type']) {
            config.headers['Content-Type'] = 'application/json'
          }
        }
        return config
      },
      (error) => {
        return Promise.reject(transformTransportError(error))
      }
    )

    // HTTP 200 仅表示收到业务响应，具体结果由信封的 code 决定。
    this.client.interceptors.response.use(
      async (response) => {
        const requestConfig = response.config as RetriableRequestConfig

        // 成功下载按原样透出;失败仍是平台统一 JSON 信封,必须复用同一错误和刷新链路。
        if (requestConfig.responseType === 'blob') {
          const disposition = response.headers['content-disposition']
          if (typeof disposition === 'string' && disposition.trim() !== '') {
            return { blob: response.data, disposition } as never
          }
          const apiResponse = await envelopeFromBlob(response.data)
          if (!apiResponse || apiResponse.code === API_SUCCESS_CODE) {
            return Promise.reject(new ApiError(API_TRANSPORT_ERROR_MESSAGES.MALFORMED_RESPONSE))
          }
          return this.handleEnvelope(apiResponse, requestConfig) as never
        }

        if (!isEnvelope(response.data)) {
          return Promise.reject(new ApiError(API_TRANSPORT_ERROR_MESSAGES.MALFORMED_RESPONSE))
        }

        return this.handleEnvelope(response.data, requestConfig) as never
      },
      async (error: AxiosError) => {
        const requestConfig = error.config as RetriableRequestConfig | undefined
        if (requestConfig?.responseType === 'blob' && error.response) {
          const apiResponse = await envelopeFromBlob(error.response.data)
          if (apiResponse && apiResponse.code !== API_SUCCESS_CODE) {
            try {
              return await this.handleEnvelope(apiResponse, requestConfig)
            } catch (cause) {
              return Promise.reject(cause)
            }
          }
        }
        return Promise.reject(transformTransportError(error))
      }
    )
  }

  /** handleEnvelope 统一处理普通 JSON 请求与 Blob 失败响应中的平台信封。 */
  private async handleEnvelope(
    apiResponse: ApiResponse,
    requestConfig: RetriableRequestConfig
  ): Promise<unknown> {
    if (apiResponse.code === API_SUCCESS_CODE) {
      return apiResponse.data !== undefined ? apiResponse.data : apiResponse
    }

    // 登录态失效：用 Refresh Token 轮转一次并重放原请求。
    if (this.shouldRefresh(apiResponse, requestConfig)) {
      requestConfig.chaimirRetried = true
      // 刷新错误保留自己的类别和报障编号，临时网络故障不能被改写为原请求的登录失效。
      await this.refreshAccessToken()
      return this.request(requestConfig)
    }

    if (requiresReauthentication(apiResponse.code)) {
      this.config.onUnauthorized?.()
    }
    throw transformApiError(apiResponse)
  }

  /** shouldRefresh 只对后端明确的登录失效错误触发一次令牌轮转。 */
  private shouldRefresh(response: ApiResponse, requestConfig: RetriableRequestConfig): boolean {
    return (
      response.code === API_ERROR_CODES.UNAUTHORIZED &&
      Boolean(this.config.hasRefreshSession?.()) &&
      !requestConfig.chaimirRetried &&
      !requestConfig.url?.endsWith('/auth/refresh')
    )
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
    const response = await axios
      .post<ApiResponse<TokenRefreshResponse>>(
        `${this.config.baseURL}/auth/refresh`,
        {},
        {
          timeout: this.config.timeout,
          withCredentials: true,
          headers: { 'Content-Type': 'application/json', 'X-Chaimir-Refresh': '1' },
        }
      )
      .catch((error: AxiosError) => {
        throw transformTransportError(error)
      })
    const envelope = response.data
    if (!isEnvelope(envelope)) {
      throw new ApiError(API_TRANSPORT_ERROR_MESSAGES.MALFORMED_RESPONSE)
    }
    if (envelope.code !== API_SUCCESS_CODE) {
      // 仅服务端明确拒绝当前会话时清除登录态；其他业务失败仍保留原信封。
      if (requiresReauthentication(envelope.code)) this.config.onUnauthorized?.()
      throw transformApiError(envelope)
    }
    if (typeof envelope.data?.access_token !== 'string' || !envelope.data.access_token.trim()) {
      throw new ApiError(API_TRANSPORT_ERROR_MESSAGES.MALFORMED_RESPONSE)
    }
    this.config.onTokensRefreshed?.(envelope.data)
  }

  private request<T>(config: AxiosRequestConfig): Promise<T> {
    // 响应拦截器已取出业务数据，Axios 类型仍声明为原始响应。
    return this.client.request(config) as Promise<unknown> as Promise<T>
  }

  get<T = unknown>(url: string, params?: object): Promise<T> {
    return this.request({ method: 'GET', url, params })
  }

  post<T = unknown>(url: string, data?: unknown): Promise<T> {
    return this.request({ method: 'POST', url, data })
  }

  /** 仅用于服务端已有执行时限的操作，避免普通客户端超时提前中断请求。 */
  postWithoutClientTimeout<T = unknown>(url: string, data?: unknown): Promise<T> {
    return this.request({ method: 'POST', url, data, timeout: 0 })
  }

  put<T = unknown>(url: string, data?: unknown): Promise<T> {
    return this.request({ method: 'PUT', url, data })
  }

  patch<T = unknown>(url: string, data?: unknown): Promise<T> {
    return this.request({ method: 'PATCH', url, data })
  }

  delete<T = unknown>(url: string): Promise<T> {
    return this.request({ method: 'DELETE', url })
  }

  // === URL 构造 ===

  /**
   * 返回规范化后的 HTTP API 根地址，供 iframe 工具入口等浏览器原生能力使用。
   */
  public baseURL(): string {
    return this.config.baseURL
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
   * 基于独立工具 origin 生成浏览器工具代理入口地址;鉴权 query 由上层短时票据接口显式传入。
   * 工具页必须与平台页面分 origin，避免被嵌入页面读取平台 DOM 或同源凭据。
   */
  public browserURLAtOrigin(
    origin: string,
    path: string,
    query?: Record<string, string | undefined>
  ): string {
    let parsedOrigin: URL
    try {
      parsedOrigin = new URL(origin.trim())
    } catch {
      throw new Error('工具代理 origin 必须配置为有效的 HTTPS origin')
    }
    if (
      parsedOrigin.protocol !== 'https:' ||
      parsedOrigin.username ||
      parsedOrigin.password ||
      parsedOrigin.pathname !== '/' ||
      parsedOrigin.search ||
      parsedOrigin.hash
    ) {
      throw new Error('工具代理 origin 必须使用 HTTPS')
    }
    return `${parsedOrigin.origin}${normalizePath(path)}${queryString(query)}`
  }

  // === 文件上传 ===

  /** 后端单文件上传协议使用 file 字段。 */
  upload<T = unknown>(
    url: string,
    file: File,
    onProgress?: (progress: number) => void
  ): Promise<T> {
    const formData = new FormData()
    formData.append('file', file)
    return this.sendFormData('POST', url, formData, onProgress)
  }

  sendFormData<T = unknown>(
    method: 'POST' | 'PATCH',
    url: string,
    data: FormData,
    onProgress?: (progress: number) => void
  ): Promise<T> {
    return this.request({
      method,
      url,
      data,
      onUploadProgress: onProgress && (({ loaded, total }) => {
        if (total) onProgress(Math.round((loaded * 100) / total))
      }),
    })
  }

  /**
   * 获取附件响应:返回文件内容与后端在 Content-Disposition 中声明的保存文件名。
   * 全站所有文件下载共用这一条 —— 文件名的单一来源是后端响应头
   * (`httpx.WriteAttachment*` 恒写入并已做单段化与字符白名单),
   * 客户端不自造名字、不按多个业务字段依次取值。
   */
  async getAttachment(url: string, params?: object): Promise<AttachmentResponse> {
    const response = await this.request<AttachmentResult>({
      method: 'GET',
      url,
      params,
      responseType: 'blob',
    })
    return { blob: response.blob, fileName: attachmentFileName(response.disposition) }
  }
}

/** encodePathSegment 将业务标识编码为单一路径段，避免标识中的斜杠或查询分隔符改变 API 路由。 */
export function encodePathSegment(value: string): string {
  return encodeURIComponent(value)
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

/** requiresReauthentication 统一识别需要结束本地受保护会话的服务端错误码。 */
function requiresReauthentication(code: string): boolean {
  return (
    code === API_ERROR_CODES.UNAUTHORIZED ||
    code === API_ERROR_CODES.IDENTITY_SESSION_INVALID ||
    code === API_ERROR_CODES.IDENTITY_SESSION_CONTEXT_MISSING
  )
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

/** envelopeFromBlob 从无附件响应头的响应体中恢复平台错误信封。 */
async function envelopeFromBlob(data: unknown): Promise<ApiResponse | undefined> {
  let text: string
  if (typeof data === 'string') {
    text = data
  } else if (typeof Blob !== 'undefined' && data instanceof Blob) {
    text = await data.text()
  } else if (data instanceof ArrayBuffer) {
    text = new TextDecoder().decode(data)
  } else if (ArrayBuffer.isView(data)) {
    text = new TextDecoder().decode(data)
  } else {
    return undefined
  }
  try {
    const parsed: unknown = JSON.parse(text)
    return isEnvelope(parsed) ? parsed : undefined
  } catch {
    return undefined
  }
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
      : API_TRANSPORT_ERROR_MESSAGES.NETWORK
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
