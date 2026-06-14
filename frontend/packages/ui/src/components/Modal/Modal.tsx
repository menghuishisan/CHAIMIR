// Modal 组件：模态对话框
// 符合 FE-2（焦点陷阱、Esc 关闭、scrim 遮罩）

import React, { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { clsx } from 'clsx'
import './Modal.css'

export interface ModalProps {
  /** 是否打开 */
  open: boolean
  /** 关闭回调 */
  onClose: () => void
  /** 标题 */
  title?: React.ReactNode
  /** 尺寸 */
  size?: 'sm' | 'md' | 'lg' | 'xl'
  /** 是否显示关闭按钮 */
  showClose?: boolean
  /** 点击遮罩是否关闭 */
  closeOnOverlayClick?: boolean
  /** 子元素 */
  children?: React.ReactNode
  /** 底部操作区 */
  footer?: React.ReactNode
  /** 自定义类名 */
  className?: string
}

export const Modal: React.FC<ModalProps> = ({
  open,
  onClose,
  title,
  size = 'md',
  showClose = true,
  closeOnOverlayClick = true,
  children,
  footer,
  className,
}) => {
  const modalRef = useRef<HTMLDivElement>(null)
  const previousActiveElement = useRef<HTMLElement | null>(null)

  // FE-2: 焦点陷阱
  useEffect(() => {
    if (open) {
      // 保存之前的焦点元素
      previousActiveElement.current = document.activeElement as HTMLElement

      // 聚焦到 modal
      modalRef.current?.focus()

      // 禁用 body 滚动
      document.body.style.overflow = 'hidden'
    } else {
      // 恢复之前的焦点
      previousActiveElement.current?.focus()

      // 恢复 body 滚动
      document.body.style.overflow = ''
    }

    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  // FE-2: Esc 关闭
  useEffect(() => {
    if (!open) return

    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }

    document.addEventListener('keydown', handleEscape)
    return () => document.removeEventListener('keydown', handleEscape)
  }, [open, onClose])

  if (!open) return null

  const handleOverlayClick = (e: React.MouseEvent) => {
    if (closeOnOverlayClick && e.target === e.currentTarget) {
      onClose()
    }
  }

  const modalClasses = clsx(
    'chaimir-modal',
    `chaimir-modal--${size}`,
    className
  )

  return (
    <div className="chaimir-modal-overlay" onClick={handleOverlayClick}>
      <div
        ref={modalRef}
        className={modalClasses}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? 'modal-title' : undefined}
        tabIndex={-1}
      >
        {(title || showClose) && (
          <div className="chaimir-modal__header">
            {title && <h2 id="modal-title" className="chaimir-modal__title">{title}</h2>}
            {showClose && (
              <button
                type="button"
                className="chaimir-modal__close"
                onClick={onClose}
                aria-label="关闭对话框"
              >
                <X size={20} />
              </button>
            )}
          </div>
        )}

        <div className="chaimir-modal__body">
          {children}
        </div>

        {footer && (
          <div className="chaimir-modal__footer">
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}

Modal.displayName = 'Modal'
