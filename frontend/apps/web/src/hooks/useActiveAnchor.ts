// useActiveAnchor 按滚动位置给出当前所在的锚点(作业答题卡的「当前题」、配置页左侧分组目录的高亮共用)。
// 视口上部 40% 区域里的第一个锚点即当前;该区域里没有锚点(正在读一段很长的内容)时保持上一个。
// 用 IntersectionObserver,不监听 scroll 事件:纸面是自己的滚动容器,observer 不关心是谁在滚。

import { useEffect, useState } from 'react'

/**
 * useActiveAnchor 返回当前锚点 id 与一个立即设置的函数(点目录时不必等滚动结束)。
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
