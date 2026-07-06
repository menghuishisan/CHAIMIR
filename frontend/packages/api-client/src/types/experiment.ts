// ===== M7 Experiment 模块 =====

import type {
  ExperimentCollabMode,
  ExperimentInstanceStatus,
  ExperimentReportStatus,
  ExperimentStageStatus,
  ExperimentStatus,
} from '../constants/experiment'
import type { SandboxToolKind, SandboxToolStatus } from '../constants/sandbox'

export interface Experiment {
  id: string
  course_id?: string
  author_id: string
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

export interface ComponentConfig {
  envs: EnvComponent[]
  sims: SimComponent[]
  checkpoints: CheckpointConfig[]
  stages: StageConfig[]
}

export interface EnvComponent {
  id: string
  runtime_code: string
  runtime_image_version?: string
  tools: string[]
  init_code_ref?: string
  init_script_ref?: string
  keep_alive?: boolean
  snapshot_enabled?: boolean
  keep_alive_minutes?: number
  snapshot_retention_minutes?: number
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
  mode?: string
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
  course_id: number
  template_ref: string
  template_version: string
  name: string
  description: string
  components: ComponentConfig
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
  level: string
  message: string
}

export interface CreateInstanceRequest {
  group_id?: number
}

export interface ExperimentInstance {
  instance_id: string
  experiment_id: string
  owner_account_id: string
  group_id?: string
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
  sandbox_id: string
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
  session_id: string
  package_code: string
  version: string
  bundle_ref: string
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
  id: string
  instance_id: string
  student_id: string
  content_ref: string
  manual_score: number
  comment?: string
  status: ExperimentReportStatus
  submitted_at: string
}

export interface GradeReportRequest {
  manual_score: number
  comment: string
}

export interface ExperimentGroupRequest {
  name: string
}

export interface ExperimentGroupMemberRequest {
  student_id: number
  role: string
}

export interface ExperimentGroup {
  id: string
  experiment_id: string
  name: string
  members: ExperimentGroupMember[]
  shared_instance?: ExperimentInstance
  created_at: string
}

export interface ExperimentGroupMember {
  id: string
  group_id: string
  student_id: string
  role: string
  created_at: string
}
