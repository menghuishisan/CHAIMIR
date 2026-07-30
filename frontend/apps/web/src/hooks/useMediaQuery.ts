// useMediaQuery 订阅媒体查询结果,供壳层在断点间切换侧栏形态(常驻/抽屉)。

import { useSyncExternalStore } from 'react'

/**
 * useMediaQuery 返回查询当前是否命中;窗口缩放实时响应。
 */
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (callback) => {
      const mql = window.matchMedia(query)
      mql.addEventListener('change', callback)
      return () => mql.removeEventListener('change', callback)
    },
    () => window.matchMedia(query).matches,
  )
}
