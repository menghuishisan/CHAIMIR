// usePagedResource 管理分页资源的页码状态与读取,统一四端列表页的分页行为。
// 后端分页契约是 { list, total, page, size }(docs/总-API接口总览.md §2),
// 页大小与后端默认一致(pagex 默认 20、上限 100),页码由本 Hook 持有 ——
// 页面自己管页码容易在筛选变化时忘记回到第一页,把空列表误当成"没数据"。

import { useCallback, useState } from 'react'
import type { DependencyList } from 'react'
import type { PaginatedResponse } from '@chaimir/api-client'
import { useAsyncResource, type AsyncResourceState } from './useAsyncResource'

/** 列表页默认页大小,与后端 pagex.defaultSize 一致。 */
export const DEFAULT_PAGE_SIZE = 20

export interface PagedResourceState<T> extends AsyncResourceState<PaginatedResponse<T>> {
  page: number
  pageSize: number
  /** 后端返回的总记录数;首次读取完成前为 0 */
  total: number
  setPage: (page: number) => void
}

/**
 * usePagedResource 读取分页列表并持有页码。
 * deps 变化(如筛选条件切换)时页码回到第一页:筛选后停在第 5 页往往越界,
 * 后端会回空列表,用户看到的是"没有数据"而不是"这一页没有数据"。
 */
export function usePagedResource<T>(
  loader: (params: { page: number; size: number }) => Promise<PaginatedResponse<T>>,
  deps: DependencyList,
  pageSize: number = DEFAULT_PAGE_SIZE,
): PagedResourceState<T> {
  const [page, setPageState] = useState(1)

  // 筛选条件变化时重置页码:用 key 而不是 useEffect,避免多渲染一帧越界页
  const [depsKey, setDepsKey] = useState(() => JSON.stringify(deps))
  const nextDepsKey = JSON.stringify(deps)
  if (nextDepsKey !== depsKey) {
    setDepsKey(nextDepsKey)
    setPageState(1)
  }

  const resource = useAsyncResource(
    () => loader({ page, size: pageSize }),
    [page, pageSize, nextDepsKey],
    (value) => value.list.length === 0,
  )

  const setPage = useCallback((next: number) => setPageState(Math.max(1, next)), [])

  return {
    ...resource,
    page,
    pageSize,
    total: resource.data ? resource.data.total : 0,
    setPage,
  }
}
