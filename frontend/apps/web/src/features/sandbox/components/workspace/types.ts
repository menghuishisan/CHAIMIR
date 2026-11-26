import type {
  SandboxChainRequest, SandboxChainResponse, SandboxCommandToolRunRequest,
  SandboxCommandToolRunResponse, SandboxFileListResponse, SandboxFileReadResponse,
  SandboxFileSaveResponse, SandboxFileWriteRequest, SandboxInstance,
} from '@chaimir/api-client'

/** 先写入当前编辑；archive 为 true 时再归档。失败或仍有未写入编辑时返回 false。 */
export interface SandboxWorkspaceHandle {
  save(archive?: boolean): Promise<boolean>
}

/** 实验使用 M2 租户入口，跨校竞赛注入 M8 授权网关。 */
export interface SandboxWorkspaceApi {
  getInstance(instanceId: string): Promise<SandboxWorkspaceInstance>
  getTerminalWsUrl(instanceId: string, container?: string): string
  getProgressWsUrl(instanceId: string): string
  readFile(instanceId: string, path: string): Promise<SandboxFileReadResponse>
  listFiles(instanceId: string, path?: string): Promise<SandboxFileListResponse>
  writeFile(
    instanceId: string,
    data: SandboxFileWriteRequest
  ): Promise<{ workspace_revision: number }>
  saveFiles(instanceId: string): Promise<Pick<SandboxFileSaveResponse, 'workspace_revision'>>
  runCommandTool(
    instanceId: string,
    toolCode: string,
    data: SandboxCommandToolRunRequest
  ): Promise<SandboxCommandToolRunResponse>
  chainDeploy(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse>
  chainSendTx(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse>
  chainQuery(
    instanceId: string,
    runtimeInstance: string,
    target: string
  ): Promise<SandboxChainResponse>
  getToolProxyUrl(
    instanceId: string,
    toolCode: string,
    proxyPath: string,
    toolOrigin: string
  ): Promise<string>
}

/** 工作台只依赖公开运行时字段,租户编号由具体网关决定是否返回。 */
export type SandboxWorkspaceInstance = Omit<SandboxInstance, 'tenant_id'> & {
  tenant_id?: SandboxInstance['tenant_id']
}
