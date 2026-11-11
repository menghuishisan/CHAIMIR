// useReviewQueue 审阅队列的键盘与「提交后进下一条」(设计 §6 QueueDetailLayout):
// 队列列表上按上下方向键切换当前条;详情里按 Ctrl/⌘ + 回车提交当前条。
// 批改、成绩审核、申诉、实验报告四处队列共用,移动规则见 utils/reviewQueue.ts。

import { useCallback, type KeyboardEvent } from 'react'
import { nextPendingId, stepQueue } from '../utils/reviewQueue'

export interface ReviewQueueOptions<T extends { id: string }> {
  items: readonly T[]
  currentId: string | undefined
  onSelect: (id: string | undefined) => void
  /** 哪些条目还待处理(提交后跳到下一条待处理) */
  isPending: (item: T) => boolean
}

/** useReviewQueue 返回队列列表的键盘处理、详情的提交快捷键处理,以及「跳到下一条待处理」 */
export function useReviewQueue<T extends { id: string }>({ items, currentId, onSelect, isPending }: ReviewQueueOptions<T>) {
  /** onListKeyDown 队列列表上的上下方向键 */
  const onListKeyDown = useCallback(
    (event: KeyboardEvent<HTMLElement>) => {
      if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
      event.preventDefault()
      const next = stepQueue(items, currentId, event.key === 'ArrowDown' ? 1 : -1)
      if (next) onSelect(next)
    },
    [currentId, items, onSelect],
  )

  /** advance 提交后跳到下一条待处理;返回是否还有待处理的条目 */
  const advance = useCallback((): boolean => {
    const next = nextPendingId(items, currentId, isPending)
    onSelect(next)
    return next !== undefined
  }, [currentId, isPending, items, onSelect])

  /** submitShortcut 返回详情区的键盘处理:Ctrl/⌘ + 回车时调用 submit */
  const submitShortcut = useCallback(
    (submit: () => void) => (event: KeyboardEvent<HTMLElement>) => {
      if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault()
        submit()
      }
    },
    [],
  )

  return { onListKeyDown, advance, submitShortcut }
}
