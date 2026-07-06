// Toast 组件：轻量级通知
// 符合 FE-2（aria-live="polite"，不抢焦点）、FE-8（显示 trace_id）

import React, { useEffect } from 'react'
import { CheckCircle, XCircle, Info, AlertCircle, X } from 'lucide-react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
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
  const isError = variant === 'error'

  useEffect(() => {
    if (duration > 0) {
      const timer = setTimeout(() => {
        onClose?.()
      }, duration)

      return () => clearTimeout(timer)
    }
  }, [duration, onClose])

  const classes = clsx('chaimir-toast', `chaimir-toast--${variant}`)

  return (
    <div
      className={classes}
      role={isError ? 'alert' : 'status'}
      aria-live={isError ? 'assertive' : 'polite'}
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
            onClose()
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
    <div className={classes} aria-live="polite" aria-relevant="additions removals">
      {toasts.map((toast) => (
        <Toast key={toast.id} {...toast} onClose={() => onRemove(toast.id)} />
      ))}
    </div>
  )
}

ToastContainer.displayName = 'ToastContainer'
