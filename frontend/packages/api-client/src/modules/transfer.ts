// Transfer API 文件定义统一导入导出任务中心前端调用入口。

import { ApiClient } from '../client'
import type { TransferChannel, TransferStatus } from '../constants/transfer'
import type { TransferDownloadGrant, TransferTask, TransferTaskListResponse } from '../types/transfer'

/**
 * TransferApi 封装后端 platform/transfer 统一任务查询和下载授权接口。
 */
export class TransferApi {
  /**
   * constructor 注入统一 API 客户端，避免各业务模块重复实现导入导出任务轮询。
   */
  constructor(private client: ApiClient) {}

  /**
   * listTasks 查询当前账号可见的导入导出任务。
   */
  async listTasks(params?: {
    channel?: TransferChannel
    status?: TransferStatus
    page?: number
    size?: number
  }): Promise<TransferTaskListResponse> {
    return this.client.get('/transfer/tasks', params)
  }

  /**
   * getTask 读取当前账号可见的单个导入导出任务。
   */
  async getTask(taskId: string): Promise<TransferTask> {
    return this.client.get(`/transfer/tasks/${taskId}`)
  }

  /**
   * downloadGrant 为已完成任务签发短时下载授权。
   */
  async downloadGrant(taskId: string): Promise<TransferDownloadGrant> {
    return this.client.post(`/transfer/tasks/${taskId}/download-grant`)
  }

  /**
   * downloadArtifact 签发并立即消费一次性授权，返回可由浏览器保存的文件内容。
   */
  async downloadArtifact(taskId: string): Promise<{ blob: Blob; fileName: string }> {
    const grant = await this.downloadGrant(taskId)
    const blob = await this.client.getBlob('/storage/download', { token: grant.token })
    return {
      blob,
      fileName: grant.task.artifact_file_name || grant.task.file_name || 'download',
    }
  }
}
