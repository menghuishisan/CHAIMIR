import type { SnowflakeID } from './common'
import type {
  CompositionComponentRef,
  CompositionInitAsset,
  CompositionChainStep,
  CompositionLink,
  CompositionRuntimeRef,
} from './composition'
import type { ContentType } from '../constants/content'
import type { SandboxAccessProfile } from '../constants/composition'
import type { JudgeSandboxMode, JudgeTaskStatusCode } from '../constants/judge'
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
  /** 本人最近一次实验的进度;从没开始过的实验不带 */
  my_progress?: StudentExperimentProgress
}

/** 本人在一个实验里最近一次实验的进度;小组实验看本人所在小组的实验 */
interface StudentExperimentProgress {
  status: ExperimentInstanceStatus
  score: number
  passed_checkpoints: number
}

interface StudentComponentConfig {
  envs: StudentEnvComponent[]
  sims: StudentSimComponent[]
  checkpoints: StudentCheckpointConfig[]
  stages: StudentStageConfig[]
}

/** 学生仅可见工具、运行时与连接。执行快照留在服务端。 */
interface StudentEnvComponent {
  id: string
  runtimes: CompositionRuntimeRef[]
  workspace_runtime_instance: string
  infra: CompositionComponentRef[]
  tools: CompositionComponentRef[]
  links: CompositionLink[]
  access_profile: SandboxAccessProfile
  composition_digest: string
}

interface StudentSimComponent {
  id: string
  package_code: string
  version: string
}

export interface StudentCheckpointConfig {
  id: string
  score: number
  mode?: JudgeSandboxMode
  title?: string
  type?: ContentType
  face?: Record<string, unknown>
}

export interface StudentStageConfig {
  stage: number
  title: string
  description?: string
  components: StageComponents
  unlock_condition?: UnlockCondition
}

/** 教师视图回显可编辑声明，并附上只读摘要。 */
interface ComponentConfig {
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

/** 教师提交环境声明。镜像与执行配置由服务端编译。 */
export interface EnvComponentRequest {
  id: string
  runtimes: CompositionRuntimeRef[]
  workspace_runtime_instance: string
  infra: CompositionComponentRef[]
  tools: CompositionComponentRef[]
  links: CompositionLink[]
  access_profile: SandboxAccessProfile
  resource_profile?: Record<string, string>
  network_profile?: Record<string, unknown>
  init_assets?: CompositionInitAsset[]
  init_chain_steps?: CompositionChainStep[]
  init_code_ref?: string
  init_script_ref?: string
  keep_alive: boolean
  snapshot_enabled: boolean
  keep_alive_minutes: number
  snapshot_retention_minutes: number
}

/** TeacherEnvComponent 是读取时的环境投影:声明字段 + 服务端算出的组合摘要。 */
interface TeacherEnvComponent extends EnvComponentRequest {
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

interface SandboxTool {
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
  judge_status: JudgeTaskStatusCode
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
  manual_score?: number
  comment?: string
  status: ExperimentReportStatus
  submitted_at: string
}

/** 学生读取本人报告批改结果的安全投影,不包含对象存储引用。 */
export interface StudentReportDTO {
  id: SnowflakeID
  instance_id: SnowflakeID
  manual_score?: number
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
  submitted_at: string
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

interface ExperimentGroupMember {
  id: SnowflakeID
  group_id: SnowflakeID
  student_id: SnowflakeID
  student_name: string
  student_no?: string
  role: string
  created_at: string
}
