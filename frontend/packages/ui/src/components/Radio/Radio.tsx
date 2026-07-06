// Radio 组件：单选框

import React from 'react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
import './Radio.css'

export interface RadioOption {
  value: string
  label: React.ReactNode
  disabled?: boolean
}

export interface RadioProps {
  /** 选项列表 */
  options: RadioOption[]
  /** 当前值 */
  value?: string
  /** 默认值 */
  defaultValue?: string
  /** name 属性 */
  name?: string
  /** 错误状态 */
  error?: boolean
  /** 禁用 */
  disabled?: boolean
  /** 变化回调 */
  onChange?: (value: string) => void
  /** 自定义类名 */
  className?: string
}

export const Radio: React.FC<RadioProps> = ({
  options,
  value,
  defaultValue,
  name,
  error = false,
  disabled = false,
  onChange,
  className,
}) => {
  const [selectedValue, setSelectedValue] = React.useState(value || defaultValue || '')

  React.useEffect(() => {
    if (value !== undefined) {
      setSelectedValue(value)
    }
  }, [value])

  const handleChange = (optionValue: string) => {
    if (disabled) return
    triggerHaptic(10)
    setSelectedValue(optionValue)
    onChange?.(optionValue)
  }

  const classes = clsx('chaimir-radio-group', className)

  return (
    <div className={classes} role="radiogroup">
      {options.map((option) => {
        const isChecked = option.value === selectedValue
        const isDisabled = disabled || option.disabled

        const optionClasses = clsx(
          'chaimir-radio',
          isDisabled && 'chaimir-radio--disabled'
        )

        const circleClasses = clsx(
          'chaimir-radio__circle',
          error && 'chaimir-radio__circle--error',
          isChecked && 'chaimir-radio__circle--checked'
        )

        return (
          <label key={option.value} className={optionClasses}>
            <input
              type="radio"
              name={name}
              value={option.value}
              checked={isChecked}
              disabled={isDisabled}
              onChange={() => handleChange(option.value)}
              className="chaimir-radio__input"
            />
            <span className={circleClasses} aria-hidden="true">
              {isChecked && <span className="chaimir-radio__dot" />}
            </span>
            <span className="chaimir-radio__label">{option.label}</span>
          </label>
        )
      })}
    </div>
  )
}

Radio.displayName = 'Radio'
