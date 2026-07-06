// Button 组件：所有按钮交互的基础组件
// 符合 FE-1（全令牌化）、FE-2（无障碍）、FE-3（禁用表情符号，使用 Lucide 图标）

import React from 'react'
import { Loader2 } from 'lucide-react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
import './Button.css'

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  /** 按钮变体 */
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger' | 'on-dark'
  /** 按钮尺寸 */
  size?: 'sm' | 'md' | 'lg'
  /** 加载状态 */
  loading?: boolean
  /** 图标（使用 Lucide） */
  icon?: React.ReactNode
  /** 图标位置 */
  iconPosition?: 'left' | 'right'
  /** 块级按钮（占满容器宽度） */
  block?: boolean
  /** 子元素 */
  children?: React.ReactNode
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      variant = 'primary',
      size = 'md',
      loading = false,
      icon,
      iconPosition = 'left',
      block = false,
      disabled,
      children,
      className,
      type = 'button',
      ...props
    },
    ref
  ) => {
    const classes = clsx(
      'chaimir-btn',
      `chaimir-btn--${variant}`,
      `chaimir-btn--${size}`,
      block && 'chaimir-btn--block',
      loading && 'chaimir-btn--loading',
      className
    )

    const handleClick = (e: React.MouseEvent<HTMLButtonElement>) => {
      if (!disabled && !loading) {
        triggerHaptic(10)
      }
      if (props.onClick) {
        props.onClick(e)
      }
    }

    return (
      <button
        ref={ref}
        type={type}
        className={classes}
        disabled={disabled || loading}
        aria-busy={loading}
        onClick={handleClick}
        {...props}
      >
        {loading && (
          <Loader2 className="chaimir-btn__spinner" size={16} aria-hidden="true" />
        )}
        {!loading && icon && iconPosition === 'left' && (
          <span className="chaimir-btn__icon" aria-hidden="true">
            {icon}
          </span>
        )}
        {children && <span className="chaimir-btn__text">{children}</span>}
        {!loading && icon && iconPosition === 'right' && (
          <span className="chaimir-btn__icon" aria-hidden="true">
            {icon}
          </span>
        )}
      </button>
    )
  }
)

Button.displayName = 'Button'
