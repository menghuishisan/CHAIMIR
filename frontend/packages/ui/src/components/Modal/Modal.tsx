// Modal 组件：模态对话框
// 符合 FE-2（焦点陷阱、Esc 关闭、scrim 遮罩）

import React, { useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import { clsx } from 'clsx'
import { useEscapeKey, useFocusTrap } from '../../hooks'
import './Modal.css'

export interface ModalProps {
  /** 是否打开 */
  open: boolean
  /** 关闭回调 */
  onClose: () => void
  /** 标题 */
  title?: React.ReactNode
  /** 无标题时的对话框说明 */
  ariaLabel?: string
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
  ariaLabel,
  size = 'md',
  showClose = true,
  closeOnOverlayClick = true,
  children,
  footer,
  className,
}) => {
  const modalRef = useRef<HTMLDivElement>(null)
  const titleId = useId()
  const touchStartY = useRef(0)
  const currentTranslateY = useRef(0)
  const [swipeStyle, setSwipeStyle] = React.useState<React.CSSProperties>({})

  // 快捷键与焦点陷阱
  useFocusTrap(modalRef, open)
  useEscapeKey(() => {
    if (open) onClose()
  }, open)

  // 禁用 body 滚动
  useEffect(() => {
    if (open) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }

    return () => {
      document.body.style.overflow = ''
      setSwipeStyle({})
    }
  }, [open])

  if (!open) return null

  const handleOverlayClick = (e: React.MouseEvent) => {
    if (closeOnOverlayClick && e.target === e.currentTarget) {
      onClose()
    }
  }

  const handleTouchStart = (e: React.TouchEvent) => {
    touchStartY.current = e.touches[0].clientY
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    const deltaY = e.touches[0].clientY - touchStartY.current
    if (deltaY > 0) {
      currentTranslateY.current = deltaY
      setSwipeStyle({ transform: `translateY(${deltaY}px)`, transition: 'none' })
    }
  }

  const handleTouchEnd = () => {
    if (currentTranslateY.current > 80) {
      onClose()
    } else {
      setSwipeStyle({ transform: `translateY(0)`, transition: 'transform 0.4s var(--ease-spring)' })
    }
    currentTranslateY.current = 0
  }

  const modalClasses = clsx(
    'chaimir-modal',
    `chaimir-modal--${size}`,
    className
  )

  const content = (
    <div className="chaimir-modal-overlay" onClick={handleOverlayClick}>
      <div
        ref={modalRef}
        className={modalClasses}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? titleId : undefined}
        aria-label={!title ? ariaLabel ?? '对话框' : undefined}
        tabIndex={-1}
        style={swipeStyle}
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
      >
        {(title || showClose) && (
          <div className="chaimir-modal__header">
            {title && <h2 id={titleId} className="chaimir-modal__title">{title}</h2>}
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

  return typeof document !== 'undefined' ? createPortal(content, document.body) : null
}

Modal.displayName = 'Modal'
