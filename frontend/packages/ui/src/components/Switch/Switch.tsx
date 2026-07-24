// Switch 组件：开关

import React from 'react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
import './Switch.css'

export interface SwitchProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'type' | 'size'> {
  /** 标签 */
  label?: React.ReactNode
  /** 尺寸 */
  size?: 'sm' | 'md'
  /** 错误状态 */
  error?: boolean
}

export const Switch = React.forwardRef<HTMLInputElement, SwitchProps>(
  (
    {
      label,
      size = 'md',
      error = false,
      className,
      disabled,
      checked,
      id,
      ...props
    },
    ref
  ) => {
    const generatedId = React.useId()
    const inputId = id || `chaimir-switch-${generatedId}`
    const wrapperClasses = clsx(
      'chaimir-switch-wrapper',
      disabled && 'chaimir-switch-wrapper--disabled',
      className
    )

    const switchClasses = clsx(
      'chaimir-switch',
      `chaimir-switch--${size}`,
      error && 'chaimir-switch--error',
      checked && 'chaimir-switch--checked'
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
          ref={ref}
          type="checkbox"
          className="chaimir-switch__input"
          disabled={disabled}
          checked={checked}
          aria-invalid={error}
          onChange={handleChange}
          {...props}
        />
        <span className={switchClasses} aria-hidden="true">
          <span className="chaimir-switch__thumb" />
        </span>
        {label && <span className="chaimir-switch__label">{label}</span>}
      </label>
    )
  }
)

Switch.displayName = 'Switch'
