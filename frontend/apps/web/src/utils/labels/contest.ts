// contest labels 文件维护 M8 竞赛模块枚举的用户向文案。

import {
  BattleMatchStatus,
  BattleResult,
  BattleRole,
  BattleRule,
  CHAIN_ASSERT_OPERATION,
  CheatAction,
  CheatType,
  ContestMode,
  ContestStatus,
  MatchMode,
  TeamMode,
  TeamStatus,
  VULN_CHAIN_OPERATION,
  VulnLevel,
  VulnPrevalidateStatus,
  VulnProblemStatus,
  VulnRuntimeMode,
  VulnSourceType,
} from '@chaimir/api-client'

const CONTEST_STATUS_LABELS: Record<ContestStatus, string> = {
  [ContestStatus.DRAFT]: '未发布',
  [ContestStatus.SIGNUP]: '报名中',
  [ContestStatus.RUNNING]: '进行中',
  [ContestStatus.FROZEN]: '封榜中',
  [ContestStatus.ENDED]: '已结束',
  [ContestStatus.ARCHIVED]: '已归档',
}

/** contestStatusLabel 返回竞赛状态文案。 */
export function contestStatusLabel(status: ContestStatus): string {
  return CONTEST_STATUS_LABELS[status]
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

/** teamStatusLabel 返回队伍状态文案。 */
export function teamStatusLabel(status: TeamStatus): string {
  return TEAM_STATUS_LABELS[status]
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

/** battleMatchStatusLabel 返回对局状态文案。 */
export function battleMatchStatusLabel(status: BattleMatchStatus): string {
  return BATTLE_MATCH_STATUS_LABELS[status]
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

const CHEAT_TYPE_LABELS: Record<CheatType, string> = {
  [CheatType.SIMILARITY]: '代码高度相似',
  [CheatType.BEHAVIOR]: '答题行为异常',
  [CheatType.ENVIRONMENT]: '环境使用违规',
}

/** cheatTypeLabel 返回违规类型文案。 */
export function cheatTypeLabel(type: CheatType): string {
  return CHEAT_TYPE_LABELS[type]
}

const CHEAT_ACTION_LABELS: Record<CheatAction, string> = {
  [CheatAction.WARN]: '警告',
  [CheatAction.PENALTY]: '扣分',
  [CheatAction.DISQUALIFY]: '取消资格',
}

/** cheatActionLabel 返回违规处理方式文案。 */
export function cheatActionLabel(action: CheatAction): string {
  return CHEAT_ACTION_LABELS[action]
}

/**
 * 漏洞源类型文案(数据模型 §6.2:1 SWC / 2 漏洞情报 / 3 CVE 链上事件)。
 * 这是封闭枚举而非开放字符串,故用 Record 而不做兜底。
 */
const VULN_SOURCE_TYPE_LABELS: Record<VulnSourceType, string> = {
  [VulnSourceType.SWC]: '合约弱点分类库',
  [VulnSourceType.INTELLIGENCE]: '公开漏洞情报',
  [VulnSourceType.CVE_ONCHAIN]: 'CVE 与链上事件',
}

/** vulnSourceTypeLabel 返回漏洞源类型文案;未登记类型给通用名,不暴露裸数字。 */
export function vulnSourceTypeLabel(type: VulnSourceType): string {
  return VULN_SOURCE_TYPE_LABELS[type]
}

const VULN_LEVEL_LABELS: Record<VulnLevel, string> = {
  [VulnLevel.A]: 'A 级 · 可自动转链上题',
  [VulnLevel.B]: 'B 级 · 需人工补全',
  [VulnLevel.C]: 'C 级 · 理论素材',
}

/** vulnLevelLabel 返回漏洞可复现性分级文案。 */
export function vulnLevelLabel(level: VulnLevel): string {
  return VULN_LEVEL_LABELS[level]
}

const VULN_RUNTIME_MODE_LABELS: Record<VulnRuntimeMode, string> = {
  [VulnRuntimeMode.ISOLATED]: '干净测试链复现',
  [VulnRuntimeMode.FORKED]: '主网分叉复现',
}

/** vulnRuntimeModeLabel 返回漏洞题运行方式文案。 */
export function vulnRuntimeModeLabel(mode: VulnRuntimeMode): string {
  return VULN_RUNTIME_MODE_LABELS[mode]
}

const VULN_PREVALIDATE_STATUS_LABELS: Record<VulnPrevalidateStatus, string> = {
  [VulnPrevalidateStatus.PENDING]: '尚未验证',
  [VulnPrevalidateStatus.PASSED]: '验证通过',
  [VulnPrevalidateStatus.FAILED]: '验证未通过',
}

/** vulnPrevalidateStatusLabel 返回漏洞题预验证状态文案。 */
export function vulnPrevalidateStatusLabel(status: VulnPrevalidateStatus): string {
  return VULN_PREVALIDATE_STATUS_LABELS[status]
}

const VULN_PROBLEM_STATUS_LABELS: Record<VulnProblemStatus, string> = {
  [VulnProblemStatus.DRAFT]: '草稿',
  [VulnProblemStatus.FINALIZED]: '已固化到题库',
  [VulnProblemStatus.DISCARDED]: '已弃用',
}

/** vulnProblemStatusLabel 返回漏洞题草稿状态文案。 */
export function vulnProblemStatusLabel(status: VulnProblemStatus): string {
  return VULN_PROBLEM_STATUS_LABELS[status]
}

const VULN_CHAIN_OP_LABELS = {
  [VULN_CHAIN_OPERATION.DEPLOY]: '部署合约',
  [VULN_CHAIN_OPERATION.TX]: '发起交易',
  [VULN_CHAIN_OPERATION.RESET]: '重置链状态',
  [VULN_CHAIN_OPERATION.QUERY]: '仅查询(不改状态)',
} as const

/** vulnChainOpLabel 返回链上步骤操作文案。 */
export function vulnChainOpLabel(op: keyof typeof VULN_CHAIN_OP_LABELS): string {
  return VULN_CHAIN_OP_LABELS[op]
}

const VULN_ASSERT_OP_LABELS = {
  [CHAIN_ASSERT_OPERATION.EQ]: '等于期望值',
  [CHAIN_ASSERT_OPERATION.NE]: '不等于期望值',
  [CHAIN_ASSERT_OPERATION.CONTAINS]: '包含期望值',
  [CHAIN_ASSERT_OPERATION.EXISTS]: '该字段存在',
} as const

/** vulnAssertOpLabel 返回断言判定方式文案。 */
export function vulnAssertOpLabel(op: keyof typeof VULN_ASSERT_OP_LABELS): string {
  return VULN_ASSERT_OP_LABELS[op]
}


/**
 * 回放归档里链操作的用户向名称。
 * 取值来自 M3 runChainStep 的封闭集(deploy/tx/reset),归档解析已把集外操作滤掉;
 * 这里仍给未登记值一个中性名,保证界面不出现裸英文 op。
 */
const BATTLE_CHAIN_OP_LABELS: Record<string, string> = {
  [VULN_CHAIN_OPERATION.DEPLOY]: '部署合约',
  [VULN_CHAIN_OPERATION.TX]: '发起交易',
  [VULN_CHAIN_OPERATION.RESET]: '重置链状态',
}

/** battleChainOpLabel 返回链操作文案。 */
export function battleChainOpLabel(op: string): string {
  return BATTLE_CHAIN_OP_LABELS[op] ?? '链上操作'
}
