// Toast 组件：轻量级通知
// 符合 FE-2（aria-live="polite"，不抢焦点）、FE-8（显示 trace_id）

import React, { useCallback, useEffect, useRef, useState } from 'react'
import { CheckCircle, XCircle, Info, AlertCircle, X } from 'lucide-react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
import { useReducedMotion } from '../../hooks'
import { motionDurationMs } from '../../tokens'
import './Toast.css'

export interface ToastProps {
  /** 唯一标识 */
  id: string
  /** 标题 */
  title?: string
  /** 描述 */
  description?: string
  /** 类型 */
  variant?: 'success' | 'error' | 'warning' | 'info'
  /** 持续时间（毫秒），0 表示不自动关闭 */
  duration?: number
  /** trace_id（错误时显示） */
  traceId?: string
  /** 关闭回调 */
  onClose?: () => void
}

const variantIcons = {
  success: CheckCircle,
  error: XCircle,
  warning: AlertCircle,
  info: Info,
}

export const Toast: React.FC<ToastProps> = ({
  id: _id,
  title,
  description,
  variant = 'info',
  duration = 5000,
  traceId,
  onClose,
}) => {
  const Icon = variantIcons[variant]
  const reducedMotion = useReducedMotion()
  const [closing, setClosing] = useState(false)
  const closeTimer = useRef<number>()
  const autoCloseTimer = useRef<number>()
  const remainingDuration = useRef(duration)
  const startedAt = useRef(0)

  /** requestClose 先进入可中断的退场状态，再通知容器移除消息。 */
  const requestClose = useCallback(() => {
    if (closing) return
    window.clearTimeout(autoCloseTimer.current)
    if (reducedMotion) {
      onClose?.()
      return
    }
    setClosing(true)
    closeTimer.current = window.setTimeout(() => onClose?.(), motionDurationMs.toastExit)
  }, [closing, onClose, reducedMotion])

  useEffect(() => {
    if (duration <= 0) return
    remainingDuration.current = duration

    const schedule = (): void => {
      if (document.hidden || remainingDuration.current <= 0) return
      startedAt.current = Date.now()
      autoCloseTimer.current = window.setTimeout(requestClose, remainingDuration.current)
    }
    const handleVisibilityChange = (): void => {
      window.clearTimeout(autoCloseTimer.current)
      if (document.hidden) {
        remainingDuration.current = Math.max(0, remainingDuration.current - (Date.now() - startedAt.current))
      } else {
        schedule()
      }
    }

    schedule()
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => {
      window.clearTimeout(autoCloseTimer.current)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [duration, requestClose])

  useEffect(() => () => window.clearTimeout(closeTimer.current), [])

  const classes = clsx('chaimir-toast', `chaimir-toast--${variant}`)

  return (
    <div
      className={classes}
      data-state={closing ? 'closed' : 'open'}
      role="status"
      aria-live="polite"
      aria-atomic="true"
    >
      <div className="chaimir-toast__icon" aria-hidden="true">
        <Icon size={20} />
      </div>

      <div className="chaimir-toast__content">
        {title && <div className="chaimir-toast__title">{title}</div>}
        {description && <div className="chaimir-toast__description">{description}</div>}
        {traceId && (
          <div className="chaimir-toast__trace">
            编号：<code>{traceId}</code>
          </div>
        )}
      </div>

      {onClose && (
        <button
          type="button"
          className="chaimir-toast__close"
          onClick={() => {
            triggerHaptic(10)
            requestClose()
          }}
          aria-label="关闭通知"
        >
          <X size={16} />
        </button>
      )}
    </div>
  )
}

Toast.displayName = 'Toast'

// ToastContainer 组件：Toast 容器
export interface ToastContainerProps {
  toasts: ToastProps[]
  onRemove: (id: string) => void
  position?: 'top-right' | 'top-center' | 'bottom-right' | 'bottom-center'
}

export const ToastContainer: React.FC<ToastContainerProps> = ({
  toasts,
  onRemove,
  position = 'top-right',
}) => {
  const classes = clsx('chaimir-toast-container', `chaimir-toast-container--${position}`)

  return (
    <div className={classes} aria-label="通知消息">
      {toasts.map((toast) => (
        <Toast key={toast.id} {...toast} onClose={() => onRemove(toast.id)} />
      ))}
    </div>
  )
}

ToastContainer.displayName = 'ToastContainer'
