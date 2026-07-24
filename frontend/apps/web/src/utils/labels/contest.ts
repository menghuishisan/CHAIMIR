// contest labels 文件维护竞赛、对抗和漏洞题领域的枚举文案。

import { BattleMatchStatus, BattleResult, BattleRole, BattleRule, CheatAction, CheatType, ContestMode, ContestStatus, MatchMode, TeamMode, VulnLevel, VulnPrevalidateStatus, VulnProblemStatus, VulnRuntimeMode } from '@chaimir/api-client'
import { labelFromMap } from './map'

/** contestStatusLabel 返回竞赛生命周期状态文案。 */
export function contestStatusLabel(status: ContestStatus, draftLabel = '草稿'): string {
  return labelFromMap(status, {
    [ContestStatus.DRAFT]: draftLabel,
    [ContestStatus.SIGNUP]: '报名中',
    [ContestStatus.RUNNING]: '进行中',
    [ContestStatus.FROZEN]: '封榜中',
    [ContestStatus.ENDED]: '已结束',
    [ContestStatus.ARCHIVED]: '已归档',
  }, draftLabel)
}

/** contestModeLabel 返回竞赛模式文案。 */
export function contestModeLabel(mode: ContestMode): string {
  return labelFromMap(mode, { [ContestMode.SOLVE]: '解题赛', [ContestMode.BATTLE]: '对抗赛' }, '未识别的竞赛模式')
}

/** teamModeLabel 返回竞赛组队方式文案。 */
export function teamModeLabel(mode: TeamMode): string {
  return labelFromMap(mode, { [TeamMode.SOLO]: '个人参赛', [TeamMode.GROUP]: '团队参赛' }, '未识别的组队方式')
}

/** matchModeLabel 返回对抗赛匹配模式文案。 */
export function matchModeLabel(mode: MatchMode): string {
  return labelFromMap(mode, { [MatchMode.ROUND_ROBIN]: '循环赛', [MatchMode.ELO]: '积分匹配' }, '未识别的对局模式')
}

/** battleRoleLabel 返回对抗赛参战角色文案。 */
export function battleRoleLabel(role: BattleRole): string {
  return labelFromMap(role, { [BattleRole.ATTACK]: '攻击', [BattleRole.DEFENSE]: '防守', [BattleRole.STRATEGY]: '策略' }, '未识别的参战角色')
}

/** battleRuleLabel 返回题目级对局规则文案。 */
export function battleRuleLabel(rule: BattleRule): string {
  return labelFromMap(rule, { [BattleRule.ATTACK_DEFENSE]: '攻防对局', [BattleRule.GAME]: '策略博弈' }, '未识别的对局规则')
}

/** cheatTypeLabel 返回违规线索类别文案。 */
export function cheatTypeLabel(type: CheatType): string {
  return labelFromMap(type, { [CheatType.SIMILARITY]: '代码相似', [CheatType.BEHAVIOR]: '行为异常', [CheatType.ENVIRONMENT]: '环境异常' }, '未识别的线索类型')
}

/** cheatActionLabel 返回违规处理方式文案。 */
export function cheatActionLabel(action: CheatAction): string {
  return labelFromMap(action, { [CheatAction.WARN]: '警告', [CheatAction.PENALTY]: '扣分', [CheatAction.DISQUALIFY]: '取消资格' }, '未识别的处理方式')
}

/** vulnLevelLabel 返回漏洞题等级文案。 */
export function vulnLevelLabel(level: VulnLevel): string {
  return labelFromMap(level, { [VulnLevel.A]: 'A', [VulnLevel.B]: 'B', [VulnLevel.C]: 'C' }, '未识别的漏洞等级')
}

/** vulnRuntimeModeLabel 返回漏洞题运行方式文案。 */
export function vulnRuntimeModeLabel(mode: VulnRuntimeMode): string {
  return labelFromMap(mode, { [VulnRuntimeMode.ISOLATED]: '隔离环境', [VulnRuntimeMode.FORKED]: '主网分叉' }, '未识别的运行方式')
}

/** battleMatchStatusLabel 返回对抗对局状态文案。 */
export function battleMatchStatusLabel(status: BattleMatchStatus): string {
  return labelFromMap(status, { [BattleMatchStatus.PENDING]: '等待匹配', [BattleMatchStatus.RUNNING]: '对局进行中', [BattleMatchStatus.DONE]: '已完成', [BattleMatchStatus.FAILED]: '执行失败' }, '未识别的对局状态')
}

/** battleResultLabel 返回对抗赛赛果文案。 */
export function battleResultLabel(result: BattleResult): string {
  return labelFromMap(result, { [BattleResult.A_WIN]: '甲方获胜', [BattleResult.B_WIN]: '乙方获胜', [BattleResult.DRAW]: '平局' }, '未识别的赛果')
}

/** vulnPrevalidateStatusLabel 返回漏洞题双向预验证状态文案。 */
export function vulnPrevalidateStatusLabel(status: VulnPrevalidateStatus): string {
  return labelFromMap(status, { [VulnPrevalidateStatus.PENDING]: '待验证', [VulnPrevalidateStatus.PASSED]: '验证通过', [VulnPrevalidateStatus.FAILED]: '验证未通过' }, '未识别的验证状态')
}

/** vulnProblemStatusLabel 返回漏洞题转化状态文案。 */
export function vulnProblemStatusLabel(status: VulnProblemStatus): string {
  return labelFromMap(status, { [VulnProblemStatus.DRAFT]: '草稿', [VulnProblemStatus.FINALIZED]: '已固化', [VulnProblemStatus.DISCARDED]: '已废弃' }, '未识别的漏洞题状态')
}
