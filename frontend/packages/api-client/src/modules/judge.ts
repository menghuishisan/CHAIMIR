// Judge API：判题
// 对应后端 M3 模块

import { ApiClient } from '../client'
import type { JudgeTask, JudgeManualScoreRequest, PaginatedResponse } from '../types'

export class JudgeApi {
  constructor(private client: ApiClient) {}

  /**
   * 获取判题任务详情
   */
  async getTask(taskId: string): Promise<JudgeTask> {
    return this.client.get(`/judge/tasks/${taskId}`)
  }

  /**
   * 获取判题进度 WebSocket URL
   */
  getProgressWsUrl(taskId: string): string {
    const baseUrl = this.client['config'].baseURL || ''
    const wsProtocol = baseUrl.startsWith('https') ? 'wss' : 'ws'
    const wsBaseUrl = baseUrl.replace(/^https?/, wsProtocol)
    return `${wsBaseUrl}/judge/tasks/${taskId}/progress`
  }

  /**
   * 获取判题任务列表
   */
  async getTasks(params?: {
    source_ref?: string
    pending_manual?: boolean
    page?: number
    size?: number
  }): Promise<PaginatedResponse<JudgeTask>> {
    return this.client.get('/judge/tasks', params)
  }

  /**
   * 按原始快照重判
   */
  async rejudgeTask(taskId: string): Promise<JudgeTask> {
    return this.client.post(`/judge/tasks/${taskId}/rejudge`)
  }

  /**
   * 提交人工评分结果
   */
  async manualScore(taskId: string, data: JudgeManualScoreRequest): Promise<JudgeTask> {
    return this.client.post(`/judge/tasks/${taskId}/manual-score`, data)
  }
}
