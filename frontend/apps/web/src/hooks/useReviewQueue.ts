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
  const onListKeyDown = useCallback(
    (event: KeyboardEvent<HTMLElement>) => {
      if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
      event.preventDefault()
      const next = stepQueue(items, currentId, event.key === 'ArrowDown' ? 1 : -1)
      if (next) onSelect(next)
    },
    [currentId, items, onSelect],
  )

  /** 提交后选择下一条待处理项；没有候选时清空选择并返回 false。 */
  const advance = useCallback((): boolean => {
    const next = nextPendingId(items, currentId, isPending)
    onSelect(next)
    return next !== undefined
  }, [currentId, isPending, items, onSelect])

  /** 详情区按 Ctrl/⌘ + 回车时提交。 */
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
