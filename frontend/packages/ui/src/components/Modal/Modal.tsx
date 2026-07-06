// Modal 组件：模态对话框
// 符合 FE-2（焦点陷阱、Esc 关闭、scrim 遮罩）

import React, { useEffect, useId, useRef } from 'react'
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
  const previousActiveElement = useRef<HTMLElement | null>(null)
  const titleId = useId()
  const touchStartY = useRef(0)
  const currentTranslateY = useRef(0)
  const [swipeStyle, setSwipeStyle] = React.useState<React.CSSProperties>({})

  // FE-2: 焦点陷阱
  useEffect(() => {
    if (open) {
      // 保存之前的焦点元素
      previousActiveElement.current = document.activeElement as HTMLElement

      // 优先聚焦第一个可操作元素；纯展示弹窗再聚焦容器。
      const firstFocusable = modalRef.current ? getFocusableElements(modalRef.current)[0] : undefined
      firstFocusable?.focus()
      if (!firstFocusable) {
        modalRef.current?.focus()
      }

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
      setSwipeStyle({})
    }
  }, [open])

  // FE-2: Esc 关闭
  useEffect(() => {
    if (!open) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
        return
      }
      if (e.key === 'Tab') {
        trapFocus(e, modalRef.current)
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

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
      setSwipeStyle({ transform: `translateY(0)`, transition: 'transform 0.4s var(--ease-spring-bouncy)' })
    }
    currentTranslateY.current = 0
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
}

Modal.displayName = 'Modal'

/**
 * trapFocus 让 Tab/Shift+Tab 在当前模态框内循环，避免焦点逃逸到页面底层。
 */
function trapFocus(event: KeyboardEvent, container: HTMLElement | null): void {
  if (!container) {
    return
  }
  const focusable = getFocusableElements(container)
  if (focusable.length === 0) {
    event.preventDefault()
    container.focus()
    return
  }

  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
    return
  }
  if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

/**
 * getFocusableElements 获取模态框内可见的交互元素。
 */
function getFocusableElements(container: HTMLElement): HTMLElement[] {
  const selector = [
    'a[href]',
    'button:not([disabled])',
    'input:not([disabled])',
    'select:not([disabled])',
    'textarea:not([disabled])',
    '[tabindex]:not([tabindex="-1"])',
  ].join(',')
  return Array.from(container.querySelectorAll<HTMLElement>(selector)).filter((element) => element.offsetParent !== null)
}
