// 竞赛契约常量：维护前端需要与后端 contest 模块枚举编号对齐的值。

export enum ContestMode {
  SOLVE = 1,
  BATTLE = 2,
}

export enum MatchMode {
  ROUND_ROBIN = 1,
  ELO = 2,
}

export enum TeamMode {
  SOLO = 1,
  GROUP = 2,
}

export enum ContestStatus {
  DRAFT = 1,
  SIGNUP = 2,
  RUNNING = 3,
  FROZEN = 4,
  ENDED = 5,
  ARCHIVED = 6,
}

export enum TeamStatus {
  BUILDING = 1,
  LOCKED = 2,
}

export enum BattleRule {
  ATTACK_DEFENSE = 1,
  GAME = 2,
}

export enum BattleRole {
  STRATEGY = 0,
  DEFENSE = 1,
  ATTACK = 2,
}

export enum BattleMatchStatus {
  PENDING = 1,
  RUNNING = 2,
  DONE = 3,
  FAILED = 4,
}

export enum BattleResult {
  A_WIN = 1,
  B_WIN = 2,
  DRAW = 3,
}

export enum CheatType {
  SIMILARITY = 1,
  BEHAVIOR = 2,
  ENVIRONMENT = 3,
}

export enum CheatAction {
  WARN = 1,
  PENALTY = 2,
  DISQUALIFY = 3,
}

export enum VulnSourceType {
  SWC = 1,
  INTELLIGENCE = 2,
  CVE_ONCHAIN = 3,
}

export enum VulnLevel {
  A = 1,
  B = 2,
  C = 3,
}

export enum VulnRuntimeMode {
  ISOLATED = 1,
  FORKED = 2,
}

export enum VulnPrevalidateStatus {
  PENDING = 1,
  PASSED = 2,
  FAILED = 3,
}

export enum VulnProblemStatus {
  DRAFT = 1,
  FINALIZED = 2,
  DISCARDED = 3,
}

/** VULN_CHAIN_OPERATION 是漏洞题预验证步骤的公开操作协议。 */
export const VULN_CHAIN_OPERATION = {
  DEPLOY: 'deploy',
  TX: 'tx',
  RESET: 'reset',
  QUERY: 'query',
} as const

export type VulnChainOperation =
  (typeof VULN_CHAIN_OPERATION)[keyof typeof VULN_CHAIN_OPERATION]

/** CHAIN_ASSERT_OPERATION 是链上状态断言的公开操作协议。 */
export const CHAIN_ASSERT_OPERATION = {
  EQ: 'eq',
  NE: 'ne',
  CONTAINS: 'contains',
  EXISTS: 'exists',
} as const

export type ChainAssertOperation =
  (typeof CHAIN_ASSERT_OPERATION)[keyof typeof CHAIN_ASSERT_OPERATION]
