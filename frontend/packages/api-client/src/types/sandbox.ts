// ===== M2 Sandbox 模块 =====

import type { SnowflakeID } from './common'
import type { WorkloadComponent, WorkloadNetworkRule, WorkloadRoute, WorkloadService } from './workload'
import type {
  ImagePrepullStatus,
  RuntimeImageStatus,
  RuntimeSelftestStatus,
  RuntimeStatus,
  SandboxPhase,
  SandboxStatus,
  SandboxToolKind,
  SandboxToolStatus,
  ToolStatus,
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

export type SandboxChainOperation = 'deploy' | 'transaction' | 'query'

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

export interface SandboxProgressMessage {
  phase: number
  status: number
  stage: string
  message: string
  trace_id?: string
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
  adapter_level: number
  adapter_spec: SandboxAdapterSpec
  capability_impl: string
  plugin_ref: string
  status?: RuntimeStatus
}

export interface SandboxAdapterSpec {
  workspace_dir: string
  volume_domains?: Array<{
    name: string
    mount_path: string
    student_access: 'none' | 'read_only' | 'read_write'
    persistence: 'ephemeral' | 'minio_code'
    snapshot_scope: 'never' | 'always' | 'snapshot_enabled'
  }>
  runtime_container: WorkloadComponent
  infra_sidecars?: WorkloadComponent[]
  services?: WorkloadService[]
  routes?: WorkloadRoute[]
  network_rules?: WorkloadNetworkRule[]
  default_tool_codes?: string[]
  selftest?: Record<string, unknown>
  workspace_ops: {
    read_file: string[]
    write_file: string[]
    list_files: string[]
    pack_tar: string[]
    unpack_tar: string[]
    run_script: string[]
    terminal: string[]
    selftest: string[]
  }
  capability_commands?: Record<'deploy' | 'tx' | 'query' | 'reset', { command: string[]; timeout_seconds: number }>
}

export interface SandboxRuntime extends Omit<SandboxRuntimeRequest, 'status'> {
  id: SnowflakeID
  status: RuntimeStatus
  selftest_status: RuntimeSelftestStatus
  selftest_detail?: Record<string, unknown>
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
  prepull_detail?: Record<string, unknown>
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
  components?: WorkloadComponent[]
  services?: WorkloadService[]
  routes?: WorkloadRoute[]
  network_rules?: WorkloadNetworkRule[]
  command_policy?: {
    allowed_commands: string[]
    default_timeout_seconds: number
    max_timeout_seconds: number
  }
  prepull_command?: string[]
}

export interface SandboxToolDefinition extends SandboxToolRequest {
  id: SnowflakeID
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

export type SandboxQuotaRequest = Omit<SandboxQuota, 'active_sandbox_count'>

export interface SandboxPrepullStatus {
  image_id: SnowflakeID
  prepull_status: ImagePrepullStatus
  desired_nodes: number
  ready_nodes: number
  daemonset: string
  image_count: number
  images: string[]
}

export interface SandboxRuntimeSelftestStatus {
  runtime_id: SnowflakeID
  selftest_status: RuntimeSelftestStatus
  runtime_status: RuntimeStatus
  detail: Record<string, unknown>
}
