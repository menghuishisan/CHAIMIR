// contest rules 文件维护竞赛页面共享的业务判断。

import { ContestStatus } from '@chaimir/api-client'

/** 封榜期榜单不再更新,页面据此提示用户当前名次可能与最终结果不同。 */
export function isContestLeaderboardFrozen(status: ContestStatus): boolean {
  return status === ContestStatus.FROZEN
}
