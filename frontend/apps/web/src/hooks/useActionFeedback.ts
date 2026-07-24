// useActionFeedback 统一页面写操作的互斥提交、成功提示和用户向错误反馈。

import { useCallback, useState } from 'react'
import { userFacingErrorMessage } from '../utils/userFacingError'
import { usePendingAction } from './usePendingAction'

/** useActionFeedback 复用页面写操作共同流程，不包含任何领域判断。 */
export function useActionFeedback(reload: () => void, fallbackMessage: string) {
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()

  const runAction = useCallback(async (key: string, action: () => Promise<unknown>, successMessage: string): Promise<boolean> => {
    setError('')
    setMessage('')
    const completed = await runPendingAction(key, async () => {
      try {
        await action()
        setMessage(successMessage)
        reload()
        return true
      } catch (actionError) {
        setError(userFacingErrorMessage(actionError, fallbackMessage))
        return false
      }
    })
    return completed === true
  }, [fallbackMessage, reload, runPendingAction])

  return { error, message, pendingAction, runAction }
}
