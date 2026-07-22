// contest labels 文件维护竞赛、对抗和漏洞题领域的枚举文案。

import { BattleRole, ContestMode, ContestStatus, MatchMode, TeamMode, VulnLevel, VulnRuntimeMode } from '@chaimir/api-client'
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

/** vulnLevelLabel 返回漏洞题等级文案。 */
export function vulnLevelLabel(level: VulnLevel): string {
  return labelFromMap(level, { [VulnLevel.A]: 'A', [VulnLevel.B]: 'B', [VulnLevel.C]: 'C' }, '未识别的漏洞等级')
}

/** vulnRuntimeModeLabel 返回漏洞题运行方式文案。 */
export function vulnRuntimeModeLabel(mode: VulnRuntimeMode): string {
  return labelFromMap(mode, { [VulnRuntimeMode.ISOLATED]: '隔离环境', [VulnRuntimeMode.FORKED]: '主网分叉' }, '未识别的运行方式')
}
