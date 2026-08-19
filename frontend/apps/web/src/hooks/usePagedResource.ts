// usePagedResource 管理分页资源的页码状态与读取,统一四端列表页的分页行为。
// 后端分页契约是 { list, total, page, size }(docs/总-API接口总览.md §2),
// 页大小与后端默认一致(pagex 默认 20、上限 100),页码由本 Hook 持有 ——
// 页面自己管页码容易在筛选变化时忘记回到第一页,把空列表误当成"没数据"。

import { useCallback, useEffect, useState } from 'react'
import type { DependencyList } from 'react'
import { PAGINATION_DEFAULT_SIZE, type PaginatedResponse } from '@chaimir/api-client'
import { useAsyncResource, type AsyncResourceState } from './useAsyncResource'

export interface PagedResourceState<T, R extends PaginatedResponse<T> = PaginatedResponse<T>>
  extends AsyncResourceState<R> {
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
export function usePagedResource<T, R extends PaginatedResponse<T> = PaginatedResponse<T>>(
  loader: (params: { page: number; size: number }) => Promise<R>,
  deps: DependencyList,
  pageSize: number = PAGINATION_DEFAULT_SIZE,
): PagedResourceState<T, R> {
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
    (value) => value.total === 0,
  )

  // 删除或筛选后旧页可能越过最后一页;按服务端 total 回退到最后有效页,
  // 让分页控件保持可见,不把“这一页暂时没有记录”误报成资源为空。
  useEffect(() => {
    if (resource.status !== 'success' || !resource.data || resource.data.total <= 0) return
    if (resource.data.list.length > 0) return
    const lastPage = Math.max(1, Math.ceil(resource.data.total / pageSize))
    if (page > lastPage) setPageState(lastPage)
  }, [page, pageSize, resource.data, resource.status])

  const setPage = useCallback((next: number) => setPageState(Math.max(1, next)), [])

  return {
    ...resource,
    page,
    pageSize,
    total: resource.data ? resource.data.total : 0,
    setPage,
  }
}
