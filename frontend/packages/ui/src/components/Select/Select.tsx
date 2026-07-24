// Select 组件：下拉选择
// 符合 FE-2（无障碍）、FE-4（文案面向用户）

import React, { useState, useEffect, useId } from 'react'
import { createPortal } from 'react-dom'
import { useFloating, offset, flip, shift, autoUpdate, size } from '@floating-ui/react'
import { ChevronDown, Check } from 'lucide-react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
import { useClickOutside, useEscapeKey, useTransformOrigin } from '../../hooks'
import './Select.css'

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

export interface SelectProps {
  id?: string
  options: SelectOption[]
  value?: string
  defaultValue?: string
  placeholder?: string
  size?: 'sm' | 'md' | 'lg'
  disabled?: boolean
  error?: boolean
  fullWidth?: boolean
  onChange?: (value: string) => void
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
      size: selectSize = 'md',
      disabled = false,
      error = false,
      fullWidth = false,
      onChange,
      className,
    },
    ref
  ) => {
    const [isOpen, setIsOpen] = useState(false)
    const [selectedValue, setSelectedValue] = useState(value || defaultValue || '')
    const [activeIndex, setActiveIndex] = useState(0)
    const [animateDropdown, setAnimateDropdown] = useState(false)
    const generatedId = useId()
    const triggerId = id ?? `chaimir-select-${generatedId}`
    const listboxId = `${triggerId}-listbox`

    const { refs, floatingStyles, placement, isPositioned } = useFloating<HTMLDivElement>({
      open: isOpen,
      onOpenChange: setIsOpen,
      placement: 'bottom-start',
      whileElementsMounted: autoUpdate,
      middleware: [
        offset(4),
        flip({ padding: 8 }),
        shift({ padding: 8 }),
        size({
          apply({ rects, elements }) {
            Object.assign(elements.floating.style, {
              width: `${rects.reference.width}px`,
            })
          },
        }),
      ],
    })
    const transformOrigin = useTransformOrigin(placement)

    // 监听 value 变化
    useEffect(() => {
      if (value !== undefined) {
        setSelectedValue(value)
      }
    }, [value])

    useClickOutside(
      refs.floating as React.RefObject<HTMLDivElement>,
      (e) => {
        if (refs.domReference.current && refs.domReference.current.contains(e.target as Node)) {
          return
        }
        setIsOpen(false)
      },
      isOpen
    )

    useEscapeKey(() => setIsOpen(false), isOpen)

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

    const handleKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
      if (disabled) return
      setAnimateDropdown(false)
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
      `chaimir-select--${selectSize}`,
      error && 'chaimir-select--error',
      disabled && 'chaimir-select--disabled',
      fullWidth && 'chaimir-select--full',
      isOpen && 'chaimir-select--open',
      className
    )

    const handleRef = (node: HTMLDivElement | null) => {
      refs.setReference(node)
      if (typeof ref === 'function') {
        ref(node)
      } else if (ref) {
        ref.current = node
      }
    }

    return (
      <div ref={handleRef} className={classes}>
        <button
          id={triggerId}
          type="button"
          className="chaimir-select__trigger"
          onClick={() => {
            triggerHaptic(10)
            handleToggle()
          }}
          onPointerDown={() => setAnimateDropdown(true)}
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

        {isOpen &&
          typeof document !== 'undefined' &&
          createPortal(
            <div
              ref={refs.setFloating}
              style={{
                ...floatingStyles,
                '--transform-origin': transformOrigin,
                visibility: isPositioned ? 'visible' : 'hidden',
                pointerEvents: isPositioned ? 'auto' : 'none',
              } as React.CSSProperties}
              className="chaimir-select__positioner"
              data-placement={placement}
            >
              <div
                id={listboxId}
                className={clsx('chaimir-select__dropdown', animateDropdown && 'chaimir-select__dropdown--animated')}
                role="listbox"
              >
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
                    onClick={() => {
                      if (!option.disabled) {
                        triggerHaptic(10)
                        handleSelect(option.value)
                      }
                    }}
                  >
                    {option.label}
                    {option.value === selectedValue && (
                      <Check size={16} className="chaimir-select__check" />
                    )}
                  </div>
                ))}
              </div>
            </div>,
            document.body
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
 * nextEnabledIndex 按方向寻找下一个可选项，支持首尾循环和禁用项跳过。
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
