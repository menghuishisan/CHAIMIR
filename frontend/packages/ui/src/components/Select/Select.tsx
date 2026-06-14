// Select 组件：下拉选择
// 符合 FE-2（无障碍）、FE-4（文案面向用户）

import React, { useState, useRef, useEffect } from 'react'
import { ChevronDown, Check } from 'lucide-react'
import { clsx } from 'clsx'
import './Select.css'

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

export interface SelectProps {
  /** 选项列表 */
  options: SelectOption[]
  /** 当前值 */
  value?: string
  /** 默认值 */
  defaultValue?: string
  /** 提示文字 */
  placeholder?: string
  /** 尺寸 */
  size?: 'sm' | 'md' | 'lg'
  /** 禁用 */
  disabled?: boolean
  /** 错误状态 */
  error?: boolean
  /** 完整宽度 */
  fullWidth?: boolean
  /** 变化回调 */
  onChange?: (value: string) => void
  /** 自定义类名 */
  className?: string
}

export const Select = React.forwardRef<HTMLDivElement, SelectProps>(
  (
    {
      options,
      value,
      defaultValue,
      placeholder = '请选择',
      size = 'md',
      disabled = false,
      error = false,
      fullWidth = false,
      onChange,
      className,
    },
    _ref
  ) => {
    const [isOpen, setIsOpen] = useState(false)
    const [selectedValue, setSelectedValue] = useState(value || defaultValue || '')
    const containerRef = useRef<HTMLDivElement>(null)

    // 监听 value 变化
    useEffect(() => {
      if (value !== undefined) {
        setSelectedValue(value)
      }
    }, [value])

    // 点击外部关闭下拉
    useEffect(() => {
      if (!isOpen) return

      const handleClickOutside = (event: MouseEvent | TouchEvent) => {
        const el = containerRef.current
        if (!el || el.contains(event.target as Node)) {
          return
        }
        setIsOpen(false)
      }

      document.addEventListener('mousedown', handleClickOutside)
      document.addEventListener('touchstart', handleClickOutside)

      return () => {
        document.removeEventListener('mousedown', handleClickOutside)
        document.removeEventListener('touchstart', handleClickOutside)
      }
    }, [isOpen])

    // Esc 键关闭
    useEffect(() => {
      if (!isOpen) return

      const handleEscape = (event: KeyboardEvent) => {
        if (event.key === 'Escape') {
          setIsOpen(false)
        }
      }

      document.addEventListener('keydown', handleEscape)
      return () => document.removeEventListener('keydown', handleEscape)
    }, [isOpen])

    const selectedOption = options.find((opt) => opt.value === selectedValue)

    const handleSelect = (optionValue: string) => {
      if (disabled) return

      setSelectedValue(optionValue)
      setIsOpen(false)
      onChange?.(optionValue)
    }

    const handleToggle = () => {
      if (!disabled) {
        setIsOpen(!isOpen)
      }
    }

    const classes = clsx(
      'chaimir-select',
      `chaimir-select--${size}`,
      error && 'chaimir-select--error',
      disabled && 'chaimir-select--disabled',
      fullWidth && 'chaimir-select--full',
      isOpen && 'chaimir-select--open',
      className
    )

    return (
      <div ref={containerRef} className={classes}>
        <button
          type="button"
          className="chaimir-select__trigger"
          onClick={handleToggle}
          disabled={disabled}
          aria-haspopup="listbox"
          aria-expanded={isOpen}
        >
          <span className="chaimir-select__value">
            {selectedOption ? selectedOption.label : placeholder}
          </span>
          <ChevronDown
            className={clsx('chaimir-select__icon', isOpen && 'chaimir-select__icon--open')}
            size={16}
          />
        </button>

        {isOpen && (
          <div className="chaimir-select__dropdown" role="listbox">
            {options.map((option) => (
              <div
                key={option.value}
                className={clsx(
                  'chaimir-select__option',
                  option.value === selectedValue && 'chaimir-select__option--selected',
                  option.disabled && 'chaimir-select__option--disabled'
                )}
                role="option"
                aria-selected={option.value === selectedValue}
                onClick={() => !option.disabled && handleSelect(option.value)}
              >
                {option.label}
                {option.value === selectedValue && (
                  <Check size={16} className="chaimir-select__check" />
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    )
  }
)

Select.displayName = 'Select'
