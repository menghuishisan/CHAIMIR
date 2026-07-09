// useAsyncResource 管理页面级异步资源读取，统一加载、空态、错误和刷新状态。

import { useCallback, useEffect, useMemo, useState } from 'react'
import type { DependencyList } from 'react'
import type { ApiError } from '@chaimir/api-client'

type ResourceStatus = 'loading' | 'success' | 'empty' | 'error'

export interface AsyncResourceState<T> {
  status: ResourceStatus
  data: T | null
  error: ApiError | null
  reload: () => void
}

/**
 * isDefaultEmpty 判断常见数组和分页列表响应是否为空。
 */
function isDefaultEmpty<T>(value: T): boolean {
  if (Array.isArray(value)) {
    return value.length === 0
  }
  if (value && typeof value === 'object' && 'items' in value) {
    const items = (value as { items?: unknown }).items
    return Array.isArray(items) && items.length === 0
  }
  if (value && typeof value === 'object' && 'list' in value) {
    const list = (value as { list?: unknown }).list
    return Array.isArray(list) && list.length === 0
  }
  return false
}

/**
 * normalizeError 把未知异常转换成页面可展示的用户向错误对象。
 */
function normalizeError(error: unknown): ApiError {
  if (error && typeof error === 'object' && 'message' in error) {
    return error as ApiError
  }
  return { message: '暂时无法获取数据，请稍后重试' }
}

/**
 * useAsyncResource 在组件挂载或依赖变化时读取后端资源。
 */
export function useAsyncResource<T>(
  loader: () => Promise<T>,
  deps: DependencyList,
  isEmpty: (value: T) => boolean = isDefaultEmpty
): AsyncResourceState<T> {
  const [version, setVersion] = useState(0)
  const [state, setState] = useState<Omit<AsyncResourceState<T>, 'reload'>>({
    status: 'loading',
    data: null,
    error: null,
  })

  const reload = useCallback(() => {
    setVersion((current) => current + 1)
  }, [])

  useEffect(() => {
    let active = true
    setState((current) => ({
      status: 'loading',
      data: current.data,
      error: null,
    }))

    loader()
      .then((data) => {
        if (!active) {
          return
        }
        setState({
          status: isEmpty(data) ? 'empty' : 'success',
          data,
          error: null,
        })
      })
      .catch((error) => {
        if (!active) {
          return
        }
        setState({
          status: 'error',
          data: null,
          error: normalizeError(error),
        })
      })

    return () => {
      active = false
    }
  }, [...deps, version])

  return useMemo(() => ({ ...state, reload }), [reload, state])
}
