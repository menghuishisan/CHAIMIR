// Sandbox API：沙箱管理
// 对应后端 M2 模块

import { ApiClient } from '../client'
import type { SandboxInstance, CreateSandboxRequest } from '../types'

export class SandboxApi {
  constructor(private client: ApiClient) {}

  /**
   * 创建沙箱实例
   */
  async createInstance(data: CreateSandboxRequest): Promise<SandboxInstance> {
    return this.client.post('/sandbox/instances', data)
  }

  /**
   * 获取沙箱实例详情
   */
  async getInstance(instanceId: string): Promise<SandboxInstance> {
    return this.client.get(`/sandbox/instances/${instanceId}`)
  }

  /**
   * 获取沙箱列表
   */
  async getInstances(params?: {
    status?: number
    page?: number
    size?: number
  }): Promise<SandboxInstance[]> {
    return this.client.get('/sandbox/instances', params)
  }

  /**
   * 销毁沙箱实例
   */
  async destroyInstance(instanceId: string): Promise<void> {
    return this.client.delete(`/sandbox/instances/${instanceId}`)
  }

  /**
   * 获取终端 WebSocket URL
   */
  getTerminalWsUrl(instanceId: string): string {
    const baseUrl = this.client['config'].baseURL || ''
    const wsProtocol = baseUrl.startsWith('https') ? 'wss' : 'ws'
    const wsBaseUrl = baseUrl.replace(/^https?/, wsProtocol)
    return `${wsBaseUrl}/sandbox/instances/${instanceId}/terminal`
  }
}
