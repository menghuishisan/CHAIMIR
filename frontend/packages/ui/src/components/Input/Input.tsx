// Input 组件：文本输入框
// 符合 FE-2（无障碍）、FE-4（文案面向用户）

import React, { useState } from 'react'
import { Eye, EyeOff, Loader2 } from 'lucide-react'
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
  /** 加载状态，用于异步校验或远程搜索 */
  loading?: boolean
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
      loading = false,
      fullWidth = false,
      className,
      disabled,
      readOnly,
      type,
      ...props
    },
    ref
  ) => {
    const [passwordVisible, setPasswordVisible] = useState(false)
    const isPassword = type === 'password'
    const wrapperClasses = clsx(
      'chaimir-input-wrapper',
      `chaimir-input-wrapper--${size}`,
      error && 'chaimir-input-wrapper--error',
      disabled && 'chaimir-input-wrapper--disabled',
      readOnly && 'chaimir-input-wrapper--readonly',
      loading && 'chaimir-input-wrapper--loading',
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
          readOnly={readOnly}
          aria-invalid={error}
          aria-busy={loading || undefined}
          type={isPassword && passwordVisible ? 'text' : type}
          {...props}
        />
        {loading && (
          <span className="chaimir-input__icon chaimir-input__icon--right" aria-hidden="true">
            <Loader2 className="chaimir-input__spinner" size={16} />
          </span>
        )}
        {!loading && rightIcon && (
          <span className="chaimir-input__icon chaimir-input__icon--right" aria-hidden="true">
            {rightIcon}
          </span>
        )}
        {!loading && isPassword && (
          <button
            type="button"
            className="chaimir-input__password-toggle"
            aria-label={passwordVisible ? '隐藏密码' : '显示密码'}
            aria-pressed={passwordVisible}
            disabled={disabled}
            onClick={() => setPasswordVisible((visible) => !visible)}
          >
            {passwordVisible ? <EyeOff size={16} /> : <Eye size={16} />}
          </button>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'
