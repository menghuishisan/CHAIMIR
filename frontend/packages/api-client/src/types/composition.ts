// ===== 沙箱组合声明与编译快照(实验 / 竞赛 / 判题共用)=====
//
// 后端把「教师声明什么」和「服务端编译出什么」分成了两组字段:
// 教师提交 SandboxCompositionSpec(主运行时 + 组件 + 连接 + 访问边界),
// 服务端编译后冻结成 SandboxCompositionSnapshot(含镜像地址、适配器规格、镜像闭包)。
//
// 前端只提交前者、只读展示后者 —— 快照里的镜像地址、digest、启动命令与安全上下文
// 不得回填成编辑输入(docs/对齐-后端待补齐清单-2026-08-23.md §6.3 / §7.5)。

import type { SandboxAccessProfile, CompositionSelection } from '../constants/composition'
import type {
  RuntimeAdapterLevel,
  SandboxComponentCategory,
  SandboxToolKind,
} from '../constants/sandbox'

/** CompositionRuntimeRef 是主运行时及其明确镜像版本。 */
export interface CompositionRuntimeRef {
  runtime_code: string
  image_version: string
  params?: Record<string, unknown>
}

/** CompositionComponentRef 是工具或基础设施组件引用。 */
export interface CompositionComponentRef {
  code: string
  /** selection 记录这一项是教师显式选的,还是编译器按依赖展开的 */
  selection: CompositionSelection
  params?: Record<string, unknown>
}

/** CompositionLink 是组件之间唯一允许的内网连接声明。 */
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

/** SandboxCompositionSpec 是单个环境的唯一声明来源,由教师提交。 */
export interface SandboxCompositionSpec {
  id: string
  primary_runtime: CompositionRuntimeRef
  infra?: CompositionComponentRef[]
  tools?: CompositionComponentRef[]
  links?: CompositionLink[]
  access_profile: SandboxAccessProfile
  resource_profile?: Record<string, string>
  network_profile?: Record<string, unknown>
  init_code_ref?: string
  init_script_ref?: string
}

/**
 * ScenarioNeutralCompositionSpec 是题库正文里的环境声明。
 * 同一道题既可能进解题赛也可能进对抗赛,访问边界由使用场景决定并由服务端写入,
 * 故题库这一份不固定 access_profile。
 */
export type ScenarioNeutralCompositionSpec = Omit<SandboxCompositionSpec, 'access_profile'>

/**
 * CompiledRuntimeSnapshot 是编译后冻结的运行时事实,只读。
 * adapter_spec 是服务端校验过的声明序列化结果,前端只做摘要展示。
 */
export interface CompiledRuntimeSnapshot {
  runtime_id: number
  image_id: number
  code: string
  eco: string
  adapter_level: RuntimeAdapterLevel
  capability_impl: string
  adapter_spec: Record<string, unknown>
  image_url: string
  image_version: string
}

/** CompiledComponentSnapshot 是编译后冻结的工具/基础设施工作负载规格,只读。 */
export interface CompiledComponentSnapshot {
  component_id: number
  category: SandboxComponentCategory
  code: string
  kind: SandboxToolKind
  resource_spec: Record<string, unknown>
}

/** ImageClosureItem 是已编译快照锁定的镜像地址与版本。 */
export interface ImageClosureMount {
  name: string
  mount_path: string
}

export interface ImageClosureItem {
  category: string
  code: string
  image_url: string
  version: string
  prepull_command: string[]
  prepull_hold: boolean
  ephemeral_mounts?: ImageClosureMount[]
}

/**
 * SandboxCompositionSnapshot 是发布后不可变的组合事实来源。
 * 它由服务端编译产出,前端只读:要改内容只能改 spec 再重新编译,不能改快照。
 */
export interface SandboxCompositionSnapshot {
  composition_digest: string
  spec: SandboxCompositionSpec
  runtime: CompiledRuntimeSnapshot
  components?: CompiledComponentSnapshot[]
  image_closure: ImageClosureItem[]
}
