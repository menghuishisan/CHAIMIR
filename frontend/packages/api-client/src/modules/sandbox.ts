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
  SandboxPrepullStatus,
  SandboxQuota,
  SandboxRuntime,
  SandboxRuntimeImage,
  SandboxRuntimeImageRequest,
  SandboxRuntimeRequest,
  SandboxRuntimeSelftestStatus,
  SandboxToolDefinition,
  SandboxToolRequest,
} from '../types/sandbox'

/**
 * SandboxApi 封装后端 M2 沙箱、运行时、文件、工具和链交互接口。
 */
export class SandboxApi {
  /**
   * constructor 注入统一 API 客户端，避免沙箱模块自行拼接鉴权和错误协议。
   */
  constructor(private client: ApiClient) {}

  /**
   * 查询平台运行时列表。
   */
  async listRuntimes(): Promise<SandboxRuntime[]> {
    return this.client.get('/sandbox/runtimes')
  }

  /**
   * 注册新的链运行时。
   */
  async registerRuntime(data: SandboxRuntimeRequest): Promise<SandboxRuntime> {
    return this.client.post('/sandbox/runtimes', data)
  }

  /**
   * 更新链运行时声明。
   */
  async updateRuntime(runtimeId: string, data: SandboxRuntimeRequest): Promise<SandboxRuntime> {
    return this.client.patch(`/sandbox/runtimes/${runtimeId}`, data)
  }

  /**
   * 触发运行时接入即测。
   */
  async runRuntimeSelftest(runtimeId: string): Promise<SandboxRuntimeSelftestStatus> {
    return this.client.post(`/sandbox/runtimes/${runtimeId}/selftest`)
  }

  /**
   * 查询运行时接入即测结果。
   */
  async getRuntimeSelftest(runtimeId: string): Promise<SandboxRuntimeSelftestStatus> {
    return this.client.get(`/sandbox/runtimes/${runtimeId}/selftest`)
  }

  /**
   * 为运行时登记镜像版本。
   */
  async registerRuntimeImage(runtimeId: string, data: SandboxRuntimeImageRequest): Promise<SandboxRuntimeImage> {
    return this.client.post(`/sandbox/runtimes/${runtimeId}/images`, data)
  }

  /**
   * 查询运行时镜像版本列表。
   */
  async listRuntimeImages(runtimeId: string): Promise<SandboxRuntimeImage[]> {
    return this.client.get(`/sandbox/runtimes/${runtimeId}/images`)
  }

  /**
   * 停用运行时镜像版本。
   */
  async disableRuntimeImage(runtimeId: string, imageId: string): Promise<SandboxRuntimeImage> {
    return this.client.delete(`/sandbox/runtimes/${runtimeId}/images/${imageId}`)
  }

  /**
   * 触发运行时镜像预拉取。
   */
  async prepullRuntimeImage(runtimeId: string, imageId: string): Promise<SandboxPrepullStatus> {
    return this.client.post(`/sandbox/runtimes/${runtimeId}/images/${imageId}/prepull`)
  }

  /**
   * 查询镜像预拉取闭环状态。
   */
  async getRuntimeImagePrepull(runtimeId: string, imageId: string): Promise<SandboxPrepullStatus> {
    return this.client.get(`/sandbox/runtimes/${runtimeId}/images/${imageId}/prepull`)
  }

  /**
   * 查询平台工具定义列表。
   */
  async listTools(): Promise<SandboxToolDefinition[]> {
    return this.client.get('/sandbox/tools')
  }

  /**
   * 注册沙箱工具定义。
   */
  async registerTool(data: SandboxToolRequest): Promise<SandboxToolDefinition> {
    return this.client.post('/sandbox/tools', data)
  }

  /**
   * 查询当前租户沙箱配额与活跃数量。
   */
  async getQuota(): Promise<SandboxQuota> {
    return this.client.get('/sandbox/quota')
  }

  /**
   * 更新租户沙箱配额，平台管理员可指定 tenant_id，学校管理员只更新本租户。
   */
  async updateQuota(data: SandboxQuota): Promise<SandboxQuota> {
    return this.client.patch('/sandbox/quota', data)
  }

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
    return this.client.wsURL(`/sandbox/sandboxes/${instanceId}/terminal`, container ? { container } : undefined)
  }

  /**
   * 获取进度 WebSocket URL
   */
  getProgressWsUrl(instanceId: string): string {
    return this.client.wsURL(`/sandbox/sandboxes/${instanceId}/progress`)
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
    const normalizedPath = proxyPath.replace(/^\/+/, '')
    const encodedTool = encodeURIComponent(toolCode)
    return this.client.browserURL(`/sandbox/sandboxes/${instanceId}/tools/${encodedTool}/${normalizedPath}`)
  }
}
