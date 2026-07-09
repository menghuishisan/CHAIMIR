// IconButton 组件：用于工具栏、表格行操作和工作台中的纯图标按钮。

import React from 'react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
import './IconButton.css'

export interface IconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  /** 无可见文字时必须提供可访问名称。 */
  'aria-label': string
  /** 图标内容，统一由调用方传入 Lucide 图标。 */
  icon: React.ReactNode
  /** 按钮视觉语义。 */
  variant?: 'ghost' | 'outline' | 'primary' | 'danger' | 'on-dark'
  /** 按钮尺寸。 */
  size?: 'sm' | 'md' | 'lg'
  /** 选中态，用于工具模式、视图切换等场景。 */
  selected?: boolean
}

export interface IconLinkProps extends React.AnchorHTMLAttributes<HTMLAnchorElement> {
  /** 无可见文字时必须提供可访问名称。 */
  'aria-label': string
  /** 图标内容，统一由调用方传入 Lucide 图标。 */
  icon: React.ReactNode
  /** 链接视觉语义。 */
  variant?: 'ghost' | 'outline' | 'primary' | 'danger' | 'on-dark'
  /** 链接尺寸。 */
  size?: 'sm' | 'md' | 'lg'
  /** 选中态，用于当前页入口等场景。 */
  selected?: boolean
  /** 附加内容，例如角标。 */
  children?: React.ReactNode
}

export const IconButton = React.forwardRef<HTMLButtonElement, IconButtonProps>(
  (
    {
      icon,
      variant = 'ghost',
      size = 'md',
      selected = false,
      className,
      type = 'button',
      ...props
    },
    ref
  ) => {
    const handleClick = (e: React.MouseEvent<HTMLButtonElement>) => {
      triggerHaptic(10)
      if (props.onClick) {
        props.onClick(e)
      }
    }

    return (
      <button
        ref={ref}
        type={type}
        className={clsx(
          'chaimir-icon-button',
          `chaimir-icon-button--${variant}`,
          `chaimir-icon-button--${size}`,
          selected && 'is-selected',
          className
        )}
        aria-pressed={selected || undefined}
        onClick={handleClick}
        {...props}
      >
        <span className="chaimir-icon-button__icon" aria-hidden="true">
          {icon}
        </span>
      </button>
    )
  }
)

IconButton.displayName = 'IconButton'

export const IconLink = React.forwardRef<HTMLAnchorElement, IconLinkProps>(
  (
    {
      icon,
      variant = 'ghost',
      size = 'md',
      selected = false,
      className,
      children,
      ...props
    },
    ref
  ) => {
    const handleClick = (e: React.MouseEvent<HTMLAnchorElement>) => {
      triggerHaptic(10)
      if (props.onClick) {
        props.onClick(e)
      }
    }

    return (
      <a
        ref={ref}
        className={clsx(
          'chaimir-icon-button',
          `chaimir-icon-button--${variant}`,
          `chaimir-icon-button--${size}`,
          selected && 'is-selected',
          className
        )}
        aria-current={selected ? 'page' : props['aria-current']}
        onClick={handleClick}
        {...props}
      >
        <span className="chaimir-icon-button__icon" aria-hidden="true">
          {icon}
        </span>
        {children}
      </a>
    )
  }
)

IconLink.displayName = 'IconLink'
