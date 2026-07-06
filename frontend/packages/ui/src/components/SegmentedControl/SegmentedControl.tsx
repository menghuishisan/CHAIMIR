// SegmentedControl 组件：用于视图模式、工具模式和密度切换的分段选择。

import React from 'react'
import { clsx } from 'clsx'
import './SegmentedControl.css'

export interface SegmentedControlOption {
  value: string
  label: React.ReactNode
  icon?: React.ReactNode
  disabled?: boolean
}

export interface SegmentedControlProps extends Omit<React.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  options: SegmentedControlOption[]
  value: string
  label: string
  size?: 'sm' | 'md'
  onChange: (value: string) => void
}

export function SegmentedControl({
  options,
  value,
  label,
  size = 'md',
  onChange,
  className,
  ...props
}: SegmentedControlProps): React.ReactElement {
  const name = React.useId()
  const [sliderStyle, setSliderStyle] = React.useState<React.CSSProperties>({})
  const containerRef = React.useRef<HTMLDivElement>(null)
  const itemRefs = React.useRef<Array<HTMLLabelElement | null>>([])

  React.useEffect(() => {
    const activeIndex = options.findIndex((opt) => opt.value === value)
    const activeItem = itemRefs.current[activeIndex]
    const container = containerRef.current

    if (!activeItem || !container) return

    const updateSlider = () => {
      // 容器是相对定位，offsetLeft 可直接用于滑块位移。
      setSliderStyle({
        transform: `translateX(${activeItem.offsetLeft}px)`,
        width: `${activeItem.offsetWidth}px`,
        opacity: 1,
      })
    }

    updateSlider()

    const observer = new ResizeObserver(updateSlider)
    observer.observe(container)
    observer.observe(activeItem)

    return () => observer.disconnect()
  }, [value, options])

  return (
    <div
      ref={containerRef}
      className={clsx('chaimir-segmented-control', `chaimir-segmented-control--${size}`, className)}
      role="radiogroup"
      aria-label={label}
      {...props}
    >
      {options.map((option, index) => {
        const selected = option.value === value
        return (
          <label
            className={clsx('chaimir-segmented-control__item', selected && 'is-selected', option.disabled && 'is-disabled')}
            key={option.value}
            ref={(el) => {
              itemRefs.current[index] = el
            }}
          >
            <input
              type="radio"
              name={name}
              value={option.value}
              checked={selected}
              disabled={option.disabled}
              onChange={() => onChange(option.value)}
            />
            {option.icon && <span className="chaimir-segmented-control__icon" aria-hidden="true">{option.icon}</span>}
            <span className="chaimir-segmented-control__label">{option.label}</span>
          </label>
        )
      })}
      <div className="chaimir-segmented-control__slider" style={sliderStyle} aria-hidden="true" />
    </div>
  )
}
