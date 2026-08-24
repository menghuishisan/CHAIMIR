// contest rules 文件维护竞赛页面共享的业务判断。

import {
  BattleRole,
  BattleRule,
  ContestStatus,
  SANDBOX_ACCESS_PROFILE,
  type ContestBattleRuntimeConfig,
} from '@chaimir/api-client'

/** 封榜期榜单不再更新,页面据此提示用户当前名次可能与最终结果不同。 */
export function isContestLeaderboardFrozen(status: ContestStatus): boolean {
  return status === ContestStatus.FROZEN
}

/**
 * battleEntryRoles 返回某个对抗规则下可以入场的角色。
 * 撮合器只接受「攻 vs 守」或「策略 vs 策略」,故入场角色由规则唯一决定 ——
 * 让教师另选一遍只会造出撮合器永远配不上的组合。
 */
export function battleEntryRoles(rule: BattleRule): BattleRole[] {
  return rule === BattleRule.GAME
    ? [BattleRole.STRATEGY]
    : [BattleRole.ATTACK, BattleRole.DEFENSE]
}

/**
 * battleRuntimeConfig 组装对抗题的赛制参数。
 * 环境组合不在这里 —— 它由题库锁定版本的组合声明唯一提供,竞赛侧只决定「怎么打」。
 * 回放取平台唯一支持的完整记录模式(键与平台目录基线一致)。
 */
export function battleRuntimeConfig(rule: BattleRule): ContestBattleRuntimeConfig {
  return {
    execution_profile: SANDBOX_ACCESS_PROFILE.CONTEST_BATTLE,
    entry_roles: battleEntryRoles(rule),
    replay_profile: { mode: 'full' },
  }
}
