// contest 定义竞赛领域页面使用的选择项。

import { BattleRole, ContestMode, MatchMode, TeamMode, VulnLevel, VulnRuntimeMode } from '@chaimir/api-client'
import { battleRoleLabel, contestModeLabel, matchModeLabel, teamModeLabel, vulnLevelLabel, vulnRuntimeModeLabel } from '../labels'
import { option } from './shared'

export const contestModeOptions = [option(ContestMode.SOLVE, contestModeLabel(ContestMode.SOLVE)), option(ContestMode.BATTLE, contestModeLabel(ContestMode.BATTLE))]
export const teamModeOptions = [option(TeamMode.SOLO, teamModeLabel(TeamMode.SOLO)), option(TeamMode.GROUP, teamModeLabel(TeamMode.GROUP))]
export const matchModeOptions = [option(MatchMode.ROUND_ROBIN, matchModeLabel(MatchMode.ROUND_ROBIN)), option(MatchMode.ELO, matchModeLabel(MatchMode.ELO))]
export const battleRoleOptions = [option(BattleRole.ATTACK, battleRoleLabel(BattleRole.ATTACK)), option(BattleRole.DEFENSE, battleRoleLabel(BattleRole.DEFENSE)), option(BattleRole.STRATEGY, battleRoleLabel(BattleRole.STRATEGY))]
export const vulnLevelOptions = [option(VulnLevel.A, vulnLevelLabel(VulnLevel.A)), option(VulnLevel.B, vulnLevelLabel(VulnLevel.B)), option(VulnLevel.C, vulnLevelLabel(VulnLevel.C))]
export const vulnRuntimeModeOptions = [option(VulnRuntimeMode.ISOLATED, vulnRuntimeModeLabel(VulnRuntimeMode.ISOLATED)), option(VulnRuntimeMode.FORKED, vulnRuntimeModeLabel(VulnRuntimeMode.FORKED))]
