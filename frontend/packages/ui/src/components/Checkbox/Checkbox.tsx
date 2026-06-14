// Checkbox 组件：复选框
// 符合 FE-2（无障碍）、FE-4（label 显式关联）

import React from 'react'
import { Check } from 'lucide-react'
import { clsx } from 'clsx'
import './Checkbox.css'

export interface CheckboxProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'type'> {
  /** 标签文字 */
  label?: React.ReactNode
  /** 错误状态 */
  error?: boolean
  /** 不确定状态 */
  indeterminate?: boolean
}

export const Checkbox = React.forwardRef<HTMLInputElement, CheckboxProps>(
  (
    {
      label,
      error = false,
      indeterminate = false,
      className,
      disabled,
      checked,
      ...props
    },
    ref
  ) => {
    const wrapperClasses = clsx(
      'chaimir-checkbox-wrapper',
      disabled && 'chaimir-checkbox-wrapper--disabled',
      className
    )

    const boxClasses = clsx(
      'chaimir-checkbox__box',
      error && 'chaimir-checkbox__box--error',
      (checked || indeterminate) && 'chaimir-checkbox__box--checked'
    )

    return (
      <label className={wrapperClasses}>
        <input
          ref={ref}
          type="checkbox"
          className="chaimir-checkbox__input"
          disabled={disabled}
          checked={checked}
          aria-invalid={error}
          {...props}
        />
        <span className={boxClasses} aria-hidden="true">
          {(checked || indeterminate) && (
            <Check size={14} strokeWidth={3} className="chaimir-checkbox__icon" />
          )}
        </span>
        {label && <span className="chaimir-checkbox__label">{label}</span>}
      </label>
    )
  }
)

Checkbox.displayName = 'Checkbox'
