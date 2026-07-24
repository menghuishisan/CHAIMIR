// useDelayedUnmount 为需要退场过渡的浮层保留 DOM，并在 reduced-motion 下立即卸载。

import { useEffect, useState } from 'react'
import { useReducedMotion } from './useReducedMotion'

export type PresenceState = 'open' | 'closed'

/** useDelayedUnmount 返回浮层是否保持挂载以及当前进出场状态。 */
export function useDelayedUnmount(open: boolean, exitDurationMs: number): { mounted: boolean; state: PresenceState } {
  const reducedMotion = useReducedMotion()
  const [mounted, setMounted] = useState(open)

  useEffect(() => {
    if (open) {
      setMounted(true)
      return
    }
    if (reducedMotion) {
      setMounted(false)
      return
    }
    const timer = window.setTimeout(() => setMounted(false), exitDurationMs)
    return () => window.clearTimeout(timer)
  }, [exitDurationMs, open, reducedMotion])

  return { mounted, state: open ? 'open' : 'closed' }
}
