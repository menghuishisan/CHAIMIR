// UnsavedChangesGuard 在有未保存内容时拦住离开(设计 §6.7:长表单和多步骤流程有未保存内容时离开页面要先提醒)。
// 站内跳转弹出确认框(数据路由的 useBlocker,见 app/App.tsx);刷新、关闭标签页或离开本站时
// 浏览器只允许显示它自己的确认框(beforeunload)。只有路径变化才拦:页面自己改地址参数(筛选、标签页)不算离开。

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

export interface UnsavedChangesGuardProps {
  /** 当前是否有未保存内容 */
  when: boolean
  /** 弹窗里的说明:说清现在离开会怎样 */
  description: string
  /** 提供时给出「保存后离开」:返回 true 表示已保存到服务器,随后离开;失败留在本页 */
  onSave?: () => Promise<boolean>
}

/** UnsavedChangesGuard 拦截站内跳转与页面卸载;自身只渲染确认框 */
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

  // 拦下之后内容已经保存完(例如自动保存刚好完成):离开不再丢东西,直接放行
  useEffect(() => {
    if (blocker.state === 'blocked' && !when) blocker.proceed()
  }, [blocker, when])

  const open = blocker.state === 'blocked'

  /** stay 留在本页:取消这次跳转 */
  const stay = () => {
    setSaveFailed(false)
    if (blocker.state === 'blocked') blocker.reset()
  }

  /** leave 不保存,继续这次跳转 */
  const leave = () => {
    setSaveFailed(false)
    if (blocker.state === 'blocked') blocker.proceed()
  }

  /** saveAndLeave 先保存,保存成功才离开 */
  const saveAndLeave = async () => {
    if (!onSave) return
    setSaving(true)
    setSaveFailed(false)
    try {
      const saved = await onSave()
      if (saved) leave()
      else setSaveFailed(true)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal open={open} onOpenChange={(next) => (next ? undefined : stay())}>
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
