// ===== M7 Experiment 模块 =====

import type { SnowflakeID } from './common'
import type {
  CompositionComponentRef,
  CompositionLink,
  CompositionRuntimeRef,
} from './composition'
import type { SandboxAccessProfile } from '../constants/composition'
import type { JudgeSandboxMode } from '../constants/judge'
import type {
  ExperimentCollabMode,
  ExperimentInstanceStatus,
  ExperimentReportStatus,
  ExperimentStageStatus,
  ExperimentStatus,
  ExperimentValidationLevel,
} from '../constants/experiment'
import type { SandboxToolKind, SandboxToolStatus } from '../constants/sandbox'
import type { SimCompute } from '../constants/sim'

export interface Experiment {
  id: SnowflakeID
  course_id?: SnowflakeID
  author_id: SnowflakeID
  template_ref?: string
  template_version?: string
  name: string
  description: string
  components: ComponentConfig
  collab_mode: ExperimentCollabMode
  group_config: GroupConfig
  require_report: boolean
  wizard_step: number
  status: ExperimentStatus
  created_at: string
  updated_at: string
}

export interface StudentExperiment {
  id: SnowflakeID
  course_id?: SnowflakeID
  name: string
  description: string
  components: StudentComponentConfig
  collab_mode: ExperimentCollabMode
  group_config: GroupConfig
  my_group_id?: SnowflakeID
  require_report: boolean
  status: ExperimentStatus
  created_at: string
  updated_at: string
}

export interface StudentComponentConfig {
  envs: StudentEnvComponent[]
  sims: StudentSimComponent[]
  checkpoints: StudentCheckpointConfig[]
  stages: StudentStageConfig[]
}

/**
 * StudentEnvComponent 是学生可见的环境摘要:能看到用哪个运行时、挂了哪些工具与连接,
 * 但看不到镜像地址、适配器规格与启动命令 —— 那些只在服务端快照里。
 */
export interface StudentEnvComponent {
  id: string
  primary_runtime: CompositionRuntimeRef
  infra: CompositionComponentRef[]
  tools: CompositionComponentRef[]
  links: CompositionLink[]
  access_profile: SandboxAccessProfile
  composition_digest: string
}

export interface StudentSimComponent {
  id: string
  package_code: string
  version: string
}

export interface StudentCheckpointConfig {
  id: string
  score: number
  mode?: JudgeSandboxMode
}

export interface StudentStageConfig {
  stage: number
  title: string
  description?: string
  components: StageComponents
  unlock_condition?: UnlockCondition
}

/**
 * ComponentConfig 是教师读取实验定义时的安全投影。
 * 它与提交用的 ComponentConfigRequest 只差 envs 一项:读取多带一个服务端算出的
 * composition_digest,而那一项不能回填成提交字段(§6.3 高级视图只读)。
 */
export interface ComponentConfig {
  envs: TeacherEnvComponent[]
  sims: SimComponent[]
  checkpoints: CheckpointConfig[]
  stages: StageConfig[]
}

/** ComponentConfigRequest 是教师提交的声明,只含 runtime/tools/infra/links/参数。 */
export interface ComponentConfigRequest {
  envs: EnvComponentRequest[]
  sims: SimComponent[]
  checkpoints: CheckpointConfig[]
  stages: StageConfig[]
}

/**
 * EnvComponentRequest 是教师声明的一个环境:主运行时 + 组件 + 连接 + 访问边界 + 生命周期。
 * 镜像地址、digest、启动命令与安全上下文都由服务端编译产出,不在这里提交(§7.5)。
 */
export interface EnvComponentRequest {
  id: string
  primary_runtime: CompositionRuntimeRef
  infra: CompositionComponentRef[]
  tools: CompositionComponentRef[]
  links: CompositionLink[]
  access_profile: SandboxAccessProfile
  resource_profile?: Record<string, string>
  network_profile?: Record<string, unknown>
  init_code_ref?: string
  init_script_ref?: string
  keep_alive: boolean
  snapshot_enabled: boolean
  keep_alive_minutes: number
  snapshot_retention_minutes: number
}

/** TeacherEnvComponent 是读取时的环境投影:声明字段 + 服务端算出的组合摘要。 */
export interface TeacherEnvComponent extends EnvComponentRequest {
  /** composition_digest 是服务端编译后的不可变摘要,只读展示,不作为提交输入 */
  composition_digest: string
}

export interface SimComponent {
  id: string
  package_code: string
  version: string
  seed: number
  params: Record<string, unknown>
}

export interface CheckpointConfig {
  id: string
  judger: string
  item_code: string
  item_version: string
  score: number
  mode?: JudgeSandboxMode
  env_id?: string
  sim_id?: string
  extra_input?: Record<string, unknown>
}

export interface StageConfig {
  stage: number
  title: string
  description?: string
  components: StageComponents
  unlock_condition?: UnlockCondition
  param_bindings?: ParamBinding[]
}

export interface StageComponents {
  envs?: string[]
  sims?: string[]
}

export interface UnlockCondition {
  type: 'checkpoint' | 'manual'
  checkpoint_id?: string
  min_score?: number
}

export interface ParamBinding {
  target_component: string
  target_param: string
  source_type: 'checkpoint' | 'constant'
  source_ref?: string
  source_path?: string
  constant_value?: unknown
}

export interface GroupConfig {
  size: number
  roles: string[]
}

export interface ExperimentRequest {
  course_id?: SnowflakeID
  template_ref: string
  template_version: string
  name: string
  description: string
  components: ComponentConfigRequest
  collab_mode: ExperimentCollabMode
  group_config: GroupConfig
  require_report: boolean
  wizard_step: number
}

export interface ValidationResult {
  ok: boolean
  issues: ValidationIssue[]
}

export interface ValidationIssue {
  level: ExperimentValidationLevel
  message: string
}

export interface CreateInstanceRequest {
  group_id?: SnowflakeID
}

export interface ExperimentInstance {
  instance_id: SnowflakeID
  experiment_id: SnowflakeID
  owner_account_id: SnowflakeID
  group_id?: SnowflakeID
  source_ref: string
  sandboxes: SandboxRef[]
  sims: SimSessionRef[]
  status: ExperimentInstanceStatus
  score: number
  started_at: string
  finished_at?: string
  last_active_at: string
  checkpoints?: CheckpointResult[]
  stages?: StageState[]
}

export interface SandboxRef {
  component_id: string
  stage: number
  sandbox_id: SnowflakeID
  runtime_code: string
  tools: SandboxTool[]
}

export interface SandboxTool {
  code: string
  kind: SandboxToolKind
  endpoint: string
  status: SandboxToolStatus
}

export interface SimSessionRef {
  component_id: string
  stage: number
  session_id: SnowflakeID
  package_code: string
  version: string
  compute: SimCompute
}

export interface CheckpointResult {
  id: string
  judge_task_ref?: string
  passed: boolean
  score: number
  detail_ref?: string
  binding_output?: Record<string, unknown>
}

export interface StageState {
  stage: number
  title: string
  description?: string
  status: ExperimentStageStatus
  components: StageComponents
  unlock_condition?: UnlockCondition
}

export interface ProgressDTO {
  topic: string
  channel: string
}

export interface CheckpointJudgeRequest {
  code_storage_key?: string
  code_hash?: string
  extra_input?: Record<string, unknown>
  binding_output?: Record<string, unknown>
}

export interface ReportDTO {
  id: SnowflakeID
  instance_id: SnowflakeID
  student_id: SnowflakeID
  student_name: string
  student_no?: string
  content_ref: string
  manual_score: number
  comment?: string
  status: ExperimentReportStatus
  submitted_at: string
}

export interface ReportAccessDTO {
  token: string
  expires_at: string
}

export interface GradeReportRequest {
  manual_score: number
  comment: string
}

export interface ExperimentGroupRequest {
  name: string
}

export interface ExperimentGroupMemberRequest {
  student_id: SnowflakeID
  role: string
}

export interface ExperimentGroup {
  id: SnowflakeID
  experiment_id: SnowflakeID
  name: string
  members: ExperimentGroupMember[]
  shared_instance?: ExperimentInstance
  created_at: string
}

export interface ExperimentGroupMember {
  id: SnowflakeID
  group_id: SnowflakeID
  student_id: SnowflakeID
  student_name: string
  student_no?: string
  role: string
  created_at: string
}
