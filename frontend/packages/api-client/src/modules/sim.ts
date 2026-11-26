// 仿真包管理、回放与隔离执行客户端。

import { ApiClient, encodePathSegment } from '../client'
import type { SimPackageStatus, SimReviewResult } from '../constants/sim'
import type { PaginatedResponse } from '../types/common'
import type {
  SimPackageMeta,
  SimPackageReviewDetail,
  SimPackageReview,
  SimPackageSubmissionResult,
  SimActionLog,
  SimPackageSubmit,
  SimReplay,
  SimShareCreate,
  SimShareResult,
} from '../types/sim'

/**
 * SimApi 封装后端 M4 仿真包、审核、回放、分享和实时流接口。
 *
 * 没有 bundle 下载授权方法:内置包由 sim-sdk 在浏览器 Worker 内按 code 装配,
 * 扩展包的归档只经受控 k8s exec 投入隔离容器,从不下发浏览器。
 */
export class SimApi {
  constructor(private client: ApiClient) {}

  /**
   * 获取仿真包列表。
   * 缺省只回已上架包(status 只接受 published);传 mine=true 时按服务端会话回本人提交的包,
   * 此时 status 可取任一状态、缺省返回全部状态 —— 教师据此看到自己被退回或仍是草稿的包。
   */
  async getPackages(params?: {
    category?: string
    keyword?: string
    status?: SimPackageStatus
    mine?: boolean
    page?: number
    size?: number
  }): Promise<PaginatedResponse<SimPackageMeta>> {
    return this.client.get('/sim/packages', params)
  }

  async getPackageVersions(code: string): Promise<SimPackageMeta[]> {
    return this.client.get(`/sim/packages/${encodePathSegment(code)}/versions`)
  }

  async submitPackage(
    data: SimPackageSubmit,
    onProgress?: (progress: number) => void
  ): Promise<SimPackageSubmissionResult> {
    return this.client.sendFormData('POST', '/sim/packages', packageFormData(data), onProgress)
  }

  async updatePackage(
    packageId: string,
    data: SimPackageSubmit,
    onProgress?: (progress: number) => void
  ): Promise<SimPackageSubmissionResult> {
    return this.client.sendFormData('PATCH', `/sim/packages/${encodePathSegment(packageId)}`, packageFormData(data), onProgress)
  }

  /**
   * 获取审核前预览报告(仅包作者本人可读)。
   */
  async previewPackage(packageId: string): Promise<SimPackageReviewDetail> {
    return this.client.get(`/sim/packages/${encodePathSegment(packageId)}/preview`)
  }

  async getReviews(params?: {
    result?: SimReviewResult
    page?: number
    size?: number
  }): Promise<PaginatedResponse<SimPackageReview>> {
    return this.client.get('/sim/reviews', params)
  }

  async approveReview(reviewId: string): Promise<SimPackageReviewDetail> {
    return this.client.post(`/sim/reviews/${encodePathSegment(reviewId)}/approve`)
  }

  async rejectReview(reviewId: string, comment: string): Promise<SimPackageReviewDetail> {
    return this.client.post(`/sim/reviews/${encodePathSegment(reviewId)}/reject`, { comment })
  }

  async archivePackage(packageId: string): Promise<SimPackageMeta> {
    return this.client.post(`/sim/packages/${encodePathSegment(packageId)}/archive`)
  }

  async republishPackage(packageId: string): Promise<SimPackageMeta> {
    return this.client.post(`/sim/packages/${encodePathSegment(packageId)}/republish`)
  }

  async reportAction(sessionId: string, action: SimActionLog): Promise<SimActionLog> {
    return this.client.post(`/sim/sessions/${encodePathSegment(sessionId)}/actions`, action)
  }

  async getReplay(sessionId: string): Promise<SimReplay> {
    return this.client.get(`/sim/sessions/${encodePathSegment(sessionId)}/replay`)
  }

  async shareSession(sessionId: string, data: SimShareCreate = {}): Promise<SimShareResult> {
    return this.client.post(`/sim/sessions/${encodePathSegment(sessionId)}/share`, data)
  }

  async getSharedReplay(code: string): Promise<SimReplay> {
    return this.client.get(`/sim/shared/${encodePathSegment(code)}`)
  }

  /**
   * 获取隔离执行仿真的 WebSocket URL。
   * 扩展包与重计算仿真经这条流运行:容器算出完整教学帧,浏览器只渲染。
   */
  getStreamWsUrl(sessionId: string): string {
    return this.client.wsURL(`/sim/sessions/${encodePathSegment(sessionId)}/stream`)
  }
}

function packageFormData(data: SimPackageSubmit): FormData {
  const formData = new FormData()
  formData.append('bundle', data.bundle)
  formData.append('code', data.code)
  formData.append('version', data.version)
  formData.append('name', data.name)
  formData.append('category', data.category)
  formData.append('scale_limit', JSON.stringify(data.scale_limit))
  return formData
}
