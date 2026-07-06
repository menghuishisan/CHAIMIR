// Sim API：仿真引擎
// 对应后端 M4 模块

import { ApiClient } from '../client'
import type { SimPackageStatus, SimReviewResult } from '../constants/sim'
import type { PaginatedResponse } from '../types/common'
import type {
  SimPackageMeta,
  SimPackageReview,
  SimBundleDownloadGrant,
  SimPackageSubmissionResult,
  SimReviewDecision,
  SimActionLog,
  SimPackageSubmit,
  SimReplay,
  SimShareCreate,
  SimShareResult,
} from '../types/sim'

/**
 * SimApi 封装后端 M4 仿真包、审核、回放、分享和实时流接口。
 */
export class SimApi {
  /**
   * constructor 注入统一 API 客户端，复用 multipart、鉴权和 WebSocket URL 规则。
   */
  constructor(private client: ApiClient) {}

  // ===== 仿真包管理 =====

  /**
   * 获取仿真包列表
   */
  async getPackages(params?: {
    category?: string
    keyword?: string
    status?: SimPackageStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<SimPackageMeta>> {
    return this.client.get('/sim/packages', params)
  }

  /**
   * 获取指定仿真包的所有版本
   */
  async getPackageVersions(code: string): Promise<SimPackageMeta[]> {
    return this.client.get(`/sim/packages/${code}/versions`)
  }

  /**
   * 提交仿真包，字段名与后端 multipart 绑定保持一致
   */
  async submitPackage(
    data: SimPackageSubmit,
    onProgress?: (progress: number) => void
  ): Promise<SimPackageSubmissionResult> {
    return this.client.postFormData('/sim/packages', packageFormData(data), onProgress)
  }

  /**
   * 更新草稿或退回后的仿真包
   */
  async updatePackage(
    packageId: string,
    data: SimPackageSubmit,
    onProgress?: (progress: number) => void
  ): Promise<SimPackageSubmissionResult> {
    return this.client.patchFormData(`/sim/packages/${packageId}`, packageFormData(data), onProgress)
  }

  /**
   * 获取仿真包 bundle 短时下载授权
   */
  async getBundleGrant(code: string, version: string): Promise<SimBundleDownloadGrant> {
    return this.client.get(`/sim/packages/${code}/${version}/bundle`)
  }

  /**
   * 获取审核前预览报告
   */
  async previewPackage(packageId: string): Promise<SimPackageReview> {
    return this.client.get(`/sim/packages/${packageId}/preview`)
  }

  // ===== 仿真包审核 =====

  /**
   * 获取审核列表（审核员）
   */
  async getReviews(params?: {
    result?: SimReviewResult
    page?: number
    size?: number
  }): Promise<PaginatedResponse<SimPackageReview>> {
    return this.client.get('/sim/reviews', params)
  }

  /**
   * 审核通过
   */
  async approveReview(reviewId: string): Promise<SimReviewDecision> {
    return this.client.post(`/sim/reviews/${reviewId}/approve`)
  }

  /**
   * 审核退回
   */
  async rejectReview(reviewId: string, comment: string): Promise<SimReviewDecision> {
    return this.client.post(`/sim/reviews/${reviewId}/reject`, { comment })
  }

  /**
   * 下架已发布仿真包
   */
  async archivePackage(packageId: string): Promise<SimPackageMeta> {
    return this.client.post(`/sim/packages/${packageId}/archive`)
  }

  /**
   * 重新上架已下架仿真包
   */
  async republishPackage(packageId: string): Promise<SimPackageMeta> {
    return this.client.post(`/sim/packages/${packageId}/republish`)
  }

  // ===== 仿真会话 =====

  /**
   * 上报用户操作序列
   */
  async reportAction(sessionId: string, action: SimActionLog): Promise<SimActionLog> {
    return this.client.post(`/sim/sessions/${sessionId}/actions`, action)
  }

  /**
   * 获取可复现回放数据
   */
  async getReplay(sessionId: string): Promise<SimReplay> {
    return this.client.get(`/sim/sessions/${sessionId}/replay`)
  }

  /**
   * 创建公开分享码
   */
  async shareSession(sessionId: string, data: SimShareCreate = {}): Promise<SimShareResult> {
    return this.client.post(`/sim/sessions/${sessionId}/share`, data)
  }

  /**
   * 读取公开分享剧本
   */
  async getSharedReplay(code: string): Promise<SimReplay> {
    return this.client.get(`/sim/shared/${code}`)
  }

  /**
   * 获取后端计算仿真的 WebSocket URL。
   */
  getStreamWsUrl(sessionId: string): string {
    return this.client.wsURL(`/sim/sessions/${sessionId}/stream`)
  }
}

/**
 * 构造后端 M4 multipart 上传所需字段。
 */
function packageFormData(data: SimPackageSubmit): FormData {
  const formData = new FormData()
  formData.append('bundle', data.bundle)
  formData.append('code', data.code)
  formData.append('version', data.version)
  formData.append('name', data.name)
  formData.append('category', data.category)
  formData.append('compute', data.compute)
  formData.append('scale_limit', JSON.stringify(data.scale_limit ?? {}))
  formData.append('backend_config', JSON.stringify(data.backend_config ?? {}))
  if (data.backend_adapter) {
    formData.append('backend_adapter', data.backend_adapter)
  }
  return formData
}
