// ===== M8 Contest 模块 =====

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
} from '../constants/contest'
import type { SandboxStatus } from '../constants/sandbox'

export interface Contest {
  id: string
  organizer_id: string
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
  id: string
  contest_id: string
  item_code: string
  item_version: string
  score: number
  dynamic_score?: Record<string, unknown>
  battle_config?: Record<string, unknown>
  battle_rule?: BattleRule
  seq: number
  face?: Record<string, unknown>
}

export interface ContestProblemRequest {
  item_code: string
  item_version: string
  score: number
  dynamic_score?: Record<string, unknown>
  battle_config?: Record<string, unknown>
  battle_rule?: BattleRule
  seq: number
}

export interface ContestTeam {
  id: string
  contest_id: string
  name: string
  invite_code?: string
  status: TeamStatus
  created_at: string
  members: TeamMember[]
}

export interface TeamMember {
  id: string
  team_id: string
  account_id: string
  member_tenant_id: string
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
  id: string
  contest_id: string
  problem_id: string
  team_id: string
  submitter_id: string
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

export interface EnvRequest {
  runtime_code: string
  runtime_image_version: string
  tool_codes: string[]
  init_code_ref?: string
  init_script_ref?: string
}

export interface EnvSummary {
  sandbox_id: string
  source_ref: string
  status: SandboxStatus
}

export interface BattleEntryRequest {
  problem_id: number
  role: BattleRole
  artifact_ref: string
  code_hash: string
}

export interface BattleEntry {
  id: string
  contest_id: string
  problem_id: string
  team_id: string
  role: BattleRole
  artifact_ref: string
  code_hash: string
  version_no: number
  is_active: boolean
  submitted_at: string
}

export interface BattleMatch {
  id: string
  contest_id: string
  problem_id: string
  entry_a_id: string
  entry_b_id: string
  source_ref: string
  sandbox_ref?: string
  judge_task_ref?: string
  result?: BattleResult
  score_delta: Record<string, unknown>
  replay_ref?: string
  status: BattleMatchStatus
  matched_at: string
  finished_at?: string
}

export interface BattleReplayRef {
  match_id: string
  replay_ref: string
}

export interface LadderRank {
  team_id: string
  score: number
  solved_count: number
  last_solve_at?: string
  rank: number
  updated_at: string
}

export interface ResultSnapshot {
  id: string
  tenant_id?: string
  contest_id: string
  final_ranking: Record<string, unknown>[]
  generated_at: string
}

export interface CheatRecordRequest {
  team_id: number
  type: CheatType
  evidence: Record<string, unknown>
  action: CheatAction
}

export interface CheatRecord {
  id: string
  contest_id: string
  team_id: string
  type: CheatType
  evidence: Record<string, unknown>
  action: CheatAction
  operator_id?: string
  created_at: string
}

export interface CheatSuspect {
  source_ref: string
  submitter_id: string
  score: number
  code_hash?: string
}

export interface ContestRecord {
  contest_id: string
  team_id: string
  score: number
  rank: number
  contest_name: string
  contest_status: ContestStatus
}

export interface VulnSourceRequest {
  id?: number
  type: number
  name: string
  config: Record<string, unknown>
  default_level: VulnLevel
  enabled: boolean
}

export interface VulnSource {
  id: string
  type: number
  name: string
  config: Record<string, unknown>
  default_level: VulnLevel
  enabled: boolean
  last_sync_at?: string
}

export interface VulnProblemImportRequest {
  source_id?: number
  external_ref?: string
  title: string
  level: VulnLevel
  runtime_mode: VulnRuntimeMode
  draft_body: Record<string, unknown>
}

export interface VulnPrevalidateRequest {
  runtime_code: string
  runtime_image_version: string
  tool_codes: string[]
  init_code_ref?: string
  init_script_ref?: string
}

export interface VulnProblem {
  id: string
  source_id?: string
  external_ref?: string
  title: string
  level: VulnLevel
  runtime_mode: VulnRuntimeMode
  draft_body: Record<string, unknown>
  prevalidate_status: VulnPrevalidateStatus
  prevalidate_detail: Record<string, unknown>
  content_item_code?: string
  content_item_version?: string
  status: VulnProblemStatus
}
