// useMediaQuery 统一订阅浏览器媒体查询，避免组件重复管理 matchMedia 监听。

import { useEffect, useState } from 'react'

/** useMediaQuery 返回当前视口是否匹配指定媒体查询。 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => (
    typeof window !== 'undefined' && window.matchMedia(query).matches
  ))

  useEffect(() => {
    const media = window.matchMedia(query)
    const update = (event: MediaQueryListEvent): void => setMatches(event.matches)
    setMatches(media.matches)
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [query])

  return matches
}
