// useHoverPointer 判断当前设备是否具备精细指针和真实 hover 能力。

import { useEffect, useState } from 'react'

function readCapability(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(hover: hover) and (pointer: fine)').matches
}

/** useHoverPointer 避免把 hover-only 交互错误地带到触屏设备。 */
export function useHoverPointer(): boolean {
  const [canHover, setCanHover] = useState(readCapability)

  useEffect(() => {
    const media = window.matchMedia('(hover: hover) and (pointer: fine)')
    const update = (event: MediaQueryListEvent): void => setCanHover(event.matches)
    setCanHover(media.matches)
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [])

  return canHover
}
