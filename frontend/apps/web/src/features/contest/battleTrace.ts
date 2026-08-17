// 对局回放归档的受控取件。
//
// 走「确认可用 → 签发一次性授权 → 统一文件服务取件」三步,不把对象存储地址交给浏览器
// (docs/前端后端功能对齐清单.md §对抗回放)。正文的协议校验在 battleReplayArchive.ts。

import type { BattleReplayArchive } from '@chaimir/api-client'
import { api } from '../../app/api'
import { parseBattleReplayArchive } from './battleReplayArchive'

/**
 * readBattleReplayArchive 取回并校验一局的回放归档。
 * 每次都重新签发授权:授权是一次性短时凭据,缓存它等于把一张随时会过期的票留在内存里。
 */
export async function readBattleReplayArchive(matchId: string): Promise<BattleReplayArchive> {
  const grant = await api.contest.issueBattleReplayDownloadGrant(matchId)
  const file = await api.storage.consumeGrant(grant.token)
  return parseBattleReplayArchive(await file.blob.text())
}
