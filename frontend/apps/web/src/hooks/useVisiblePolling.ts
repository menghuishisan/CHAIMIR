// useVisiblePolling 在条件成立且页面可见时按固定间隔执行一次检查(设计 §6.7:后台处理中的对象就地更新)。
// 页面切到后台标签页时停止,切回来立即补一次再继续;条件不成立时完全不读。
// 检查本身由调用方决定(通常是静默读一次,状态变了再刷新页面资源,避免每次都闪骨架)。

import { useEffect, useRef } from 'react'

/** useVisiblePolling 在 active 且页面可见期间每 intervalMs 调用一次 tick */
export function useVisiblePolling(active: boolean, tick: () => void, intervalMs: number): void {
  // 始终调用最新的 tick,计时器不因调用方重新渲染而重建
  const tickRef = useRef(tick)
  useEffect(() => {
    tickRef.current = tick
  })

  useEffect(() => {
    if (!active) return
    let timer: number | undefined
    const start = () => {
      if (timer === undefined) timer = window.setInterval(() => tickRef.current(), intervalMs)
    }
    const stop = () => {
      if (timer !== undefined) window.clearInterval(timer)
      timer = undefined
    }
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        tickRef.current()
        start()
      } else {
        stop()
      }
    }
    if (document.visibilityState === 'visible') start()
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
      stop()
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [active, intervalMs])
}
