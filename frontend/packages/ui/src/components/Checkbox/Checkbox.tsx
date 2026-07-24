// Checkbox 组件：复选框
// 符合 FE-2（无障碍）、FE-4（label 显式关联）

import React, { useEffect, useId, useImperativeHandle, useRef } from 'react'
import { Check } from 'lucide-react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
import './Checkbox.css'

export interface CheckboxProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'type' | 'children'> {
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
      id,
      ...props
    },
    ref
  ) => {
    const inputRef = useRef<HTMLInputElement>(null)
    const generatedId = useId()
    const inputId = id || `chaimir-checkbox-${generatedId}`

    useImperativeHandle(ref, () => inputRef.current as HTMLInputElement)

    useEffect(() => {
      if (inputRef.current) {
        inputRef.current.indeterminate = indeterminate
      }
    }, [indeterminate])

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

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
      triggerHaptic(10)
      if (props.onChange) {
        props.onChange(e)
      }
    }

    return (
      <label className={wrapperClasses} htmlFor={inputId}>
        <input
          id={inputId}
          ref={inputRef}
          type="checkbox"
          className="chaimir-checkbox__input"
          disabled={disabled}
          checked={checked}
          aria-checked={indeterminate ? 'mixed' : checked}
          aria-invalid={error}
          onChange={handleChange}
          {...props}
        />
        <span className={boxClasses} aria-hidden="true">
          {indeterminate ? (
            <span className="chaimir-checkbox__dash" />
          ) : (
            <Check size={14} strokeWidth={3} className="chaimir-checkbox__icon" />
          )}
        </span>
        {label && <span className="chaimir-checkbox__label">{label}</span>}
      </label>
    )
  }
)

Checkbox.displayName = 'Checkbox'
