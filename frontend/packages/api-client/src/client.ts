// API 客户端核心：封装后端统一信封、鉴权头、trace_id 透传和用户向错误。

import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'

export interface ApiConfig {
  baseURL: string
  timeout?: number
  getToken?: () => string | null
  onUnauthorized?: () => void
  getTraceId?: () => string | null
}

export interface ApiResponse<T = unknown> {
  data?: T
  code?: string
  message?: string
  trace_id?: string
}

export interface ApiError {
  message: string
  code?: string
  traceId?: string
  status?: number
}

export class ApiClient {
  private client: AxiosInstance
  private config: ApiConfig

  /**
   * constructor 创建绑定后端 /api/v1 根路径的 Axios 客户端。
   */
  constructor(config: ApiConfig) {
    this.config = {
      ...config,
      baseURL: normalizeBaseURL(config.baseURL),
    }
    this.client = axios.create({
      baseURL: this.config.baseURL,
      timeout: config.timeout || 30000,
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
        const traceId = this.config.getTraceId?.()
        if (traceId && config.headers) {
          config.headers['X-Trace-Id'] = traceId
        }
        return config
      },
      (error) => {
        return Promise.reject(this.transformError(error))
      }
    )

    // 响应拦截：统一错误处理
    this.client.interceptors.response.use(
      (response) => {
        // 后端统一响应格式：{ code, message, data, trace_id }
        const apiResponse: ApiResponse = response.data

        // 如果有 code 且不是成功码，视为业务错误
        if (apiResponse.code !== undefined && !isSuccessCode(apiResponse.code)) {
          return Promise.reject(this.transformApiError(apiResponse, response.status))
        }

        // 返回 data 字段
        return (apiResponse.data !== undefined ? apiResponse.data : apiResponse) as never
      },
      (error: AxiosError) => {
        // HTTP 错误或网络错误
        if (error.response?.status === 401) {
          this.config.onUnauthorized?.()
        }
        return Promise.reject(this.transformError(error))
      }
    )
  }

  /**
   * transformError 把 HTTP 或网络错误收敛为前端可展示的用户向错误对象。
   */
  private transformError(error: AxiosError): ApiError {
    const response = error.response?.data as ApiResponse | undefined

    // FE-8: 只暴露用户友好的 message + trace_id
    return {
      message: response?.message || error.message || '网络请求失败，请稍后重试',
      code: response?.code,
      traceId: response?.trace_id,
      status: error.response?.status,
    }
  }

  /**
   * transformApiError 把后端业务错误信封转换为统一 ApiError。
   */
  private transformApiError(response: ApiResponse, status: number): ApiError {
    return {
      message: response.message || '操作失败',
      code: response.code,
      traceId: response.trace_id,
      status,
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
    const wsBaseURL = toWebSocketBaseURL(this.baseURL())
    return `${wsBaseURL}${normalizePath(path)}${this.browserTokenQuery(query)}`
  }

  /**
   * 基于 API 根地址推导同源根路径 WebSocket,用于后端 M10 的 /api/ws。
   */
  public rootWsURL(path: string, query?: Record<string, string | undefined>): string {
    const baseURL = this.baseURL()
    const apiRoot = '/api/v1'
    const originBase = baseURL.endsWith(apiRoot) ? baseURL.slice(0, -apiRoot.length) : baseURL
    const wsBaseURL = toWebSocketBaseURL(originBase)
    return `${wsBaseURL}${normalizePath(path)}${this.browserTokenQuery(query)}`
  }

  /**
   * 基于后端 HTTP 根地址生成浏览器工具代理入口地址。
   */
  public browserURL(path: string, query?: Record<string, string | undefined>): string {
    return `${this.baseURL()}${normalizePath(path)}${this.browserTokenQuery(query)}`
  }

  /**
   * 构造浏览器无法设置 Authorization 头时使用的一次性 token 查询参数。
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
   * 获取二进制响应体。
   */
  async getBlob(url: string, params?: object): Promise<Blob> {
    return this.client.get<unknown, Blob>(url, {
      params,
      responseType: 'blob',
    })
  }

  // === 文件下载 ===

  /**
   * download 获取二进制响应并触发浏览器下载。
   */
  async download(url: string, filename: string): Promise<void> {
    const blob = await this.client.get<unknown, Blob>(url, {
      responseType: 'blob',
    })

    const link = document.createElement('a')
    link.href = window.URL.createObjectURL(blob)
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(link.href)
  }
}

/**
 * isSuccessCode 判断后端统一信封是否表示成功。
 */
function isSuccessCode(code: string): boolean {
  return code === '0' || code === 'OK'
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
 * toWebSocketBaseURL 把 HTTP API 根地址转换为浏览器可直接连接的 WebSocket 根地址。
 */
function toWebSocketBaseURL(baseURL: string): string {
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
