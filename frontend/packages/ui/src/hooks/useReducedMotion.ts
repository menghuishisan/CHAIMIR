// useReducedMotion 订阅用户的系统减弱动效偏好，供需要在逻辑层裁剪动效的组件使用。

import { useEffect, useState } from 'react'

function readPreference(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/** useReducedMotion 返回当前用户是否要求减少动效。 */
export function useReducedMotion(): boolean {
  const [reducedMotion, setReducedMotion] = useState(readPreference)

  useEffect(() => {
    const media = window.matchMedia('(prefers-reduced-motion: reduce)')
    const update = (event: MediaQueryListEvent): void => setReducedMotion(event.matches)
    setReducedMotion(media.matches)
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [])

  return reducedMotion
}
