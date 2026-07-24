// usePendingAction 为页面写操作提供互斥提交状态。

import { useCallback, useRef, useState } from 'react'

/** usePendingAction 保证同一页面任一写操作完成前不会重复执行。 */
export function usePendingAction() {
  const activeRef = useRef('')
  const [pendingAction, setPendingAction] = useState('')

  const runPendingAction = useCallback(async <T>(key: string, action: () => Promise<T>): Promise<T | undefined> => {
    if (activeRef.current) return undefined
    activeRef.current = key
    setPendingAction(key)
    try {
      return await action()
    } finally {
      activeRef.current = ''
      setPendingAction('')
    }
  }, [])

  return { pendingAction, runPendingAction }
}
