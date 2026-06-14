// Input 组件：文本输入框
// 符合 FE-2（无障碍）、FE-4（文案面向用户）

import React from 'react'
import { clsx } from 'clsx'
import './Input.css'

export interface InputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size'> {
  /** 输入框尺寸 */
  size?: 'sm' | 'md' | 'lg'
  /** 错误状态 */
  error?: boolean
  /** 左侧图标 */
  leftIcon?: React.ReactNode
  /** 右侧图标 */
  rightIcon?: React.ReactNode
  /** 完整宽度 */
  fullWidth?: boolean
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  (
    {
      size = 'md',
      error = false,
      leftIcon,
      rightIcon,
      fullWidth = false,
      className,
      disabled,
      ...props
    },
    ref
  ) => {
    const wrapperClasses = clsx(
      'chaimir-input-wrapper',
      `chaimir-input-wrapper--${size}`,
      error && 'chaimir-input-wrapper--error',
      disabled && 'chaimir-input-wrapper--disabled',
      fullWidth && 'chaimir-input-wrapper--full',
      className
    )

    return (
      <div className={wrapperClasses}>
        {leftIcon && (
          <span className="chaimir-input__icon chaimir-input__icon--left" aria-hidden="true">
            {leftIcon}
          </span>
        )}
        <input
          ref={ref}
          className="chaimir-input"
          disabled={disabled}
          aria-invalid={error}
          {...props}
        />
        {rightIcon && (
          <span className="chaimir-input__icon chaimir-input__icon--right" aria-hidden="true">
            {rightIcon}
          </span>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'
