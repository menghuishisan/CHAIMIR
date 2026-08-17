// useResourceTotal 取某个筛选口径下的服务端总记录数,专供指标带使用。
// 指标带必须是全量口径(规范 §6.5):用当前页返回的 20 行统计出来的「已发布 8」,
// 在总量 41 条时就是错数,加「本页」前缀也只是把错数合法化。
// 后端分页信封的 total 与 size 无关(docs/总-API接口总览.md §2),
// 因此这里只要一页 size=1 就能拿到准确总数,不必把整表拉回浏览器再数。

import type { DependencyList } from 'react'
import type { PaginatedResponse } from '@chaimir/api-client'
import { useAsyncResource } from './useAsyncResource'

/** 指标带取数用的最小页:只为拿 total,不消费列表内容。 */
const COUNT_PROBE = { page: 1, size: 1 } as const

/**
 * useResourceTotal 返回服务端总数;读取中或读取失败时返回 undefined。
 * 调用方按 `total ?? '—'` 渲染:数不出来就不显示数字,不用 0 冒充「一条都没有」。
 */
export function useResourceTotal(
  loader: (params: { page: number; size: number }) => Promise<PaginatedResponse<unknown>>,
  deps: DependencyList,
): number | undefined {
  // isEmpty 恒为 false:这里只关心 total,空列表不是需要渲染空态的资源
  const resource = useAsyncResource(() => loader(COUNT_PROBE), deps, () => false)
  return resource.data ? resource.data.total : undefined
}
