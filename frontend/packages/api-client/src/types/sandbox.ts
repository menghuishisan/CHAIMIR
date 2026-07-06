// ===== M2 Sandbox 模块 =====

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
  sandbox_id: number
  tenant_id: number
  owner_account_id: number
  runtime_code: string
  runtime_image_version: string
  source_ref: string
  phase: SandboxPhase
  status: SandboxStatus
  tool_access: SandboxToolAccess[]
  resource_usage: SandboxResourceUsage
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
  adapter_level: number
  adapter_spec: Record<string, unknown>
  capability_impl: string
  plugin_ref: string
  status: RuntimeStatus
}

export interface SandboxRuntime extends SandboxRuntimeRequest {
  id: number
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
  id: number
  runtime_id: number
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
  resource_spec: Record<string, unknown>
  status: ToolStatus
}

export interface SandboxToolDefinition extends SandboxToolRequest {
  id: number
}

export interface SandboxQuota {
  tenant_id: number
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
  image_id: number
  prepull_status: ImagePrepullStatus
  desired_nodes: number
  ready_nodes: number
  daemonset: string
  image_count: number
  images: string[]
}

export interface SandboxRuntimeSelftestStatus {
  runtime_id: number
  selftest_status: RuntimeSelftestStatus
  runtime_status: RuntimeStatus
  detail: Record<string, unknown>
}
