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
import type { ChainAssertion, ContestProblemBody } from './content'

/** DynamicScoreConfig 定义解题人数增长时的唯一衰减规则。 */
export interface DynamicScoreConfig {
  min_score: number
  decay_per_solve: number
}

/** BattleRuntimeConfig 定义对抗题唯一的沙箱运行配置。 */
export interface BattleRuntimeConfig {
  runtime_code: string
  runtime_image_version: string
  tool_codes: string[]
}

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
}

export interface ContestProblem {
  id: SnowflakeID
  contest_id: SnowflakeID
  title: string
  item_code: string
  item_version: string
  score: number
  dynamic_score?: DynamicScoreConfig
  battle_config?: BattleRuntimeConfig
  battle_rule?: BattleRule
  seq: number
  face?: ContestProblemBody
}

export interface ContestProblemRequest {
  item_code: string
  item_version: string
  score: number
  dynamic_score?: DynamicScoreConfig
  battle_config?: BattleRuntimeConfig
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
  content_ref: ContestAnswerContentRef
  source_ref: string
  judge_task_ref?: string
  passed: boolean
  score: number
  sandbox_ref?: string
  submitted_at: string
}

export interface ContestSubmitRequest {
  content_ref: ContestAnswerContentRef
  code_storage_key?: string
  code_hash?: string
  sandbox_ref?: string
}

/** ContestAnswerContentRef 是学生可直接补充的唯一答案结构。 */
export interface ContestAnswerContentRef {
  answer: string
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
  replay_available: boolean
  status: BattleMatchStatus
  matched_at: string
  finished_at?: string
}

export interface BattleReplayStep {
  seq: number
  title: string
  source?: string
  target?: string
  passed: boolean
  actual?: string
  hint?: string
}

export interface BattleReplay {
  match_id: SnowflakeID
  problem_title: string
  result: BattleResult
  score_delta: Record<string, unknown>
  steps: BattleReplayStep[]
  finished_at: string
}

export interface LadderRank {
  team_id: SnowflakeID
  team_name: string
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
  final_ranking: LadderRank[]
  generated_at: string
}

export interface CheatRecordRequest {
  team_id: SnowflakeID
  type: CheatType
  evidence: CheatEvidence
  action: CheatAction
}

export interface CheatRecord {
  id: SnowflakeID
  contest_id: SnowflakeID
  team_id: SnowflakeID
  type: CheatType
  evidence: CheatEvidence
  action: CheatAction
  operator_id?: SnowflakeID
  created_at: string
}

/** CheatEvidence 是人工违规复核的唯一可读证据结构。 */
export interface CheatEvidence {
  review_note: string
  source_refs: string[]
  penalty_score?: number
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
  config: VulnSourceConfig
  default_level: VulnLevel
  enabled: boolean
}

export interface VulnSource {
  id: SnowflakeID
  type: number
  name: string
  config: VulnSourceConfig
  default_level: VulnLevel
  enabled: boolean
  last_sync_at?: string
}

export interface VulnSourceConfig {
  endpoint: string
  method: 'GET' | 'POST'
  timeout_seconds?: number
  headers?: Record<string, string>
  body?: Record<string, unknown>
  cases_path?: string
  mapping: VulnSourceMapping
}

export interface VulnSourceMapping {
  external_ref: string
  title: string
  level?: string
  runtime_mode?: string
  draft_body: string
}

export interface VulnProblemImportRequest {
  source_id?: SnowflakeID
  external_ref?: string
  title: string
  level: VulnLevel
  runtime_mode: VulnRuntimeMode
  draft_body: VulnDraftBody
}

/** VulnChainStep 描述漏洞预验证中的一条链操作。 */
export interface VulnChainStep {
  op: 'deploy' | 'tx' | 'query' | 'reset'
  payload: Record<string, unknown>
}

/** VulnJudgeConfig 是漏洞草稿在固化前使用的判题器配置。 */
export interface VulnJudgeConfig {
  judger_code: string
  suite_ref?: string
  max_score: number
}

/** VulnDraftBody 是漏洞题转化与预验证共用的唯一草稿结构。 */
export interface VulnDraftBody {
  statement: string
  judge_config: VulnJudgeConfig
  init_contracts: string[]
  init_steps: VulnChainStep[]
  positive_steps: VulnChainStep[]
  assertions: ChainAssertion[]
  ad_config?: BattleRuntimeConfig
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
  draft_body: VulnDraftBody
  prevalidate_status: VulnPrevalidateStatus
  prevalidate_detail: Record<string, unknown>
  content_item_code?: string
  content_item_version?: string
  status: VulnProblemStatus
}
