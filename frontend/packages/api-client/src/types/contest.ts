// ===== M8 Contest 模块 =====

import type { SnowflakeID } from './common'
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

export interface EnvRequest {
  runtime_code: string
  runtime_image_version: string
  tool_codes: string[]
  init_code_ref?: string
  init_script_ref?: string
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
  score_delta: Record<string, unknown>
  replay_ref?: string
  status: BattleMatchStatus
  matched_at: string
  finished_at?: string
}

export interface BattleReplayRef {
  match_id: SnowflakeID
  replay_ref: string
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
  final_ranking: Record<string, unknown>[]
  generated_at: string
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
  type: number
  name: string
  config: Record<string, unknown>
  default_level: VulnLevel
  enabled: boolean
}

export interface VulnSource {
  id: SnowflakeID
  type: number
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

export interface VulnPrevalidateRequest {
  runtime_code: string
  runtime_image_version: string
  tool_codes: string[]
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
  content_item_code?: string
  content_item_version?: string
  status: VulnProblemStatus
}
