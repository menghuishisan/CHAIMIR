// Sandbox API：沙箱管理
// 对应后端 M2 模块

import { ApiClient, encodePathSegment } from '../client'
import { API_BASE_PATH } from '../constants'
import type { IdentityApi } from './identity'
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
  SandboxOrchestrationCatalog,
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
  constructor(
    private client: ApiClient,
    private identity: IdentityApi
  ) {}

  /**
   * 查询编排目录：可用运行时（含其可用镜像版本）与可用工具。
   * 教师编排实验环境、竞赛对抗环境和漏洞题预验证环境都用这一条 —— 运行时管理接口
   * 是平台面，会连带下发容器编排清单与镜像地址，编排面不该拿到那些内容。
   */
  async getOrchestrationCatalog(): Promise<SandboxOrchestrationCatalog> {
    return this.client.get('/sandbox/catalog')
  }

  /**
   * 查询平台运行时列表。
   */
  async listRuntimes(): Promise<SandboxRuntime[]> {
    return this.client.get('/sandbox/runtimes')
  }

  /**
   * getRuntime 读取单条链运行时。
   * 运行时详情页用它做深链首屏读取 —— 不再拉全量列表在浏览器里筛单条。
   * 返回对象同时带 `status`(是否对学校开放)与 `selftest_status`(接入自检结果):
   * 两者是独立的事,`adapter_spec.disabled_reason` 说明尚未完成适配的原因。
   */
  async getRuntime(runtimeId: string): Promise<SandboxRuntime> {
    return this.client.get(`/sandbox/runtimes/${encodePathSegment(runtimeId)}`)
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
    return this.client.patch(`/sandbox/runtimes/${encodePathSegment(runtimeId)}`, data)
  }

  /**
   * 触发运行时接入即测。
   */
  async runRuntimeSelftest(runtimeId: string): Promise<SandboxRuntimeSelftestStatus> {
    return this.client.post(`/sandbox/runtimes/${encodePathSegment(runtimeId)}/selftest`)
  }

  /**
   * 查询运行时接入即测结果。
   */
  async getRuntimeSelftest(runtimeId: string): Promise<SandboxRuntimeSelftestStatus> {
    return this.client.get(`/sandbox/runtimes/${encodePathSegment(runtimeId)}/selftest`)
  }

  /**
   * 为运行时登记镜像版本。
   */
  async registerRuntimeImage(
    runtimeId: string,
    data: SandboxRuntimeImageRequest
  ): Promise<SandboxRuntimeImage> {
    return this.client.post(`/sandbox/runtimes/${encodePathSegment(runtimeId)}/images`, data)
  }

  /**
   * 查询运行时镜像版本列表。
   */
  async listRuntimeImages(runtimeId: string): Promise<SandboxRuntimeImage[]> {
    return this.client.get(`/sandbox/runtimes/${encodePathSegment(runtimeId)}/images`)
  }

  /**
   * 停用运行时镜像版本。
   */
  async disableRuntimeImage(runtimeId: string, imageId: string): Promise<SandboxRuntimeImage> {
    return this.client.delete(
      `/sandbox/runtimes/${encodePathSegment(runtimeId)}/images/${encodePathSegment(imageId)}`
    )
  }

  /**
   * 触发运行时镜像预拉取。
   */
  async prepullRuntimeImage(
    runtimeId: string,
    imageId: string,
    compositionDigest: string
  ): Promise<SandboxPrepullStatus> {
    return this.client.post(
      `/sandbox/runtimes/${encodePathSegment(runtimeId)}/images/${encodePathSegment(imageId)}/prepull`,
      { composition_digest: compositionDigest }
    )
  }

  /**
   * 查询镜像预拉取闭环状态。
   */
  async getRuntimeImagePrepull(
    runtimeId: string,
    imageId: string,
    compositionDigest: string
  ): Promise<SandboxPrepullStatus> {
    return this.client.get(
      `/sandbox/runtimes/${encodePathSegment(runtimeId)}/images/${encodePathSegment(imageId)}/prepull`,
      { composition_digest: compositionDigest }
    )
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
   * 查询沙箱配额与活跃数量。学校管理员读本租户（服务端忽略 tenant_id），平台管理员必须传 tenant_id 指定目标租户。
   */
  async getQuota(params?: { tenant_id?: string }): Promise<SandboxQuota> {
    return this.client.get('/sandbox/quota', params)
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
    return this.client.get(`/sandbox/sandboxes/${encodePathSegment(instanceId)}`)
  }

  /**
   * 获取终端 WebSocket URL
   */
  getTerminalWsUrl(instanceId: string, container?: string): string {
    return this.client.wsURL(
      `/sandbox/sandboxes/${encodePathSegment(instanceId)}/terminal`,
      container ? { container } : undefined
    )
  }

  /**
   * 获取进度 WebSocket URL
   */
  getProgressWsUrl(instanceId: string): string {
    return this.client.wsURL(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/progress`)
  }

  /**
   * 读取工作区文件
   */
  async readFile(instanceId: string, path: string): Promise<SandboxFileReadResponse> {
    return this.client.get(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/files`, { path })
  }

  /**
   * 列出工作区目录
   */
  async listFiles(instanceId: string, path = '.'): Promise<SandboxFileListResponse> {
    return this.client.get(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/files`, {
      mode: 'list',
      path,
    })
  }

  /**
   * 写入工作区文件
   */
  async writeFile(
    instanceId: string,
    data: SandboxFileWriteRequest
  ): Promise<{ workspace_revision: number }> {
    return this.client.put(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/files`, data)
  }

  /**
   * 立即持久化工作区
   */
  async saveFiles(instanceId: string): Promise<SandboxFileSaveResponse> {
    return this.client.post(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/files/save`)
  }

  /**
   * 执行受控命令工具
   */
  async runCommandTool(
    instanceId: string,
    toolCode: string,
    data: SandboxCommandToolRunRequest
  ): Promise<SandboxCommandToolRunResponse> {
    return this.client.post(
      `/sandbox/sandboxes/${encodePathSegment(instanceId)}/command-tools/${encodePathSegment(toolCode)}/run`,
      data
    )
  }

  /**
   * 调用运行时统一链部署能力。
   */
  async chainDeploy(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse> {
    return this.client.post(
      `/sandbox/sandboxes/${encodePathSegment(instanceId)}/chain/deploy`,
      data
    )
  }

  /**
   * 调用运行时统一链交易能力。
   */
  async chainSendTx(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse> {
    return this.client.post(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/chain/tx`, data)
  }

  /**
   * 查询运行时链上状态。
   */
  async chainQuery(instanceId: string, runtimeInstance: string, target: string): Promise<SandboxChainResponse> {
    return this.client.get(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/chain/query`, {
      runtime_instance: runtimeInstance,
      target,
    })
  }

  /**
   * 获取 Web 工具代理 URL
   */
  async getToolProxyUrl(
    instanceId: string,
    toolCode: string,
    proxyPath = '',
    toolOrigin: string
  ): Promise<string> {
    const normalizedPath = normalizeProxyPath(proxyPath)
    const encodedTool = encodePathSegment(toolCode)
    const pathPrefix = `${API_BASE_PATH}/sandbox/sandboxes/${encodePathSegment(instanceId)}/tools/${encodedTool}`
    const path = `${API_BASE_PATH}/sandbox/sandboxes/${encodePathSegment(instanceId)}/tools/${encodedTool}/${normalizedPath}`
    const { ticket } = await this.identity.issueBrowserAccessTicket(pathPrefix)
    return this.client.browserURLAtOrigin(toolOrigin, path, { ticket })
  }
}

/** 将代理下游路径限制为不含查询、片段或路径逃逸的编码路径。 */
function normalizeProxyPath(proxyPath: string): string {
  const normalized = proxyPath.trim().replace(/^\/+/, '')
  if (/[?#\\]/.test(normalized) || hasControlCharacter(normalized)) {
    throw new Error('工具代理路径包含不允许的字符')
  }
  const segments = normalized
    .split('/')
    .filter(Boolean)
    .map((segment) => {
      let decoded: string
      try {
        decoded = decodeURIComponent(segment)
      } catch {
        throw new Error('工具代理路径包含无效编码')
      }
      if (
        decoded === '.' ||
        decoded === '..' ||
        /[\\/?#]/.test(decoded) ||
        hasControlCharacter(decoded)
      ) {
        throw new Error('工具代理路径包含不允许的路径段')
      }
      return encodeURIComponent(decoded)
    })
  if (segments.length === 0) {
    return ''
  }
  return segments.join('/')
}

/** 检查路径中不可进入 URL 的 ASCII 控制字符。 */
function hasControlCharacter(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index)
    if (code <= 0x1f || code === 0x7f) return true
  }
  return false
}
