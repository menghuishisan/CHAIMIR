// 回放归档统一经一次性授权取件，正文校验由 battleReplayArchive 负责。

import type { BattleReplayArchive } from '@chaimir/api-client'
import { api } from '../../app/api'
import { parseBattleReplayArchive } from './battleReplayArchive'

/** 每次取件重新签发授权，不缓存短时一次性凭据。 */
export async function readBattleReplayArchive(matchId: string): Promise<BattleReplayArchive> {
  const grant = await api.contest.issueBattleReplayDownloadGrant(matchId)
  const file = await api.storage.consumeGrant(grant.token)
  return parseBattleReplayArchive(await file.blob.text())
}
