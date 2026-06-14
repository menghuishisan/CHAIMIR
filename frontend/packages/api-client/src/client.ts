// API 客户端核心：HTTP 请求封装
// 符合 FE-8：错误分层暴露（只展示 message + trace_id）

import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'

export interface ApiConfig {
  baseURL: string
  timeout?: number
  getToken?: () => string | null
  onUnauthorized?: () => void
}

export interface ApiResponse<T = any> {
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

  constructor(config: ApiConfig) {
    this.config = config
    this.client = axios.create({
      baseURL: config.baseURL,
      timeout: config.timeout || 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    })

    this.setupInterceptors()
  }

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
        return Promise.reject(this.transformError(error))
      }
    )

    // 响应拦截：统一错误处理
    this.client.interceptors.response.use(
      (response) => {
        // 后端统一响应格式：{ code, message, data, trace_id }
        const apiResponse: ApiResponse = response.data

        // 如果有 code 且不是成功码，视为业务错误
        if (apiResponse.code && apiResponse.code !== '0' && apiResponse.code !== 'OK') {
          return Promise.reject(this.transformApiError(apiResponse, response.status))
        }

        // 返回 data 字段
        return apiResponse.data !== undefined ? apiResponse.data : apiResponse
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

  private transformApiError(response: ApiResponse, status: number): ApiError {
    return {
      message: response.message || '操作失败',
      code: response.code,
      traceId: response.trace_id,
      status,
    }
  }

  // === HTTP 方法 ===

  async get<T = any>(url: string, params?: any): Promise<T> {
    return this.client.get<any, T>(url, { params })
  }

  async post<T = any>(url: string, data?: any): Promise<T> {
    return this.client.post<any, T>(url, data)
  }

  async put<T = any>(url: string, data?: any): Promise<T> {
    return this.client.put<any, T>(url, data)
  }

  async patch<T = any>(url: string, data?: any): Promise<T> {
    return this.client.patch<any, T>(url, data)
  }

  async delete<T = any>(url: string): Promise<T> {
    return this.client.delete<any, T>(url)
  }

  // === 文件上传 ===

  async upload<T = any>(url: string, file: File, onProgress?: (progress: number) => void): Promise<T> {
    const formData = new FormData()
    formData.append('file', file)

    return this.client.post<any, T>(url, formData, {
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
  async postFormData<T = any>(
    url: string,
    formData: FormData,
    onProgress?: (progress: number) => void
  ): Promise<T> {
    return this.client.post<any, T>(url, formData, {
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
  async patchFormData<T = any>(
    url: string,
    formData: FormData,
    onProgress?: (progress: number) => void
  ): Promise<T> {
    return this.client.patch<any, T>(url, formData, {
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
  async getBlob(url: string, params?: any): Promise<Blob> {
    return this.client.get<any, Blob>(url, {
      params,
      responseType: 'blob',
    })
  }

  // === 文件下载 ===

  async download(url: string, filename: string): Promise<void> {
    const response = await this.client.get(url, {
      responseType: 'blob',
    })

    const blob = new Blob([response.data])
    const link = document.createElement('a')
    link.href = window.URL.createObjectURL(blob)
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(link.href)
  }
}
