// useAccountNames 按账号编号解析姓名与学工号(设计 §10:校管申诉队列、学业预警、审计、批量成绩单)。
// 账号列表每页最多 100 条,超过 100 个账号时按页取完(collectAllPages),不再只取第一页。
// 解析不到的显示「已移除的账号」,读取中由调用方显示占位,失败时显示「姓名暂时读不到」。

import { useCallback, useMemo } from 'react'
import { PAGINATION_MAX_SIZE, type Account, type BaseIdentity } from '@chaimir/api-client'
import { api } from '../../app/api'
import { useAsyncResource } from '../../hooks'
import { collectAllPages } from '../../utils/allPages'

export type AccountName =
  | { state: 'loading' }
  | { state: 'failed' }
  | { state: 'missing' }
  | { state: 'found'; account: Account }

/**
 * useAccountNames 读取本校全部账号(可按基础身份收窄),返回按编号查账号的函数与全部账号清单。
 * enabled 为 false 时不读(如页面当前没有需要解析的记录)。
 */
export function useAccountNames(role?: BaseIdentity, enabled = true) {
  const accounts = useAsyncResource<Account[]>(
    () => (enabled ? collectAllPages((params) => api.identity.getAccounts({ role, ...params }), PAGINATION_MAX_SIZE) : Promise.resolve([])),
    [role, enabled],
    () => false,
  )
  const byId = useMemo(() => new Map((accounts.data ?? []).map((account) => [account.id, account])), [accounts.data])
  const status = accounts.status
  const lookup = useCallback(
    (accountId: string): AccountName => {
      if (status === 'loading') return { state: 'loading' }
      if (status === 'error') return { state: 'failed' }
      const account = byId.get(accountId)
      return account ? { state: 'found', account } : { state: 'missing' }
    },
    [byId, status],
  )
  return { lookup, list: accounts.data ?? [], status: accounts.status, reload: accounts.reload }
}
