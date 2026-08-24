// ===== M2 Sandbox 模块 =====

import type { SnowflakeID } from './common'
import type {
  ImagePrepullStatus,
  RuntimeImageStatus,
  RuntimeAdapterLevel,
  RuntimeSelftestStatus,
  RuntimeStatus,
  SandboxComponentCategory,
  SandboxPhase,
  SandboxStatus,
  SandboxToolKind,
  SandboxToolStatus,
  ToolStatus,
  SandboxChainOperation,
} from '../constants/sandbox'

export interface SandboxInstance {
  sandbox_id: SnowflakeID
  tenant_id: SnowflakeID
  owner_account_id: SnowflakeID
  runtime_code: string
  runtime_image_version: string
  runtime_instances: string[]
  source_ref: string
  phase: SandboxPhase
  status: SandboxStatus
  tool_access: SandboxToolAccess[]
  capabilities: SandboxCapabilities
  resource_usage: SandboxResourceUsage
  workspace_revision: number
}

export interface SandboxCapabilities {
  file_workspace: boolean
  terminal: boolean
  command_tools: boolean
  chain_operations: SandboxChainOperation[]
}

export interface SandboxToolAccess {
  tool_code: string
  kind: SandboxToolKind
  endpoint: string
  status: SandboxToolStatus
}

export interface SandboxCommandToolRunRequest {
  command: string[]
  stdin_base64?: string
  timeout_sec?: number
}

export interface SandboxCommandToolRunResponse {
  stdout_base64: string
  stderr_base64: string
  exit_code: number
}

export interface SandboxResourceUsage {
  cpu_usage_milli: number
  memory_usage_mib: number
  cpu_request_milli: number
  cpu_limit_milli: number
  memory_request_mib: number
  memory_limit_mib: number
  storage_bytes: number
}

export interface SandboxFileReadResponse {
  relative_path: string
  content_base64: string
  content_sha256: string
  content_size: number
  workspace_revision: number
}

export interface SandboxFileEntry {
  name: string
  relative_path: string
  is_dir: boolean
  size: number
}

export interface SandboxFileListResponse {
  relative_path: string
  entries: SandboxFileEntry[]
}

export interface SandboxFileWriteRequest {
  relative_path: string
  content_base64: string
  expected_revision: number
}

export interface SandboxFileSaveResponse {
  code_storage_key: string
  code_hash: string
  workspace_revision: number
}

export interface SandboxChainRequest {
  runtime_instance: string
  payload: Record<string, unknown>
}

export type SandboxChainResponse = Record<string, unknown>

export interface SandboxRuntimeRequest {
  code: string
  name: string
  eco: string
  adapter_level: RuntimeAdapterLevel
  adapter_spec: SandboxAdapterSpec
  capability_impl: string
  plugin_ref: string
  status?: RuntimeStatus
}

/** SandboxAdapterSpec 是管理员维护的动态声明,内部编排/K8s 结构不进入公开 SDK 类型。 */
export type SandboxAdapterSpec = Record<string, unknown>

export interface SandboxRuntime extends Omit<SandboxRuntimeRequest, 'status'> {
  id: SnowflakeID
  status: RuntimeStatus
  selftest_status: RuntimeSelftestStatus
}

export interface SandboxRuntimeImageRequest {
  image_url: string
  version: string
  digest: string
  genesis_baked: boolean
}

/**
 * SandboxRuntimeImage 是镜像版本的读取投影。
 * 它刻意不继承登记请求:digest 只在登记时提交(服务端按镜像地址二次校验),
 * 读取时不回显 —— 前端也就无法把它当编辑输入再送回去(§7.5)。
 */
export interface SandboxRuntimeImage {
  id: SnowflakeID
  runtime_id: SnowflakeID
  image_url: string
  version: string
  status: RuntimeImageStatus
  genesis_baked: boolean
}

export interface SandboxToolRequest {
  code: string
  name: string
  kind: SandboxToolKind
  eco_tags: string[]
  resource_spec: SandboxToolResourceSpec
  status: ToolStatus
}

export interface SandboxToolResourceSpec {
  /** disabled_reason 是服务端给出的用户向不可用原因,只读展示,不由前端编造 */
  disabled_reason?: string
  builtin_endpoint?: string
  components?: unknown[]
  services?: unknown[]
  routes?: unknown[]
  network_rules?: unknown[]
  command_policy?: {
    allowed_argv: string[][]
    default_timeout_seconds: number
    max_timeout_seconds: number
  }
  prepull_command?: string[]
  bindings?: SandboxComponentBinding[]
  capabilities?: SandboxComponentCapabilities
}

/** SandboxComponentBinding 描述组件按能力和端点声明的连接角色。 */
export interface SandboxComponentBinding {
  name: string
  capability: string
  endpoint: string
  protocol: 'TCP' | 'HTTP' | 'HTTPS' | 'WS' | 'WSS' | 'GRPC'
  required_at_start: boolean
  config_binding: `env:${string}`
}

/** SandboxComponentCapabilities 是组件能力声明,供服务端编译器校验组合;前端只读展示。 */
export interface SandboxComponentCapabilities {
  provides?: string[] | null
  requires?: string[] | null
  conflicts?: string[] | null
  cardinality?: string
  placement?: string
  config_schema?: Record<string, unknown> | null
  student_access?: string
}

export interface SandboxToolDefinition extends SandboxToolRequest {
  id: SnowflakeID
  /** category 区分教师可见工具(tool)与只参与编排的基础设施组件(infra) */
  category: SandboxComponentCategory
}

/**
 * SandboxOrchestrationCatalog 是编排目录响应:业务模块选运行时与组件只需这些字段。
 * 它刻意不是 SandboxRuntime / SandboxToolDefinition 的子集别名 —— 适配器清单、镜像地址、
 * 完整 argv 白名单与自检详情属平台运维面,后端也不下发,故编排面按独立类型对接。
 *
 * 目录只出真正可调度的项:运行时必须 status=available 且自检通过,被引用的镜像必须内置创世,
 * 且已发布组合的完整镜像闭包必须有成功的组合预拉取证明。服务端在 SQL 层过滤这些条件,
 * 故前端不再按状态二次筛(避免两边口径不一致)。
 * 基础设施单独成组,不能混进学生工具入口(§7.2)。
 */
export interface SandboxOrchestrationCatalog {
  runtimes: SandboxCatalogRuntime[]
  infra: SandboxCatalogTool[]
  tools: SandboxCatalogTool[]
}

export interface SandboxCatalogRuntime {
  code: string
  name: string
  eco: string
  images: SandboxCatalogRuntimeImage[]
  capabilities: SandboxComponentCapabilities
}

export interface SandboxCatalogRuntimeImage {
  version: string
}

export interface SandboxCatalogTool {
  code: string
  name: string
  kind: SandboxToolKind
  capabilities: SandboxComponentCapabilities
}

export interface SandboxQuota {
  tenant_id: SnowflakeID
  active_sandbox_count?: number
  max_concurrent_sandbox: number
  max_cpu: number
  max_memory_mb: number
  idle_timeout_min: number
  max_lifetime_min: number
  max_keepalive_min: number
  max_snapshot_retention_min: number
}

export interface SandboxPrepullStatus {
  image_id: SnowflakeID
  composition_digest: string
  prepull_status: ImagePrepullStatus
  desired_nodes: number
  ready_nodes: number
  image_count: number
}

export interface SandboxRuntimeSelftestStatus {
  runtime_id: SnowflakeID
  selftest_status: RuntimeSelftestStatus
  runtime_status: RuntimeStatus
  detail: SandboxRuntimeSelftestDetail
}

/**
 * SandboxRuntimeSelftestDetail 是可展示的自检摘要,不含命名空间或编排明细。
 * reason 由服务端给出,可能是用户向说明也可能是稳定原因码,前端按码表转成用户向文案。
 */
export interface SandboxRuntimeSelftestDetail {
  result?: string
  stage?: string
  reason?: string
  trace_id?: string
}
