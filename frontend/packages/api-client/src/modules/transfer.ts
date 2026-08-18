// Transfer API 文件定义统一导入导出任务中心前端调用入口。

import { ApiClient, encodePathSegment } from '../client'
import type { AttachmentResponse } from '../client'
import type { PaginatedResponse } from '../types/common'
import type { TransferChannel, TransferStatus } from '../constants/transfer'
import type { TransferDownloadGrant, TransferTask } from '../types/transfer'
import type { StorageApi } from './storage'

/**
 * TransferApi 封装后端 platform/transfer 统一任务查询和下载授权接口。
 */
export class TransferApi {
  /**
   * constructor 注入统一 API 客户端与统一文件服务入口。
   * 取件不在此重复实现:transfer 只负责签发任务产物授权,消费统一走 storage。
   */
  constructor(
    private client: ApiClient,
    private storage: StorageApi,
  ) {}

  /**
   * listTasks 查询当前账号可见的导入导出任务。
   */
  async listTasks(params?: {
    channel?: TransferChannel
    status?: TransferStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<TransferTask>> {
    return this.client.get('/transfer/tasks', params)
  }

  /**
   * getTask 读取当前账号可见的单个导入导出任务。
   */
  async getTask(taskId: string): Promise<TransferTask> {
    return this.client.get(`/transfer/tasks/${encodePathSegment(taskId)}`)
  }

  /**
   * downloadGrant 为已完成任务签发短时下载授权。
   */
  async downloadGrant(taskId: string): Promise<TransferDownloadGrant> {
    return this.client.post(`/transfer/tasks/${encodePathSegment(taskId)}/download-grant`)
  }

  /**
   * downloadArtifact 签发并立即消费一次性授权，返回可由浏览器保存的文件内容。
   * 取件经统一文件服务，文件名取自其响应头(后端已做单段化)，与全站其他下载同一条通道。
   */
  async downloadArtifact(taskId: string): Promise<AttachmentResponse> {
    const grant = await this.downloadGrant(taskId)
    return this.storage.consumeGrant(grant.token)
  }
}
