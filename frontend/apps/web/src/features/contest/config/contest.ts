// contest.ts 定义竞赛配置页使用的后端请求默认值。

import { ContestMode, MatchMode, TeamMode, type ContestRequest } from '@chaimir/api-client'

/**
 * defaultContestRequest 提供新建竞赛草稿的默认后端请求结构。
 */
export const defaultContestRequest: ContestRequest = {
  name: '',
  mode: ContestMode.SOLVE,
  match_mode: MatchMode.ROUND_ROBIN,
  team_mode: TeamMode.SOLO,
  signup_start: '',
  signup_end: '',
  start_at: '',
  end_at: '',
  freeze_minutes: 30,
  rules: {},
}
