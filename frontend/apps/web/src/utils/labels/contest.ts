// contest labels 文件维护 M8 竞赛模块枚举的用户向文案与语义色。

import type { BadgeTone, StatusTone } from '@chaimir/ui'
import {
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
} from '@chaimir/api-client'

const CONTEST_STATUS_LABELS: Record<ContestStatus, string> = {
  [ContestStatus.DRAFT]: '未发布',
  [ContestStatus.SIGNUP]: '报名中',
  [ContestStatus.RUNNING]: '进行中',
  [ContestStatus.FROZEN]: '封榜中',
  [ContestStatus.ENDED]: '已结束',
  [ContestStatus.ARCHIVED]: '已归档',
}

const CONTEST_STATUS_TONES: Record<ContestStatus, StatusTone> = {
  [ContestStatus.DRAFT]: 'neutral',
  [ContestStatus.SIGNUP]: 'info',
  [ContestStatus.RUNNING]: 'primary',
  [ContestStatus.FROZEN]: 'warning',
  [ContestStatus.ENDED]: 'success',
  [ContestStatus.ARCHIVED]: 'neutral',
}

/** contestStatusLabel 返回竞赛状态文案。 */
export function contestStatusLabel(status: ContestStatus): string {
  return CONTEST_STATUS_LABELS[status]
}

/** contestStatusTone 返回竞赛状态语义色。 */
export function contestStatusTone(status: ContestStatus): StatusTone {
  return CONTEST_STATUS_TONES[status]
}

/** 封榜期榜单不再更新,页面据此提示用户当前名次可能与最终结果不同。 */
export function isContestLeaderboardFrozen(status: ContestStatus): boolean {
  return status === ContestStatus.FROZEN
}

const CONTEST_MODE_LABELS: Record<ContestMode, string> = {
  [ContestMode.SOLVE]: '解题赛',
  [ContestMode.BATTLE]: '对抗赛',
}

/** contestModeLabel 返回赛制文案。 */
export function contestModeLabel(mode: ContestMode): string {
  return CONTEST_MODE_LABELS[mode]
}

const MATCH_MODE_LABELS: Record<MatchMode, string> = {
  [MatchMode.ROUND_ROBIN]: '循环对局',
  [MatchMode.ELO]: '积分匹配',
}

/** matchModeLabel 返回对抗匹配方式文案。 */
export function matchModeLabel(mode: MatchMode): string {
  return MATCH_MODE_LABELS[mode]
}

const TEAM_MODE_LABELS: Record<TeamMode, string> = {
  [TeamMode.SOLO]: '个人参赛',
  [TeamMode.GROUP]: '组队参赛',
}

/** teamModeLabel 返回参赛形式文案。 */
export function teamModeLabel(mode: TeamMode): string {
  return TEAM_MODE_LABELS[mode]
}

const TEAM_STATUS_LABELS: Record<TeamStatus, string> = {
  [TeamStatus.BUILDING]: '组队中',
  [TeamStatus.LOCKED]: '已锁定',
}

const TEAM_STATUS_TONES: Record<TeamStatus, StatusTone> = {
  [TeamStatus.BUILDING]: 'info',
  [TeamStatus.LOCKED]: 'success',
}

/** teamStatusLabel 返回队伍状态文案。 */
export function teamStatusLabel(status: TeamStatus): string {
  return TEAM_STATUS_LABELS[status]
}

/** teamStatusTone 返回队伍状态语义色。 */
export function teamStatusTone(status: TeamStatus): StatusTone {
  return TEAM_STATUS_TONES[status]
}

const BATTLE_RULE_LABELS: Record<BattleRule, string> = {
  [BattleRule.ATTACK_DEFENSE]: '攻防对抗',
  [BattleRule.GAME]: '博弈对局',
}

/** battleRuleLabel 返回对抗规则文案。 */
export function battleRuleLabel(rule: BattleRule): string {
  return BATTLE_RULE_LABELS[rule]
}

const BATTLE_ROLE_LABELS: Record<BattleRole, string> = {
  [BattleRole.STRATEGY]: '策略方',
  [BattleRole.DEFENSE]: '守方',
  [BattleRole.ATTACK]: '攻方',
}

/** battleRoleLabel 返回参战角色文案。 */
export function battleRoleLabel(role: BattleRole): string {
  return BATTLE_ROLE_LABELS[role]
}

const BATTLE_MATCH_STATUS_LABELS: Record<BattleMatchStatus, string> = {
  [BattleMatchStatus.PENDING]: '等待开局',
  [BattleMatchStatus.RUNNING]: '对局进行中',
  [BattleMatchStatus.DONE]: '对局结束',
  [BattleMatchStatus.FAILED]: '对局未完成',
}

const BATTLE_MATCH_STATUS_TONES: Record<BattleMatchStatus, StatusTone> = {
  [BattleMatchStatus.PENDING]: 'neutral',
  [BattleMatchStatus.RUNNING]: 'primary',
  [BattleMatchStatus.DONE]: 'success',
  [BattleMatchStatus.FAILED]: 'danger',
}

/** battleMatchStatusLabel 返回对局状态文案。 */
export function battleMatchStatusLabel(status: BattleMatchStatus): string {
  return BATTLE_MATCH_STATUS_LABELS[status]
}

/** battleMatchStatusTone 返回对局状态语义色。 */
export function battleMatchStatusTone(status: BattleMatchStatus): StatusTone {
  return BATTLE_MATCH_STATUS_TONES[status]
}

const BATTLE_RESULT_LABELS: Record<BattleResult, string> = {
  [BattleResult.A_WIN]: '先手方获胜',
  [BattleResult.B_WIN]: '后手方获胜',
  [BattleResult.DRAW]: '平局',
}

/** battleResultLabel 返回对局结果文案。 */
export function battleResultLabel(result: BattleResult): string {
  return BATTLE_RESULT_LABELS[result]
}

/** BATTLE_RULES 供赛题表单按登记顺序渲染对抗规则选项。 */
export const BATTLE_RULES = [BattleRule.ATTACK_DEFENSE, BattleRule.GAME] as const

const CHEAT_TYPE_LABELS: Record<CheatType, string> = {
  [CheatType.SIMILARITY]: '代码高度相似',
  [CheatType.BEHAVIOR]: '答题行为异常',
  [CheatType.ENVIRONMENT]: '环境使用违规',
}

/** cheatTypeLabel 返回违规类型文案。 */
export function cheatTypeLabel(type: CheatType): string {
  return CHEAT_TYPE_LABELS[type]
}

/** CHEAT_TYPES 供违规处理表单按登记顺序渲染类型选项。 */
export const CHEAT_TYPES = [CheatType.SIMILARITY, CheatType.BEHAVIOR, CheatType.ENVIRONMENT] as const

const CHEAT_ACTION_LABELS: Record<CheatAction, string> = {
  [CheatAction.WARN]: '警告',
  [CheatAction.PENALTY]: '扣分',
  [CheatAction.DISQUALIFY]: '取消资格',
}

const CHEAT_ACTION_TONES: Record<CheatAction, BadgeTone> = {
  [CheatAction.WARN]: 'warning',
  [CheatAction.PENALTY]: 'danger',
  [CheatAction.DISQUALIFY]: 'danger',
}

/** cheatActionLabel 返回违规处理方式文案。 */
export function cheatActionLabel(action: CheatAction): string {
  return CHEAT_ACTION_LABELS[action]
}

/** cheatActionTone 返回违规处理方式语义色(处理方式呈现为徽标)。 */
export function cheatActionTone(action: CheatAction): BadgeTone {
  return CHEAT_ACTION_TONES[action]
}

/** CHEAT_ACTIONS 供违规处理表单按登记顺序渲染处理方式选项。 */
export const CHEAT_ACTIONS = [CheatAction.WARN, CheatAction.PENALTY, CheatAction.DISQUALIFY] as const

/**
 * 漏洞源类型文案(数据模型 §6.2:1 SWC / 2 漏洞情报 / 3 CVE 链上事件)。
 * 这是封闭枚举而非开放字符串,故用 Record 而不做兜底。
 */
const VULN_SOURCE_TYPE_LABELS = {
  1: '合约弱点分类库',
  2: '公开漏洞情报',
  3: 'CVE 与链上事件',
} as const

export type VulnSourceType = keyof typeof VULN_SOURCE_TYPE_LABELS

/** VULN_SOURCE_TYPES 供漏洞源表单按登记顺序渲染类型选项。 */
export const VULN_SOURCE_TYPES = [1, 2, 3] as const satisfies readonly VulnSourceType[]

/** vulnSourceTypeLabel 返回漏洞源类型文案;未登记类型给通用名,不暴露裸数字。 */
export function vulnSourceTypeLabel(type: number): string {
  return VULN_SOURCE_TYPE_LABELS[type as VulnSourceType] ?? '其他来源'
}

const VULN_LEVEL_LABELS: Record<VulnLevel, string> = {
  [VulnLevel.A]: 'A 级 · 可自动转链上题',
  [VulnLevel.B]: 'B 级 · 需人工补全',
  [VulnLevel.C]: 'C 级 · 理论素材',
}

const VULN_LEVEL_TONES: Record<VulnLevel, BadgeTone> = {
  [VulnLevel.A]: 'success',
  [VulnLevel.B]: 'warning',
  [VulnLevel.C]: 'neutral',
}

/** vulnLevelLabel 返回漏洞可复现性分级文案。 */
export function vulnLevelLabel(level: VulnLevel): string {
  return VULN_LEVEL_LABELS[level]
}

/** vulnLevelTone 返回漏洞分级语义色(分级呈现为徽标)。 */
export function vulnLevelTone(level: VulnLevel): BadgeTone {
  return VULN_LEVEL_TONES[level]
}

/** VULN_LEVELS 供漏洞题表单按登记顺序渲染分级选项。 */
export const VULN_LEVELS = [VulnLevel.A, VulnLevel.B, VulnLevel.C] as const

const VULN_RUNTIME_MODE_LABELS: Record<VulnRuntimeMode, string> = {
  [VulnRuntimeMode.ISOLATED]: '干净测试链复现',
  [VulnRuntimeMode.FORKED]: '主网分叉复现',
}

/** vulnRuntimeModeLabel 返回漏洞题运行方式文案。 */
export function vulnRuntimeModeLabel(mode: VulnRuntimeMode): string {
  return VULN_RUNTIME_MODE_LABELS[mode]
}

/** VULN_RUNTIME_MODES 供漏洞题表单按登记顺序渲染运行方式选项。 */
export const VULN_RUNTIME_MODES = [VulnRuntimeMode.ISOLATED, VulnRuntimeMode.FORKED] as const

const VULN_PREVALIDATE_STATUS_LABELS: Record<VulnPrevalidateStatus, string> = {
  [VulnPrevalidateStatus.PENDING]: '尚未验证',
  [VulnPrevalidateStatus.PASSED]: '验证通过',
  [VulnPrevalidateStatus.FAILED]: '验证未通过',
}

const VULN_PREVALIDATE_STATUS_TONES: Record<VulnPrevalidateStatus, StatusTone> = {
  [VulnPrevalidateStatus.PENDING]: 'neutral',
  [VulnPrevalidateStatus.PASSED]: 'success',
  [VulnPrevalidateStatus.FAILED]: 'danger',
}

/** vulnPrevalidateStatusLabel 返回漏洞题预验证状态文案。 */
export function vulnPrevalidateStatusLabel(status: VulnPrevalidateStatus): string {
  return VULN_PREVALIDATE_STATUS_LABELS[status]
}

/** vulnPrevalidateStatusTone 返回漏洞题预验证状态语义色。 */
export function vulnPrevalidateStatusTone(status: VulnPrevalidateStatus): StatusTone {
  return VULN_PREVALIDATE_STATUS_TONES[status]
}

const VULN_PROBLEM_STATUS_LABELS: Record<VulnProblemStatus, string> = {
  [VulnProblemStatus.DRAFT]: '草稿',
  [VulnProblemStatus.FINALIZED]: '已固化到题库',
  [VulnProblemStatus.DISCARDED]: '已弃用',
}

const VULN_PROBLEM_STATUS_TONES: Record<VulnProblemStatus, StatusTone> = {
  [VulnProblemStatus.DRAFT]: 'neutral',
  [VulnProblemStatus.FINALIZED]: 'success',
  [VulnProblemStatus.DISCARDED]: 'neutral',
}

/** vulnProblemStatusLabel 返回漏洞题草稿状态文案。 */
export function vulnProblemStatusLabel(status: VulnProblemStatus): string {
  return VULN_PROBLEM_STATUS_LABELS[status]
}

/** vulnProblemStatusTone 返回漏洞题草稿状态语义色。 */
export function vulnProblemStatusTone(status: VulnProblemStatus): StatusTone {
  return VULN_PROBLEM_STATUS_TONES[status]
}

/**
 * 漏洞题草稿正文的结构化字段。draft_body 是 JSONB 开放对象,
 * 但键名不是自由的:后端预验证按 init_steps / positive_steps / assertions 三个数组执行
 * (service_vuln 的 validationSteps 与 checkVulnAssertions),
 * 题面部分(说明/合约源码/分叉区块)随 finalize 原样进 M5 题目正文。
 * 故表单按这些键渲染显式字段,不给裸 JSON 文本域。
 */
export const VULN_DRAFT_BODY_FIELDS = {
  description: 'description',
  contractSource: 'contract_source',
  forkBlock: 'fork_block',
  initSteps: 'init_steps',
  positiveSteps: 'positive_steps',
  assertions: 'assertions',
} as const

/** 链上步骤的键(后端 runVulnChainStep 读 op 与 payload)。 */
export const VULN_CHAIN_STEP_FIELDS = {
  op: 'op',
  payload: 'payload',
} as const

const VULN_CHAIN_OP_LABELS = {
  deploy: '部署合约',
  tx: '发起交易',
  reset: '重置链状态',
  query: '仅查询(不改状态)',
} as const

export type VulnChainOp = keyof typeof VULN_CHAIN_OP_LABELS

/** VULN_CHAIN_OPS 供步骤表单按后端支持顺序渲染操作选项。 */
export const VULN_CHAIN_OPS = ['deploy', 'tx', 'reset', 'query'] as const satisfies readonly VulnChainOp[]

/** vulnChainOpLabel 返回链上步骤操作文案。 */
export function vulnChainOpLabel(op: VulnChainOp): string {
  return VULN_CHAIN_OP_LABELS[op]
}

/** 断言的键(后端 chainassert.FromMap 读取这些键)。 */
export const VULN_ASSERTION_FIELDS = {
  label: 'label',
  target: 'target',
  field: 'field',
  op: 'op',
  value: 'value',
  expectedLabel: 'expected_label',
  hint: 'hint',
} as const

const VULN_ASSERT_OP_LABELS = {
  eq: '等于期望值',
  ne: '不等于期望值',
  contains: '包含期望值',
  exists: '该字段存在',
} as const

export type VulnAssertOp = keyof typeof VULN_ASSERT_OP_LABELS

/** VULN_ASSERT_OPS 供断言表单按后端支持顺序渲染判定方式。 */
export const VULN_ASSERT_OPS = ['eq', 'ne', 'contains', 'exists'] as const satisfies readonly VulnAssertOp[]

/** vulnAssertOpLabel 返回断言判定方式文案。 */
export function vulnAssertOpLabel(op: VulnAssertOp): string {
  return VULN_ASSERT_OP_LABELS[op]
}

/**
 * 漏洞源同步配置的结构化字段(专题设计 §1 的 config 结构)。
 * 表单按这些键渲染,密钥类字段由后端加密保存。
 */
export const VULN_SOURCE_CONFIG_FIELDS = {
  endpoint: 'endpoint',
  method: 'method',
  timeoutSeconds: 'timeout_seconds',
  casesPath: 'cases_path',
  mapping: 'mapping',
} as const

/** 源同步字段映射的五个键:后端 validateVulnSourceConfig 要求前三个必填。 */
export const VULN_SOURCE_MAPPING_FIELDS = {
  externalRef: 'external_ref',
  title: 'title',
  level: 'level',
  runtimeMode: 'runtime_mode',
  draftBody: 'draft_body',
} as const

/** 漏洞源请求方法:后端只允许 GET 与 POST。 */
export const VULN_SOURCE_METHODS = ['GET', 'POST'] as const
