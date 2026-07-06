// Judge API：判题
// 对应后端 M3 模块

import { ApiClient } from '../client'
import type { PaginatedResponse } from '../types/common'
import type { JudgeTask, JudgeManualScoreRequest, Judger, JudgerRequest } from '../types/judge'

/**
 * JudgeApi 封装后端 M3 判题器、判题任务和人工评分接口。
 */
export class JudgeApi {
  /**
   * constructor 注入统一 API 客户端，确保判题模块复用同一错误处理和鉴权链路。
   */
  constructor(private client: ApiClient) {}

  /**
   * 查询判题器列表。
   */
  async listJudgers(): Promise<Judger[]> {
    return this.client.get('/judge/judgers')
  }

  /**
   * 创建判题器配置。
   */
  async createJudger(data: JudgerRequest): Promise<Judger> {
    return this.client.post('/judge/judgers', data)
  }

  /**
   * 更新判题器配置。
   */
  async updateJudger(judgerId: string, data: JudgerRequest): Promise<Judger> {
    return this.client.patch(`/judge/judgers/${judgerId}`, data)
  }

  /**
   * 触发判题器自测。
   */
  async runJudgerSelftest(judgerId: string): Promise<Judger> {
    return this.client.post(`/judge/judgers/${judgerId}/selftest`)
  }

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
    return this.client.wsURL(`/judge/tasks/${taskId}/progress`)
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
