import { useCallback, useEffect, useRef, useState } from 'react'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import type { SandboxWorkspaceHandle } from './types'

/** 写入与后续业务动作串行；离开工作台后不再续跑尚未发出的动作。 */
export function useWorkspaceActions() {
  const workspaceRef = useRef<SandboxWorkspaceHandle>(null)
  const activeRef = useRef(true)
  const busyRef = useRef(false)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string>()

  useEffect(() => {
    activeRef.current = true
    return () => { activeRef.current = false }
  }, [])

  const perform = useCallback(async (action: () => void | Promise<void>, archive = false) => {
    if (!activeRef.current || busyRef.current) return
    busyRef.current = true
    setBusy(true)
    setActionError(undefined)
    try {
      const saved = !workspaceRef.current || await workspaceRef.current.save(archive)
      if (!activeRef.current) return
      if (!saved) {
        setActionError('工作区尚未保存,请在代码面板处理后重试。')
        return
      }
      await action()
    } catch (error) {
      if (activeRef.current) setActionError(userFacingErrorMessage(error, '这个操作没有完成,请稍后重试。'))
    } finally {
      busyRef.current = false
      if (activeRef.current) setBusy(false)
    }
  }, [])

  return { workspaceRef, busy, actionError, setActionError, perform }
}
