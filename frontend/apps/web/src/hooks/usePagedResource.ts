// usePagedResource 统一列表分页、URL 页码和筛选切页行为。

import { useCallback, useEffect, useRef, useState } from 'react'
import type { DependencyList } from 'react'
import { useSearchParams } from 'react-router'
import { PAGINATION_DEFAULT_SIZE, type PaginatedResponse } from '@chaimir/api-client'
import { readPage } from '../utils/urlQuery'
import { useAsyncResource, type AsyncResourceState } from './useAsyncResource'

export interface PagedResourceState<
  T,
  R extends PaginatedResponse<T> = PaginatedResponse<T>,
> extends AsyncResourceState<R> {
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

function sameDependencyList(previous: DependencyList, next: DependencyList): boolean {
  return (
    previous.length === next.length &&
    previous.every((value, index) => Object.is(value, next[index]))
  )
}

function usePageNumber(urlPageKey: string | undefined): readonly [number, (page: number) => void] {
  const [searchParams, setSearchParams] = useSearchParams()
  const [localPage, setLocalPage] = useState(1)
  const urlPage = urlPageKey ? readPage(searchParams, urlPageKey) : 1

  const setPage = useCallback(
    (next: number) => {
      const page = Number.isSafeInteger(next) && next > 0 ? next : 1
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
        { replace: true }
      )
    },
    [setSearchParams, urlPageKey]
  )

  return [urlPageKey ? urlPage : localPage, setPage] as const
}

/** 列表页持久化页码到 URL。浮层可使用随筛选重置的本地页码。 */
export function usePagedResource<T, R extends PaginatedResponse<T> = PaginatedResponse<T>>(
  loader: (params: { page: number; size: number }) => Promise<R>,
  deps: DependencyList,
  options: PagedResourceOptions = {}
): PagedResourceState<T, R> {
  const pageSize = options.pageSize ?? PAGINATION_DEFAULT_SIZE
  const [page, setPage] = usePageNumber(options.urlPageKey)
  const previousDepsRef = useRef<DependencyList>(deps)
  const resetPage = !sameDependencyList(previousDepsRef.current, deps)
  const requestPage = resetPage ? 1 : page

  useEffect(() => {
    if (resetPage && page !== 1) {
      // 筛选或路由对象变化时先读取第一页,再同步本地页码或 URL,避免短暂请求旧页。
      setPage(1)
    }
    previousDepsRef.current = deps
    // 依赖列表由调用方显式提供,其值正是资源请求的边界。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, resetPage, setPage, ...deps])

  const resource = useAsyncResource(
    () => loader({ page: requestPage, size: pageSize }),
    [requestPage, pageSize, ...deps],
    (value) => value.total === 0
  )

  // 删除记录或手改地址可能使页码越界,此时回到最后一页。
  useEffect(() => {
    if (resource.status !== 'success' || !resource.data || resource.data.total <= 0) return
    if (resource.data.list.length > 0) return
    const lastPage = Math.max(1, Math.ceil(resource.data.total / pageSize))
    if (requestPage > lastPage) setPage(lastPage)
  }, [pageSize, requestPage, resource.data, resource.status, setPage])

  return {
    ...resource,
    page: requestPage,
    pageSize,
    total:
      resource.status === 'success' || resource.status === 'empty'
        ? resource.data?.total
        : undefined,
    setPage,
  }
}
