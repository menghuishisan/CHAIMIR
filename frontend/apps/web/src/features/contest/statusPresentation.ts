// contest statusPresentation 文件维护竞赛状态到界面语义色的映射。

import type { BadgeTone, StatusTone } from '@chaimir/ui'
import {
  BattleMatchStatus,
  CheatAction,
  ContestStatus,
  TeamStatus,
  VulnLevel,
  VulnPrevalidateStatus,
  VulnProblemStatus,
} from '@chaimir/api-client'

const CONTEST_STATUS_TONES: Record<ContestStatus, StatusTone> = {
  [ContestStatus.DRAFT]: 'neutral',
  [ContestStatus.SIGNUP]: 'info',
  [ContestStatus.RUNNING]: 'primary',
  [ContestStatus.FROZEN]: 'warning',
  [ContestStatus.ENDED]: 'success',
  [ContestStatus.ARCHIVED]: 'neutral',
}

/** contestStatusTone 返回竞赛状态语义色。 */
export function contestStatusTone(status: ContestStatus): StatusTone {
  return CONTEST_STATUS_TONES[status]
}

const TEAM_STATUS_TONES: Record<TeamStatus, StatusTone> = {
  [TeamStatus.BUILDING]: 'info',
  [TeamStatus.LOCKED]: 'success',
}

/** teamStatusTone 返回队伍状态语义色。 */
export function teamStatusTone(status: TeamStatus): StatusTone {
  return TEAM_STATUS_TONES[status]
}

const BATTLE_MATCH_STATUS_TONES: Record<BattleMatchStatus, StatusTone> = {
  [BattleMatchStatus.PENDING]: 'neutral',
  [BattleMatchStatus.RUNNING]: 'primary',
  [BattleMatchStatus.DONE]: 'success',
  [BattleMatchStatus.FAILED]: 'danger',
}

/** battleMatchStatusTone 返回对局状态语义色。 */
export function battleMatchStatusTone(status: BattleMatchStatus): StatusTone {
  return BATTLE_MATCH_STATUS_TONES[status]
}

const CHEAT_ACTION_TONES: Record<CheatAction, BadgeTone> = {
  [CheatAction.WARN]: 'warning',
  [CheatAction.PENALTY]: 'danger',
  [CheatAction.DISQUALIFY]: 'danger',
}

/** cheatActionTone 返回违规处理方式语义色。 */
export function cheatActionTone(action: CheatAction): BadgeTone {
  return CHEAT_ACTION_TONES[action]
}

const VULN_LEVEL_TONES: Record<VulnLevel, BadgeTone> = {
  [VulnLevel.A]: 'success',
  [VulnLevel.B]: 'warning',
  [VulnLevel.C]: 'neutral',
}

/** vulnLevelTone 返回漏洞分级语义色。 */
export function vulnLevelTone(level: VulnLevel): BadgeTone {
  return VULN_LEVEL_TONES[level]
}

const VULN_PREVALIDATE_STATUS_TONES: Record<VulnPrevalidateStatus, StatusTone> = {
  [VulnPrevalidateStatus.PENDING]: 'neutral',
  [VulnPrevalidateStatus.PASSED]: 'success',
  [VulnPrevalidateStatus.FAILED]: 'danger',
}

/** vulnPrevalidateStatusTone 返回漏洞题预验证状态语义色。 */
export function vulnPrevalidateStatusTone(status: VulnPrevalidateStatus): StatusTone {
  return VULN_PREVALIDATE_STATUS_TONES[status]
}

const VULN_PROBLEM_STATUS_TONES: Record<VulnProblemStatus, StatusTone> = {
  [VulnProblemStatus.DRAFT]: 'neutral',
  [VulnProblemStatus.FINALIZED]: 'success',
  [VulnProblemStatus.DISCARDED]: 'neutral',
}

/** vulnProblemStatusTone 返回漏洞题草稿状态语义色。 */
export function vulnProblemStatusTone(status: VulnProblemStatus): StatusTone {
  return VULN_PROBLEM_STATUS_TONES[status]
}
