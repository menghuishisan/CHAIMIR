// ===== M8 Contest 模块 =====

import type { SnowflakeID } from './common'
import type { SandboxCompositionSpec } from './composition'
import type { SandboxAccessProfile } from '../constants/composition'
import type {
  BattleMatchStatus,
  BattleResult,
  BattleRole,
  BattleRule,
  CheatAction,
  CheatType,
  ContestMode,
  ContestStatus,
  MatchMode,
  TeamMode,
  TeamStatus,
  VulnLevel,
  VulnPrevalidateStatus,
  VulnProblemStatus,
  VulnRuntimeMode,
  VulnSourceType,
} from '../constants/contest'
import type { SandboxPhase, SandboxStatus } from '../constants/sandbox'
import type {
  SandboxCommandToolRunRequest,
  SandboxCommandToolRunResponse,
  SandboxFileListResponse,
  SandboxFileReadResponse,
  SandboxFileSaveResponse,
  SandboxFileWriteRequest,
  SandboxChainRequest,
  SandboxChainResponse,
} from './sandbox'

/** M8 竞赛沙箱网关复用 M2 工作区数据形状,但不向学生暴露 tenant_id。 */
export type ContestSandboxResponse = {
  sandbox_id: SnowflakeID
  source_ref: string
  owner_account_id: SnowflakeID
  runtime_code: string
  runtime_image_version: string
  phase: SandboxPhase
  status: SandboxStatus
  tool_access: import('./sandbox').SandboxToolAccess[]
  capabilities: import('./sandbox').SandboxCapabilities
  resource_usage: import('./sandbox').SandboxResourceUsage
  workspace_revision: number
}

export type ContestSandboxFileReadResponse = SandboxFileReadResponse
export type ContestSandboxFileListResponse = SandboxFileListResponse
export type ContestSandboxFileWriteRequest = SandboxFileWriteRequest
export type ContestSandboxFileWriteResponse = { workspace_revision: number }
export type ContestSandboxFileSaveResponse = SandboxFileSaveResponse
export type ContestSandboxCommandToolRunRequest = SandboxCommandToolRunRequest
export type ContestSandboxCommandToolRunResponse = SandboxCommandToolRunResponse
export type ContestSandboxChainRequest = SandboxChainRequest
export type ContestSandboxChainResponse = SandboxChainResponse

export interface Contest {
  id: SnowflakeID
  organizer_id: SnowflakeID
  name: string
  mode: ContestMode
  match_mode?: MatchMode
  team_mode: TeamMode
  signup_start: string
  signup_end: string
  start_at: string
  end_at: string
  freeze_minutes: number
  rules: Record<string, unknown>
  status: ContestStatus
  created_at: string
  updated_at: string
}

export interface ContestRequest {
  name: string
  mode: ContestMode
  match_mode?: MatchMode
  team_mode: TeamMode
  signup_start: string
  signup_end: string
  start_at: string
  end_at: string
  freeze_minutes: number
  rules: Record<string, unknown>
}

export interface ContestProblem {
  id: SnowflakeID
  contest_id: SnowflakeID
  item_code: string
  item_version: string
  score: number
  dynamic_score?: ContestDynamicScoreConfig
  battle_config?: ContestBattleRuntimeConfig
  battle_rule?: BattleRule
  seq: number
  composition_digest?: string
  face?: Record<string, unknown>
}

export interface ContestProblemRequest {
  item_code: string
  item_version: string
  score: number
  dynamic_score?: ContestDynamicScoreConfig
  battle_config?: ContestBattleRuntimeConfig
  battle_rule?: BattleRule
  seq: number
}

/** ContestDynamicScoreConfig 是按已解出队伍数衰减的固定计分参数。 */
export interface ContestDynamicScoreConfig {
  min_score: number
  decay_per_solve: number
}

/**
 * ContestBattleRuntimeConfig 是对抗题的赛制参数。
 * 环境组合不在这里 —— 对抗环境由题目版本(M5)提供的已发布组合快照决定,
 * 竞赛侧只保存「怎么打」:执行边界、入场角色与复盘参数。
 */
export interface ContestBattleRuntimeConfig {
  /** execution_profile 固定为对抗赛访问边界,由后端校验闭集 */
  execution_profile: SandboxAccessProfile
  entry_roles: BattleRole[]
  replay_profile: Record<string, unknown>
}

export interface ContestTeam {
  id: SnowflakeID
  contest_id: SnowflakeID
  name: string
  invite_code?: string
  status: TeamStatus
  created_at: string
  members: TeamMember[]
}

export interface TeamMember {
  id: SnowflakeID
  team_id: SnowflakeID
  account_id: SnowflakeID
  member_tenant_id: SnowflakeID
  is_leader: boolean
  joined_at: string
}

export interface SignupRequest {
  team_name: string
}

export interface JoinTeamRequest {
  invite_code: string
}

export interface ContestSubmission {
  id: SnowflakeID
  contest_id: SnowflakeID
  problem_id: SnowflakeID
  team_id: SnowflakeID
  submitter_id: SnowflakeID
  content_ref: Record<string, unknown>
  source_ref: string
  judge_task_ref?: string
  passed: boolean
  score: number
  sandbox_ref?: string
  submitted_at: string
}

export interface ContestSubmitRequest {
  content_ref: Record<string, unknown>
  code_storage_key?: string
  code_hash?: string
  sandbox_ref?: string
}

/**
 * EnvRequest 是实操题环境启动请求。
 * 环境内容取自题目已发布的组合快照,故这里只带一个幂等引用 ——
 * 学生端不声明运行时或工具,也就无法绕过赛题冻结的执行内容。
 */
export interface EnvRequest {
  request_ref: string
}

export interface EnvSummary {
  sandbox_id: SnowflakeID
  source_ref: string
  status: SandboxStatus
}

export interface BattleEntryRequest {
  problem_id: SnowflakeID
  role: BattleRole
  artifact_ref: string
  code_hash: string
}

export interface BattleEntry {
  id: SnowflakeID
  contest_id: SnowflakeID
  problem_id: SnowflakeID
  team_id: SnowflakeID
  role: BattleRole
  artifact_ref: string
  code_hash: string
  version_no: number
  is_active: boolean
  submitted_at: string
}

export interface BattleMatch {
  id: SnowflakeID
  contest_id: SnowflakeID
  problem_id: SnowflakeID
  entry_a_id: SnowflakeID
  entry_b_id: SnowflakeID
  source_ref: string
  sandbox_ref?: string
  judge_task_ref?: string
  result?: BattleResult
  score_delta?: BattleScoreDelta
  replay_available: boolean
  status: BattleMatchStatus
  matched_at: string
  finished_at?: string
}

export interface BattleScoreDelta {
  team_a: SnowflakeID
  team_b: SnowflakeID
  rating_a_before: number
  rating_b_before: number
  rating_a_after: number
  rating_b_after: number
  delta_a: number
  delta_b: number
  k_factor: number
  result: BattleResult
}

export interface BattleReplayRef {
  match_id: SnowflakeID
  available: boolean
}

export interface BattleReplayWindow {
  list: BattleReplayWindowItem[]
  total: number
  page: number
  size: number
  pending: number
  checkpoint: BattleReplayCheckpoint
}

export interface BattleReplayWindowItem {
  match: BattleMatch
  sequence: number
  my_side: 'a' | 'b'
  active_entry?: BattleReplayActiveEntry
}

export interface BattleReplayActiveEntry {
  id: SnowflakeID
  role: BattleRole
  version_no: number
  submitted_at: string
}

export interface BattleReplayCheckpoint {
  wins: number
  losses: number
  draws: number
  rating_delta: number
  rating: number
}

export interface BattleReplayDownloadGrant {
  token: string
  mode: 'download'
  file_name: string
  expires_at: string
}

/**
 * 对局回放归档的正文结构,镜像后端 `contest/service_battle_replay.go` 的 battleReplayArchive。
 * 它不是 REST 响应,而是经「签发授权 → 统一文件服务取件」拿到的 JSON 归档
 * (见 docs/08-竞赛/04-接口设计.md §对局回放);故类型在此登记,校验在消费方边界处做。
 */
export interface BattleReplayArchive {
  /** 归档协议版本;不认识的版本一律拒绝解析,不做兼容猜测 */
  version: number
  match_id: SnowflakeID
  task_id: SnowflakeID
  source_ref: string
  initial_state: BattleReplayInitialState
  /** M3 从 M2 链能力调用收集到的可复现动作序列,按执行顺序排列 */
  actions: BattleReplayAction[]
  result: BattleReplayResult
  finished_at: string
}

export interface BattleReplayInitialState {
  contest_id: SnowflakeID
  problem_id: SnowflakeID
  battle_rule: BattleRule
  entry_a: BattleReplayEntryState
  entry_b: BattleReplayEntryState
}

export interface BattleReplayEntryState {
  role: BattleRole
  version_no: number
  artifact_hash: string
}

export interface BattleReplayAction {
  seq: number
  /** 链操作类型:deploy 部署 / tx 交易 / reset 重置(与 M3 runChainStep 的封闭集一致) */
  op: string
  payload?: Record<string, unknown>
  output?: Record<string, unknown>
}

export interface BattleReplayResult {
  passed: boolean
  score: number
  max_score: number
  details: BattleReplayResultDetail[]
  result_ref?: string
}

export interface BattleReplayResultDetail {
  case: string
  passed: boolean
  expected_label?: string
  actual?: string
  hint?: string
}

export interface LadderRank {
  team_id: SnowflakeID
  score: number
  solved_count: number
  last_solve_at?: string
  rank: number
  updated_at: string
}

export interface ResultSnapshot {
  id: SnowflakeID
  tenant_id?: SnowflakeID
  contest_id: SnowflakeID
  final_ranking: LadderSnapshotEntry[]
  generated_at: string
}

export interface LadderSnapshotEntry {
  team_id: SnowflakeID
  score: number
  solved_count: number
  last_solve_at?: string
  rank: number
  updated_at: string
}

export interface CheatRecordRequest {
  team_id: SnowflakeID
  type: CheatType
  evidence: Record<string, unknown>
  action: CheatAction
}

export interface CheatRecord {
  id: SnowflakeID
  contest_id: SnowflakeID
  team_id: SnowflakeID
  type: CheatType
  evidence: Record<string, unknown>
  action: CheatAction
  operator_id?: SnowflakeID
  created_at: string
}

export interface CheatSuspect {
  source_ref: string
  submitter_id: SnowflakeID
  score: number
  code_hash?: string
}

export interface ContestRecord {
  contest_id: SnowflakeID
  team_id: SnowflakeID
  score: number
  rank: number
  contest_name: string
  contest_status: ContestStatus
}

export interface VulnSourceRequest {
  id?: SnowflakeID
  type: VulnSourceType
  name: string
  config: Record<string, unknown>
  default_level: VulnLevel
  enabled: boolean
}

export interface VulnSource {
  id: SnowflakeID
  type: VulnSourceType
  name: string
  config: Record<string, unknown>
  default_level: VulnLevel
  enabled: boolean
  last_sync_at?: string
}

export interface VulnProblemImportRequest {
  source_id?: SnowflakeID
  external_ref?: string
  title: string
  level: VulnLevel
  runtime_mode: VulnRuntimeMode
  draft_body: Record<string, unknown>
}

/**
 * VulnPrevalidateRequest 是漏洞题预验证请求:教师声明用什么组合跑正反向验证。
 * 只提交声明,编译与执行都在服务端;结果写回 prevalidate_status / prevalidate_detail。
 */
export interface VulnPrevalidateRequest {
  composition: SandboxCompositionSpec
  init_code_ref?: string
  init_script_ref?: string
}

export interface VulnProblem {
  id: SnowflakeID
  source_id?: SnowflakeID
  external_ref?: string
  title: string
  level: VulnLevel
  runtime_mode: VulnRuntimeMode
  draft_body: Record<string, unknown>
  prevalidate_status: VulnPrevalidateStatus
  prevalidate_detail: Record<string, unknown>
  composition_digest?: string
  content_item_code?: string
  content_item_version?: string
  status: VulnProblemStatus
}
