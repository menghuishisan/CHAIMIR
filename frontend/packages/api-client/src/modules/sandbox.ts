// Sandbox API：沙箱管理
// 对应后端 M2 模块

import { ApiClient } from '../client'
import type {
  SandboxChainRequest,
  SandboxChainResponse,
  SandboxCommandToolRunRequest,
  SandboxCommandToolRunResponse,
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
    const query = this.buildBrowserTokenQuery(container ? { container } : undefined)
    return `${wsBaseUrl}/sandbox/sandboxes/${instanceId}/terminal${query}`
  }

  /**
   * 获取进度 WebSocket URL
   */
  getProgressWsUrl(instanceId: string): string {
    const baseUrl = this.client['config'].baseURL || ''
    const wsProtocol = baseUrl.startsWith('https') ? 'wss' : 'ws'
    const wsBaseUrl = baseUrl.replace(/^https?/, wsProtocol)
    return `${wsBaseUrl}/sandbox/sandboxes/${instanceId}/progress${this.buildBrowserTokenQuery()}`
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
   * 执行受控命令工具
   */
  async runCommandTool(instanceId: string, toolCode: string, data: SandboxCommandToolRunRequest): Promise<SandboxCommandToolRunResponse> {
    return this.client.post(`/sandbox/sandboxes/${instanceId}/command-tools/${toolCode}/run`, data)
  }

  /**
   * 调用运行时统一链部署能力。
   */
  async chainDeploy(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse> {
    return this.client.post(`/sandbox/sandboxes/${instanceId}/chain/deploy`, data)
  }

  /**
   * 调用运行时统一链交易能力。
   */
  async chainSendTx(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse> {
    return this.client.post(`/sandbox/sandboxes/${instanceId}/chain/tx`, data)
  }

  /**
   * 查询运行时链上状态。
   */
  async chainQuery(instanceId: string, target: string): Promise<SandboxChainResponse> {
    return this.client.get(`/sandbox/sandboxes/${instanceId}/chain/query`, { target })
  }

  /**
   * 获取 Web 工具代理 URL
   */
  getToolProxyUrl(instanceId: string, toolCode: string, proxyPath = ''): string {
    const baseUrl = this.client['config'].baseURL || ''
    const normalizedBase = baseUrl.replace(/\/+$/, '')
    const normalizedPath = proxyPath.replace(/^\/+/, '')
    const encodedTool = encodeURIComponent(toolCode)
    return `${normalizedBase}/sandbox/sandboxes/${instanceId}/tools/${encodedTool}/${normalizedPath}${this.buildBrowserTokenQuery()}`
  }

  /**
   * 构造浏览器原生 WS/iframe 无法设置 Authorization 头时使用的一次性入口 token。
   */
  private buildBrowserTokenQuery(extra?: Record<string, string | undefined>): string {
    const params = new URLSearchParams()
    for (const [key, value] of Object.entries(extra || {})) {
      if (value) {
        params.set(key, value)
      }
    }
    const token = this.client['config'].getToken?.()
    if (token) {
      params.set('token', token)
    }
    const query = params.toString()
    return query ? `?${query}` : ''
  }
}
