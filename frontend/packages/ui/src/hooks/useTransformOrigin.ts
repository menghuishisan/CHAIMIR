// useTransformOrigin 把 Floating UI 的 placement 转为可复用的 CSS transform-origin 值。

import { useMemo } from 'react'

/** useTransformOrigin 保证浮层动效从触发源一侧展开。 */
export function useTransformOrigin(placement?: string): string {
  return useMemo(() => {
    const side = placement?.split('-')[0]
    if (side === 'top') return 'bottom center'
    if (side === 'bottom') return 'top center'
    if (side === 'left') return 'right center'
    if (side === 'right') return 'left center'
    return 'center'
  }, [placement])
}
