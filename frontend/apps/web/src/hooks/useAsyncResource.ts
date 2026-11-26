import { useCallback, useEffect, useMemo, useState } from 'react'
import type { DependencyList } from 'react'
import { ApiError } from '@chaimir/api-client'
import { errorDiagnostics, RESOURCE_LOAD_FAILED_MESSAGE } from '../utils/userFacingError'

type ResourceStatus = 'loading' | 'success' | 'empty' | 'error'

export interface AsyncResourceState<T> {
  status: ResourceStatus
  data: T | null
  error: ApiError | null
  reload: () => void
  /** 仅在显式reload时递增,供同范围统计随列表刷新,不随翻页或筛选变化。 */
  refreshVersion: number
}

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

/** 同一范围刷新时保留数据；切换范围后只暴露新范围的结果。 */
export function useAsyncResource<T>(
  loader: () => Promise<T>,
  deps: DependencyList,
  isEmpty: (value: T) => boolean = isDefaultEmpty
): AsyncResourceState<T> {
  const [version, setVersion] = useState(0)
  // 范围由调用方声明；显式刷新不改变数据归属。
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const scopeKey = useMemo(() => ({}), deps)
  const requestKey = useMemo(() => ({ scopeKey, version }), [scopeKey, version])
  const [state, setState] = useState<{
    scopeKey: object
    requestKey: object
    status: ResourceStatus
    data: T | null
    error: ApiError | null
  }>({ scopeKey, requestKey, status: 'loading', data: null, error: null })

  const reload = useCallback(() => {
    setVersion((current) => current + 1)
  }, [])

  useEffect(() => {
    let active = true
    const load = async () => {
      try {
        const data = await loader()
        if (!active) return
        setState({ scopeKey, requestKey, status: isEmpty(data) ? 'empty' : 'success', data, error: null })
      } catch (cause) {
        if (!active) return
        if (!(cause instanceof ApiError)) console.error('资源读取失败', errorDiagnostics(cause))
        const error = cause instanceof ApiError ? cause : new ApiError(RESOURCE_LOAD_FAILED_MESSAGE)
        setState({ scopeKey, requestKey, status: 'error', data: null, error })
      }
    }
    void load()

    return () => {
      active = false
    }
    // 请求归属已包含调用方显式声明的依赖和刷新次数。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [requestKey, scopeKey])

  const current = state.requestKey === requestKey
  return {
    status: current ? state.status : 'loading',
    data: state.scopeKey === scopeKey ? state.data : null,
    error: current ? state.error : null,
    reload,
    refreshVersion: version,
  }
}
