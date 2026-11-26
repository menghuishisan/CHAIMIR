import { useEffect, useState } from 'react'

/**
 * useActiveAnchor 返回当前锚点 id 与一个立即设置的函数(点目录时不必等滚动结束)。
 * 取视口上部 40% 的第一个锚点，无候选时保留原值。
 * anchorIds 须与页面上的顺序一致;内容相同时每次传新数组也不会重建观察。
 */
export function useActiveAnchor(anchorIds: readonly string[]): readonly [string | undefined, (anchorId: string) => void] {
  const [current, setCurrent] = useState<string | undefined>(anchorIds[0])
  const key = anchorIds.join(' ')

  useEffect(() => {
    const ids = key === '' ? [] : key.split(' ')
    const visible = new Map<string, boolean>()
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) visible.set(entry.target.id, entry.isIntersecting)
        const first = ids.find((id) => visible.get(id))
        if (first) setCurrent(first)
      },
      { rootMargin: '0px 0px -60% 0px' },
    )
    for (const id of ids) {
      const element = document.getElementById(id)
      if (element) observer.observe(element)
    }
    return () => observer.disconnect()
  }, [key])

  return [current, setCurrent] as const
}
