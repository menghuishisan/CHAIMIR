// usePagedResource 管理分页资源的页码状态与读取,统一四端列表页的分页行为。
// 后端分页契约是 { list, total, page, size }(docs/总-API接口总览.md §2),
// 页大小与后端默认一致(pagex 默认 20、上限 100),页码由本 Hook 持有 ——
// 页面自己管页码容易在筛选变化时忘记回到第一页,把空列表误当成"没数据"。
// 列表页传 urlPageKey,页码写进地址(规范 §6.2),刷新与返回都回到同一页;
// 弹窗、抽屉里的列表不传,页码留在组件内 —— 关掉浮层就该忘掉它。

import { useCallback, useEffect, useState } from 'react'
import type { DependencyList } from 'react'
import { useSearchParams } from 'react-router'
import { PAGINATION_DEFAULT_SIZE, type PaginatedResponse } from '@chaimir/api-client'
import { readPage } from '../utils/urlQuery'
import { useAsyncResource, type AsyncResourceState } from './useAsyncResource'

export interface PagedResourceState<T, R extends PaginatedResponse<T> = PaginatedResponse<T>>
  extends AsyncResourceState<R> {
  page: number
  pageSize: number
  /** 当前请求成功返回的总记录数;读取中或失败时未知,不能按零展示。 */
  total: number | undefined
  setPage: (page: number) => void
}

export interface PagedResourceOptions {
  /** 每页条数,默认与后端一致(20) */
  pageSize?: number
  /** 把页码写进地址的参数名(通常是 page);不传时页码只在组件内 */
  urlPageKey?: string
}

/**
 * usePageNumber 持有页码:给了 urlPageKey 时读写地址参数(替换历史记录),否则是组件内状态。
 * 两个分支都无条件调用同样的 Hook,只是按参数决定采用哪一边的值。
 */
function usePageNumber(urlPageKey: string | undefined): readonly [number, (page: number) => void] {
  const [searchParams, setSearchParams] = useSearchParams()
  const [localPage, setLocalPage] = useState(1)
  const urlPage = urlPageKey ? readPage(searchParams, urlPageKey) : 1

  const setPage = useCallback(
    (next: number) => {
      const page = Math.max(1, Math.floor(next))
      if (!urlPageKey) {
        setLocalPage(page)
        return
      }
      setSearchParams(
        (previous) => {
          const params = new URLSearchParams(previous)
          if (page === 1) params.delete(urlPageKey)
          else params.set(urlPageKey, String(page))
          return params
        },
        { replace: true },
      )
    },
    [setSearchParams, urlPageKey],
  )

  return [urlPageKey ? urlPage : localPage, setPage] as const
}

/**
 * usePagedResource 读取分页列表并持有页码。
 * deps 变化(如筛选条件切换)时页码回到第一页:筛选后停在第 5 页往往越界,
 * 后端会回空列表,用户看到的是"没有数据"而不是"这一页没有数据"。
 * 页码写进地址时,筛选变化清页码由 useUrlState 在同一次地址更新里完成,不在这里重复写地址。
 */
export function usePagedResource<T, R extends PaginatedResponse<T> = PaginatedResponse<T>>(
  loader: (params: { page: number; size: number }) => Promise<R>,
  deps: DependencyList,
  options: PagedResourceOptions = {},
): PagedResourceState<T, R> {
  const pageSize = options.pageSize ?? PAGINATION_DEFAULT_SIZE
  const [page, setPage] = usePageNumber(options.urlPageKey)

  // 筛选条件变化时把组件内页码重置为 1:用 key 而不是 useEffect,避免多渲染一帧越界页。
  // 页码在地址里时由调用方的筛选更新顺带清掉,这里不动地址。
  const [depsKey, setDepsKey] = useState(() => JSON.stringify(deps))
  const nextDepsKey = JSON.stringify(deps)
  if (nextDepsKey !== depsKey) {
    setDepsKey(nextDepsKey)
    if (!options.urlPageKey && page !== 1) setPage(1)
  }

  const resource = useAsyncResource(
    () => loader({ page, size: pageSize }),
    [page, pageSize, nextDepsKey],
    (value) => value.total === 0,
  )

  // 删除、筛选或手改地址后旧页可能越过最后一页;按服务端 total 回退到最后有效页,
  // 让分页控件保持可见,不把“这一页暂时没有记录”误报成资源为空。
  useEffect(() => {
    if (resource.status !== 'success' || !resource.data || resource.data.total <= 0) return
    if (resource.data.list.length > 0) return
    const lastPage = Math.max(1, Math.ceil(resource.data.total / pageSize))
    if (page > lastPage) setPage(lastPage)
  }, [page, pageSize, resource.data, resource.status, setPage])

  return {
    ...resource,
    page,
    pageSize,
    total: resource.status === 'success' || resource.status === 'empty' ? resource.data?.total : undefined,
    setPage,
  }
}
