// Judge API：判题
// 对应后端 M3 模块

import { ApiClient } from '../client'
import type { JudgeTask, SubmitJudgeRequest, PaginatedResponse } from '../types'

export class JudgeApi {
  constructor(private client: ApiClient) {}

  /**
   * 提交判题任务
   */
  async submitTask(data: SubmitJudgeRequest): Promise<JudgeTask> {
    return this.client.post('/judge/tasks', data)
  }

  /**
   * 获取判题任务详情
   */
  async getTask(taskId: string): Promise<JudgeTask> {
    return this.client.get(`/judge/tasks/${taskId}`)
  }

  /**
   * 获取判题任务列表
   */
  async getTasks(params?: {
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<JudgeTask>> {
    return this.client.get('/judge/tasks', params)
  }

  /**
   * 取消判题任务
   */
  async cancelTask(taskId: string): Promise<void> {
    return this.client.post(`/judge/tasks/${taskId}/cancel`)
  }
}
