// useRejudgeTask 统一教师侧判题任务重判动作的请求、反馈和进行中状态。

import { useCallback, useState } from 'react'
import { toast } from '@chaimir/ui'
import { api } from '../../app/api'
import { userFacingErrorMessage } from '../../utils/userFacingError'

export interface RejudgeTaskState {
  rejudgingId: string | undefined
  actionError: string | undefined
  rejudge: (taskId: string) => Promise<void>
}

/**
 * useRejudgeTask 提交一次判题任务重判请求。
 * 刷新由调用页面提供,这样列表仍由各自页面的数据资源负责重新读取。
 */
export function useRejudgeTask(onRejudged?: () => void): RejudgeTaskState {
  const [rejudgingId, setRejudgingId] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const rejudge = useCallback(
    async (taskId: string) => {
      setRejudgingId(taskId)
      setActionError(undefined)
      try {
        await api.judge.rejudgeTask(taskId)
        toast.success('已重新提交判题')
        onRejudged?.()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '重判没有成功,请稍后重试。'))
      } finally {
        setRejudgingId(undefined)
      }
    },
    [onRejudged],
  )

  return { rejudgingId, actionError, rejudge }
}
