// useAsyncResource 管理页面级异步资源读取，统一加载、空态、错误和刷新状态。

import { useCallback, useEffect, useMemo, useState } from 'react'
import type { DependencyList } from 'react'
import { ApiError } from '@chaimir/api-client'
import { RESOURCE_LOAD_FAILED_MESSAGE } from '../utils/userFacingError'

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
 * normalizeError 在边界处把未知异常收敛成用户向错误对象。
 * loader 内部除 API 错误外还可能抛出运行时异常(如渲染前的数据转换失败),
 * 这类异常的原文不可展示给用户(§8 错误暴露分层),故在此换成用户向文案;
 * 没有后端 trace_id 时不自造编号,页面据此不显示报障编号。
 */
function normalizeError(error: unknown): ApiError {
  return error instanceof ApiError ? error : new ApiError(RESOURCE_LOAD_FAILED_MESSAGE)
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
    // 调用方显式传入依赖列表，语义与 useEffect 的依赖参数一致。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [version, ...deps])

  return useMemo(() => ({ ...state, reload }), [reload, state])
}
