// 前端编辑组合声明;编译快照只读,变更须修改声明后重新编译。

import type { SandboxAccessProfile, CompositionSelection } from '../constants/composition'
import type {
  RuntimeAdapterLevel,
  SandboxComponentCategory,
  SandboxToolKind,
} from '../constants/sandbox'

export interface CompositionRuntimeRef {
  instance_code: string
  runtime_code: string
  image_version: string
  params?: Record<string, unknown>
}

export interface CompositionComponentRef {
  code: string
  selection: CompositionSelection
  params?: Record<string, unknown>
}

/** 未声明的组件连接不开放。 */
export interface CompositionLink {
  source_component: string
  source_endpoint: string
  target_component: string
  target_endpoint: string
  protocol: string
  required_at_start: boolean
  access_scope: string
  config_binding: string
}

/** CompositionPreviewRequest 仅声明当前选择与连接,不替使用场景确定访问边界。 */
export interface CompositionPreviewRequest {
  runtimes: CompositionRuntimeRef[]
  workspace_runtime_instance: string
  infra: CompositionComponentRef[]
  tools: CompositionComponentRef[]
  links: CompositionLink[]
}

/** CompositionBindingOption 是服务端已校验的连接目标,前端不自行拼接端点。 */
interface CompositionBindingOption {
  label: string
  link: CompositionLink
}

/** CompositionBindingPreview 中的绑定标识只作稳定键,不作为用户文案。 */
interface CompositionBindingPreview {
  source_component: string
  source_name: string
  config_binding: string
  title: string
  optional: boolean
  multiple: boolean
  options: CompositionBindingOption[]
}

/** CompositionPreviewResponse 是只读候选,不等同于保存、冻结或环境启动成功。 */
export interface CompositionPreviewResponse {
  infra: CompositionComponentRef[]
  bindings: CompositionBindingPreview[]
  links: CompositionLink[]
  complete: boolean
}

export interface CompositionInitAsset {
  name: string
  source_ref: string
  target_runtime_instance: string
  target_domain: string
  expected_sha256: string
}

export interface SandboxCompositionSpec {
  id: string
  runtimes: CompositionRuntimeRef[]
  workspace_runtime_instance: string
  infra?: CompositionComponentRef[]
  tools?: CompositionComponentRef[]
  links?: CompositionLink[]
  access_profile: SandboxAccessProfile
  resource_profile?: Record<string, string>
  network_profile?: Record<string, unknown>
  init_assets?: CompositionInitAsset[]
  init_chain_steps?: CompositionChainStep[]
  init_code_ref?: string
  init_script_ref?: string
}

/** 环境开放前执行的链初始化，纳入组合摘要。 */
export type CompositionChainStep = { runtime_instance: string } & (
  | { op: 'deploy' | 'tx'; payload: Record<string, unknown> }
  | { op: 'query'; target: string }
  | { op: 'reset' }
)

/** 题目可用于不同赛制,访问边界由使用场景确定并由服务端写入。 */
export type ScenarioNeutralCompositionSpec = Omit<SandboxCompositionSpec, 'access_profile'>

/** adapter_spec 由服务端校验并冻结,前端只展示摘要。 */
interface CompiledRuntimeSnapshot {
  instance_code: string
  runtime_id: number
  image_id: number
  code: string
  eco: string
  adapter_level: RuntimeAdapterLevel
  capability_impl: string
  plugin_ref?: string
  adapter_spec: Record<string, unknown>
  image_url: string
  image_version: string
}

interface CompiledComponentSnapshot {
  component_id: number
  category: SandboxComponentCategory
  code: string
  kind: SandboxToolKind
  resource_spec: Record<string, unknown>
}

interface ImageClosureMount {
  name: string
  mount_path: string
}

interface ImageClosureItem {
  category: string
  code: string
  image_url: string
  version: string
  prepull_command: string[]
  prepull_hold: boolean
  ephemeral_mounts?: ImageClosureMount[]
}

export interface SandboxCompositionSnapshot {
  composition_digest: string
  spec: SandboxCompositionSpec
  runtimes: CompiledRuntimeSnapshot[]
  components?: CompiledComponentSnapshot[]
  image_closure: ImageClosureItem[]
}
