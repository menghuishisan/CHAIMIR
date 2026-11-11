// SubmitConfirmModal 提交作业的确认弹窗(设计 §7.2 作业作答):列出未作答的题号(可点跳回)与剩余提交次数;
// 提交失败的原因就显示在弹窗里,不会被弹窗挡住。「确认提交」是朱砂落印按钮(提交作答属落印类动作)。

import { useRef } from 'react'
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

export interface SubmitConfirmModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  total: number
  /** 未作答的题,按题号顺序 */
  unanswered: { id: string; seq: number }[]
  /** 这次提交之后还能再交几次 */
  remainingAfter: number
  late: boolean
  latePolicyText: string
  submitting: boolean
  error: string | undefined
  onConfirm: () => void
  /** 关闭弹窗并跳到某题 */
  onJump: (itemId: string) => void
}

/** SubmitConfirmModal 渲染提交确认 */
export function SubmitConfirmModal({
  open,
  onOpenChange,
  total,
  unanswered,
  remainingAfter,
  late,
  latePolicyText,
  submitting,
  error,
  onConfirm,
  onJump,
}: SubmitConfirmModalProps) {
  // 点了未作答题号:先关弹窗,等弹窗把焦点交还时改为跳到那题(否则焦点会回到「提交作业」按钮)
  const pendingJump = useRef<string | null>(null)

  return (
    <Modal open={open} onOpenChange={(next) => (submitting ? undefined : onOpenChange(next))}>
      <ModalContent
        size="sm"
        onCloseAutoFocus={(event) => {
          const target = pendingJump.current
          if (target === null) return
          pendingJump.current = null
          event.preventDefault()
          onJump(target)
        }}
      >
        <ModalHeader>
          <ModalTitle>提交作业</ModalTitle>
          <ModalDescription>
            已作答 {total - unanswered.length} / {total} 题。
            {remainingAfter > 0
              ? `提交后还能再交 ${remainingAfter} 次。`
              : '这是最后一次提交机会,交了就不能再改。'}
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-3">
          {unanswered.length > 0 ? (
            <div>
              <p className="text-sm text-ink">这些题还没有作答,提交后按零分计:</p>
              <div className="mt-2 flex flex-wrap gap-2">
                {unanswered.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => {
                      pendingJump.current = item.id
                      onOpenChange(false)
                    }}
                    className="rounded-md bg-surface-hover px-2.5 py-1 text-sm tabular-nums text-ink transition-colors duration-fast hover:bg-primary-soft hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                  >
                    第 {item.seq} 题
                  </button>
                ))}
              </div>
            </div>
          ) : null}
          {late ? (
            <Callout tone="warning">已过截止时间,这次提交算迟交({latePolicyText})。</Callout>
          ) : null}
          {error ? <Callout tone="danger">{error}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" disabled={submitting} onClick={() => onOpenChange(false)}>
            再检查一下
          </Button>
          <Button variant="seal" loading={submitting} onClick={onConfirm}>
            确认提交
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
