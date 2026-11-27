import { ApiClient, encodePathSegment } from '../client'
import { API_BASE_PATH } from '../constants'
import type { IdentityApi } from './identity'
import type { CompositionPreviewRequest, CompositionPreviewResponse } from '../types/composition'
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

/** M2 沙箱、运行时、文件、工具与链操作客户端。 */
export class SandboxApi {
  constructor(
    private client: ApiClient,
    private identity: IdentityApi
  ) {}

  /** 编排目录不返回平台管理面的适配器、镜像地址或命令。 */
  async getOrchestrationCatalog(): Promise<SandboxOrchestrationCatalog> {
    return this.client.get('/sandbox/catalog')
  }

  async previewComposition(request: CompositionPreviewRequest): Promise<CompositionPreviewResponse> {
    return this.client.post('/sandbox/catalog/composition-preview', request)
  }

  async listRuntimes(): Promise<SandboxRuntime[]> {
    return this.client.get('/sandbox/runtimes')
  }

  /** status 与 selftest_status 分别表示开放状态和自检结果。 */
  async getRuntime(runtimeId: string): Promise<SandboxRuntime> {
    return this.client.get(`/sandbox/runtimes/${encodePathSegment(runtimeId)}`)
  }

  async registerRuntime(data: SandboxRuntimeRequest): Promise<SandboxRuntime> {
    return this.client.post('/sandbox/runtimes', data)
  }

  async updateRuntime(runtimeId: string, data: SandboxRuntimeRequest): Promise<SandboxRuntime> {
    return this.client.patch(`/sandbox/runtimes/${encodePathSegment(runtimeId)}`, data)
  }

  /** 长任务由 getRuntimeSelftest 观察，离页不取消服务端执行。 */
  async runRuntimeSelftest(runtimeId: string): Promise<SandboxRuntimeSelftestStatus> {
    return this.client.postWithoutClientTimeout(`/sandbox/runtimes/${encodePathSegment(runtimeId)}/selftest`)
  }

  async getRuntimeSelftest(runtimeId: string): Promise<SandboxRuntimeSelftestStatus> {
    return this.client.get(`/sandbox/runtimes/${encodePathSegment(runtimeId)}/selftest`)
  }

  async registerRuntimeImage(
    runtimeId: string,
    data: SandboxRuntimeImageRequest
  ): Promise<SandboxRuntimeImage> {
    return this.client.post(`/sandbox/runtimes/${encodePathSegment(runtimeId)}/images`, data)
  }

  async listRuntimeImages(runtimeId: string): Promise<SandboxRuntimeImage[]> {
    return this.client.get(`/sandbox/runtimes/${encodePathSegment(runtimeId)}/images`)
  }

  async disableRuntimeImage(runtimeId: string, imageId: string): Promise<SandboxRuntimeImage> {
    return this.client.delete(
      `/sandbox/runtimes/${encodePathSegment(runtimeId)}/images/${encodePathSegment(imageId)}`
    )
  }

  /** 按已冻结摘要准备完整闭包，通过 getRuntimeImagePrepull 查询进度。 */
  async prepullRuntimeImage(
    runtimeId: string,
    imageId: string,
    compositionDigest: string
  ): Promise<SandboxPrepullStatus> {
    return this.client.postWithoutClientTimeout(
      `/sandbox/runtimes/${encodePathSegment(runtimeId)}/images/${encodePathSegment(imageId)}/prepull`,
      { composition_digest: compositionDigest }
    )
  }

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

  async listTools(): Promise<SandboxToolDefinition[]> {
    return this.client.get('/sandbox/tools')
  }

  async registerTool(data: SandboxToolRequest): Promise<SandboxToolDefinition> {
    return this.client.post('/sandbox/tools', data)
  }

  /** 校管读取会话租户；平台管理员必须指定 tenant_id。 */
  async getQuota(params?: { tenant_id?: string }): Promise<SandboxQuota> {
    return this.client.get('/sandbox/quota', params)
  }

  async updateQuota(data: SandboxQuota): Promise<SandboxQuota> {
    return this.client.patch('/sandbox/quota', data)
  }

  async getInstance(instanceId: string): Promise<SandboxInstance> {
    return this.client.get(`/sandbox/sandboxes/${encodePathSegment(instanceId)}`)
  }

  getTerminalWsUrl(instanceId: string, container?: string): string {
    return this.client.wsURL(
      `/sandbox/sandboxes/${encodePathSegment(instanceId)}/terminal`,
      container ? { container } : undefined
    )
  }

  getProgressWsUrl(instanceId: string): string {
    return this.client.wsURL(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/progress`)
  }

  async readFile(instanceId: string, path: string): Promise<SandboxFileReadResponse> {
    return this.client.get(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/files`, { path })
  }

  async listFiles(instanceId: string, path = '.'): Promise<SandboxFileListResponse> {
    return this.client.get(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/files`, {
      mode: 'list',
      path,
    })
  }

  async writeFile(
    instanceId: string,
    data: SandboxFileWriteRequest
  ): Promise<{ workspace_revision: number }> {
    return this.client.put(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/files`, data)
  }

  async saveFiles(instanceId: string): Promise<SandboxFileSaveResponse> {
    return this.client.post(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/files/save`)
  }

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

  async chainDeploy(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse> {
    return this.client.post(
      `/sandbox/sandboxes/${encodePathSegment(instanceId)}/chain/deploy`,
      data
    )
  }

  async chainSendTx(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse> {
    return this.client.post(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/chain/tx`, data)
  }

  async chainQuery(instanceId: string, runtimeInstance: string, target: string): Promise<SandboxChainResponse> {
    return this.client.get(`/sandbox/sandboxes/${encodePathSegment(instanceId)}/chain/query`, {
      runtime_instance: runtimeInstance,
      target,
    })
  }

  /** 换取绑定路径的短时票据，access token 不进入工具 URL。 */
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

/** 只接受无查询、片段、控制字符或路径逃逸的代理路径。 */
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

function hasControlCharacter(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index)
    if (code <= 0x1f || code === 0x7f) return true
  }
  return false
}
