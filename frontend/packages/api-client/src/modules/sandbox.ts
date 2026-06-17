// Sandbox API：沙箱管理
// 对应后端 M2 模块

import { ApiClient } from '../client'
import type {
  SandboxFileListResponse,
  SandboxFileReadResponse,
  SandboxFileSaveResponse,
  SandboxFileWriteRequest,
  SandboxInstance,
} from '../types'

export class SandboxApi {
  constructor(private client: ApiClient) {}

  /**
   * 获取沙箱实例详情
   */
  async getInstance(instanceId: string): Promise<SandboxInstance> {
    return this.client.get(`/sandbox/sandboxes/${instanceId}`)
  }

  /**
   * 获取终端 WebSocket URL
   */
  getTerminalWsUrl(instanceId: string, container?: string): string {
    const baseUrl = this.client['config'].baseURL || ''
    const wsProtocol = baseUrl.startsWith('https') ? 'wss' : 'ws'
    const wsBaseUrl = baseUrl.replace(/^https?/, wsProtocol)
    const query = container ? `?container=${encodeURIComponent(container)}` : ''
    return `${wsBaseUrl}/sandbox/sandboxes/${instanceId}/terminal${query}`
  }

  /**
   * 获取进度 WebSocket URL
   */
  getProgressWsUrl(instanceId: string): string {
    const baseUrl = this.client['config'].baseURL || ''
    const wsProtocol = baseUrl.startsWith('https') ? 'wss' : 'ws'
    const wsBaseUrl = baseUrl.replace(/^https?/, wsProtocol)
    return `${wsBaseUrl}/sandbox/sandboxes/${instanceId}/progress`
  }

  /**
   * 读取工作区文件
   */
  async readFile(instanceId: string, path: string): Promise<SandboxFileReadResponse> {
    return this.client.get(`/sandbox/sandboxes/${instanceId}/files`, { path })
  }

  /**
   * 列出工作区目录
   */
  async listFiles(instanceId: string, path = '.'): Promise<SandboxFileListResponse> {
    return this.client.get(`/sandbox/sandboxes/${instanceId}/files`, { mode: 'list', path })
  }

  /**
   * 写入工作区文件
   */
  async writeFile(instanceId: string, data: SandboxFileWriteRequest): Promise<Record<string, never>> {
    return this.client.put(`/sandbox/sandboxes/${instanceId}/files`, data)
  }

  /**
   * 立即持久化工作区
   */
  async saveFiles(instanceId: string): Promise<SandboxFileSaveResponse> {
    return this.client.post(`/sandbox/sandboxes/${instanceId}/files/save`)
  }

  /**
   * 获取 Web 工具代理 URL
   */
  getToolProxyUrl(instanceId: string, toolCode: string, proxyPath = ''): string {
    const baseUrl = this.client['config'].baseURL || ''
    const normalizedBase = baseUrl.replace(/\/+$/, '')
    const normalizedPath = proxyPath.replace(/^\/+/, '')
    return `${normalizedBase}/sandbox/sandboxes/${instanceId}/tools/${toolCode}/${normalizedPath}`
  }
}
