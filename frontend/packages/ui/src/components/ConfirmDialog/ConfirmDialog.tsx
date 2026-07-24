// ConfirmDialog 提供全应用统一的异步确认流程，替代阻塞式浏览器确认框。
import React, { createContext, useCallback, useContext, useEffect, useId, useMemo, useRef, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Button } from '../Button'
import { Modal } from '../Modal'
import './ConfirmDialog.css'

export interface ConfirmOptions {
  title: string
  description: string
  confirmLabel?: string
  cancelLabel?: string
  confirmVariant?: 'primary' | 'danger'
}

type ConfirmFunction = (options: ConfirmOptions) => Promise<boolean>

interface PendingConfirmation extends ConfirmOptions {
  resolve: (confirmed: boolean) => void
}

const ConfirmContext = createContext<ConfirmFunction | null>(null)

export interface ConfirmProviderProps {
  children: React.ReactNode
}

/** ConfirmProvider 在应用根部托管唯一确认对话框，并保证卸载时释放等待中的调用。 */
export function ConfirmProvider({ children }: ConfirmProviderProps): React.ReactElement {
  const [pending, setPending] = useState<PendingConfirmation | null>(null)
  const pendingRef = useRef<PendingConfirmation | null>(null)
  const descriptionId = useId()

  useEffect(() => {
    pendingRef.current = pending
  }, [pending])

  useEffect(() => () => {
    pendingRef.current?.resolve(false)
  }, [])

  /** confirm 关闭仍未完成的旧请求后，打开新的非阻塞确认对话框。 */
  const confirm = useCallback<ConfirmFunction>((options) => new Promise<boolean>((resolve) => {
    setPending((current) => {
      current?.resolve(false)
      return { ...options, resolve }
    })
  }), [])

  /** settle 只完成当前请求一次，并同步关闭对话框。 */
  const settle = useCallback((confirmed: boolean) => {
    setPending((current) => {
      current?.resolve(confirmed)
      return null
    })
  }, [])

  const contextValue = useMemo(() => confirm, [confirm])

  return (
    <ConfirmContext.Provider value={contextValue}>
      {children}
      <Modal
        open={Boolean(pending)}
        onClose={() => settle(false)}
        title={pending?.title}
        ariaDescribedBy={descriptionId}
        closeOnOverlayClick={false}
        size="sm"
        footer={(
          <>
            <Button variant="ghost" onClick={() => settle(false)}>{pending?.cancelLabel || '取消'}</Button>
            <Button variant={pending?.confirmVariant || 'danger'} onClick={() => settle(true)}>
              {pending?.confirmLabel || '确认'}
            </Button>
          </>
        )}
      >
        <div className="chaimir-confirm-dialog__content">
          <AlertTriangle size={20} aria-hidden="true" />
          <p id={descriptionId}>{pending?.description}</p>
        </div>
      </Modal>
    </ConfirmContext.Provider>
  )
}

/** useConfirm 返回异步确认函数，调用方可在原业务流程中直接等待用户选择。 */
export function useConfirm(): ConfirmFunction {
  const confirm = useContext(ConfirmContext)
  if (!confirm) throw new Error('useConfirm 必须在 ConfirmProvider 内使用')
  return confirm
}
