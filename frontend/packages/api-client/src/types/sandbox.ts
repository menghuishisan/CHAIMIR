// ===== M2 Sandbox 模块 =====

import type { SnowflakeID } from './common'
import type {
  ImagePrepullStatus,
  RuntimeImageStatus,
  RuntimeAdapterLevel,
  RuntimeSelftestStatus,
  RuntimeStatus,
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
  source_ref: string
  phase: SandboxPhase
  status: SandboxStatus
  tool_access: SandboxToolAccess[]
  capabilities: SandboxCapabilities
  resource_usage: SandboxResourceUsage
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
}

export interface SandboxFileSaveResponse {
  code_storage_key: string
  code_hash: string
}

export interface SandboxChainRequest {
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
  is_default: boolean
}

export interface SandboxRuntimeImage extends SandboxRuntimeImageRequest {
  id: SnowflakeID
  runtime_id: SnowflakeID
  status: RuntimeImageStatus
  prepulled: boolean
  prepull_status: ImagePrepullStatus
  prepulled_at?: string
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
}

export interface SandboxToolDefinition extends SandboxToolRequest {
  id: SnowflakeID
}

/**
 * SandboxOrchestrationCatalog 是编排目录响应:业务模块选运行时与工具只需这些字段。
 * 它刻意不是 SandboxRuntime / SandboxToolDefinition 的子集别名 —— 适配器清单、镜像地址、
 * 完整 argv 白名单与自检详情属平台运维面,后端也不下发,故编排面按独立类型对接。
 */
export interface SandboxOrchestrationCatalog {
  runtimes: SandboxCatalogRuntime[]
  tools: SandboxCatalogTool[]
}

export interface SandboxCatalogRuntime {
  code: string
  name: string
  eco: string
  images: SandboxCatalogRuntimeImage[]
  tool_codes: string[]
}

export interface SandboxCatalogRuntimeImage {
  version: string
  is_default: boolean
}

export interface SandboxCatalogTool {
  code: string
  name: string
  kind: SandboxToolKind
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

/** SandboxRuntimeSelftestDetail 是可展示的自检摘要,不含命名空间或编排明细。 */
export interface SandboxRuntimeSelftestDetail {
  result?: string
  stage?: string
  trace_id?: string
}
