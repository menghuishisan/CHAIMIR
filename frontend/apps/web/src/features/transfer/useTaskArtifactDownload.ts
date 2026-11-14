// useTaskArtifactDownload 取回导入导出任务的产物文件:每次重新签发一次性授权,不复用旧 token。
// 任务与下载页和顶栏任务弹出层共用这一份,取件只有一条路径。

import { useCallback, useState } from 'react'
import type { TransferTask } from '@chaimir/api-client'
import { toast } from '@chaimir/ui'
import { api } from '../../app/api'
import { downloadAttachment } from '../../utils/downloadAttachment'
import { userFacingErrorMessage } from '../../utils/userFacingError'

export interface TaskArtifactDownload {
  /** 正在取件的任务 id(供按钮显示加载态) */
  downloadingId: string | undefined
  /** 最近一次取件失败的用户向文案;新的取件开始时清空 */
  downloadError: string | undefined
  download: (task: TransferTask) => Promise<void>
}

/** useTaskArtifactDownload 返回取件动作与它的进行中 / 失败状态 */
export function useTaskArtifactDownload(): TaskArtifactDownload {
  const [downloadingId, setDownloadingId] = useState<string>()
  const [downloadError, setDownloadError] = useState<string>()

  const download = useCallback(async (task: TransferTask) => {
    setDownloadingId(task.task_id)
    setDownloadError(undefined)
    try {
      const file = await api.transfer.downloadArtifact(task.task_id)
      downloadAttachment(file)
      toast.success('文件已开始下载')
    } catch (error) {
      setDownloadError(userFacingErrorMessage(error, '下载没有完成,请稍后重试。'))
    } finally {
      setDownloadingId(undefined)
    }
  }, [])

  return { downloadingId, downloadError, download }
}
