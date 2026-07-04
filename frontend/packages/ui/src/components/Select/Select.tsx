// Select 组件：下拉选择
// 符合 FE-2（无障碍）、FE-4（文案面向用户）

import React, { useState, useRef, useEffect, useId } from 'react'
import { ChevronDown, Check } from 'lucide-react'
import { clsx } from 'clsx'
import './Select.css'

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

export interface SelectProps {
  /** 触发按钮 ID，用于 label 关联 */
  id?: string
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
      id,
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
    const [activeIndex, setActiveIndex] = useState(0)
    const containerRef = useRef<HTMLDivElement>(null)
    const generatedId = useId()
    const triggerId = id ?? `chaimir-select-${generatedId}`
    const listboxId = `${triggerId}-listbox`

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
    const selectedIndex = options.findIndex((opt) => opt.value === selectedValue && !opt.disabled)
    const activeOption = options[activeIndex]

    const handleSelect = (optionValue: string) => {
      if (disabled) return

      setSelectedValue(optionValue)
      setIsOpen(false)
      onChange?.(optionValue)
    }

    const handleToggle = () => {
      if (!disabled) {
        setActiveIndex(selectedIndex >= 0 ? selectedIndex : firstEnabledIndex(options))
        setIsOpen(!isOpen)
      }
    }

    /**
     * handleKeyDown 提供选择器键盘操作，避免下拉项只能通过鼠标点击。
     */
    const handleKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
      if (disabled) return
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        setIsOpen(true)
        setActiveIndex((current) => nextEnabledIndex(options, current, 1))
        return
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        setIsOpen(true)
        setActiveIndex((current) => nextEnabledIndex(options, current, -1))
        return
      }
      if ((event.key === 'Enter' || event.key === ' ') && isOpen) {
        event.preventDefault()
        if (activeOption && !activeOption.disabled) {
          handleSelect(activeOption.value)
        }
        return
      }
      if (event.key === 'Home') {
        event.preventDefault()
        setIsOpen(true)
        setActiveIndex(firstEnabledIndex(options))
        return
      }
      if (event.key === 'End') {
        event.preventDefault()
        setIsOpen(true)
        setActiveIndex(lastEnabledIndex(options))
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
          id={triggerId}
          type="button"
          className="chaimir-select__trigger"
          onClick={handleToggle}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          aria-haspopup="listbox"
          aria-expanded={isOpen}
          aria-controls={listboxId}
          aria-invalid={error || undefined}
          aria-activedescendant={isOpen && activeOption ? `${listboxId}-${activeOption.value}` : undefined}
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
          <div id={listboxId} className="chaimir-select__dropdown" role="listbox">
            {options.map((option, index) => (
              <div
                key={option.value}
                id={`${listboxId}-${option.value}`}
                className={clsx(
                  'chaimir-select__option',
                  option.value === selectedValue && 'chaimir-select__option--selected',
                  index === activeIndex && 'chaimir-select__option--active',
                  option.disabled && 'chaimir-select__option--disabled'
                )}
                role="option"
                aria-selected={option.value === selectedValue}
                aria-disabled={option.disabled || undefined}
                onMouseEnter={() => !option.disabled && setActiveIndex(index)}
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

/**
 * firstEnabledIndex 返回第一个可选项位置，空列表时回到 0。
 */
function firstEnabledIndex(options: SelectOption[]): number {
  const index = options.findIndex((option) => !option.disabled)
  return index >= 0 ? index : 0
}

/**
 * lastEnabledIndex 返回最后一个可选项位置，空列表时回到 0。
 */
function lastEnabledIndex(options: SelectOption[]): number {
  for (let index = options.length - 1; index >= 0; index -= 1) {
    if (!options[index]?.disabled) {
      return index
    }
  }
  return 0
}

/**
 * nextEnabledIndex 按方向寻找下一个可选项，支持首尾循环。
 */
function nextEnabledIndex(options: SelectOption[], current: number, direction: 1 | -1): number {
  if (options.length === 0) {
    return 0
  }
  let next = current
  for (let count = 0; count < options.length; count += 1) {
    next = (next + direction + options.length) % options.length
    if (!options[next]?.disabled) {
      return next
    }
  }
  return current
}
