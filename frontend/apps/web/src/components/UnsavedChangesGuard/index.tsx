import { useCallback, useEffect, useState } from 'react'
import { useBlocker, type BlockerFunction } from 'react-router'
import {
  Button,
  Callout,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
} from '@chaimir/ui'
import { errorDiagnostics } from '../../utils/userFacingError'

export interface UnsavedChangesGuardProps {
  /** 当前是否有未保存内容 */
  when: boolean
  /** 弹窗里的说明:说清现在离开会怎样 */
  description: string
  /** 提供时给出「保存后离开」:返回 true 表示已保存到服务器,随后离开;失败留在本页 */
  onSave?: () => Promise<boolean>
}

/** 拦截离开未保存页面；同路径的筛选与标签页变化不算离开。 */
export function UnsavedChangesGuard({ when, description, onSave }: UnsavedChangesGuardProps) {
  const shouldBlock = useCallback<BlockerFunction>(
    ({ currentLocation, nextLocation }) =>
      when && currentLocation.pathname !== nextLocation.pathname,
    [when]
  )
  const blocker = useBlocker(shouldBlock)
  const [saving, setSaving] = useState(false)
  const [saveFailed, setSaveFailed] = useState(false)

  // 刷新、关闭标签页或离开本站:只能交给浏览器自带的确认框
  useEffect(() => {
    if (!when) return
    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      // 仍有浏览器要求写 returnValue 才弹确认
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [when])

  // 手动保存自己负责放行，避免 dirty 先更新时重复继续同一跳转。
  useEffect(() => {
    if (blocker.state === 'blocked' && !when && !saving) blocker.proceed()
  }, [blocker, when, saving])

  const open = blocker.state === 'blocked'

  const stay = () => {
    setSaveFailed(false)
    if (blocker.state === 'blocked') blocker.reset()
  }

  const leave = () => {
    setSaveFailed(false)
    if (blocker.state === 'blocked') blocker.proceed()
  }

  const saveAndLeave = async () => {
    if (!onSave || saving) return
    setSaving(true)
    setSaveFailed(false)
    try {
      const saved = await onSave()
      if (saved) leave()
      else setSaveFailed(true)
    } catch (error) {
      console.error('离开页面前保存失败', errorDiagnostics(error))
      setSaveFailed(true)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(next) => {
        if (!next && !saving) stay()
      }}
    >
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>还有改动没有保存</ModalTitle>
          <ModalDescription>{description}</ModalDescription>
        </ModalHeader>
        {saveFailed ? (
          <ModalBody>
            <Callout tone="danger">
              这次没能保存到服务器,请检查网络后重试,或选择不保存直接离开。
            </Callout>
          </ModalBody>
        ) : null}
        <ModalFooter>
          <Button variant="ghost" onClick={leave} disabled={saving}>
            不保存,离开
          </Button>
          <Button variant="outline" onClick={stay} disabled={saving}>
            留在本页
          </Button>
          {onSave ? (
            <Button variant="primary" loading={saving} onClick={() => void saveAndLeave()}>
              保存后离开
            </Button>
          ) : null}
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
