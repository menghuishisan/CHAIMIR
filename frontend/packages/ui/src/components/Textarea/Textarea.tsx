// Textarea 组件：多行文本输入框

import React from 'react'
import { clsx } from 'clsx'
import './Textarea.css'

export interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  /** 错误状态 */
  error?: boolean
  /** 完整宽度 */
  fullWidth?: boolean
  /** 是否可调整大小 */
  resize?: 'none' | 'vertical' | 'horizontal' | 'both'
}

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  (
    {
      error = false,
      fullWidth = true,
      resize = 'vertical',
      className,
      disabled,
      ...props
    },
    ref
  ) => {
    const classes = clsx(
      'chaimir-textarea',
      error && 'chaimir-textarea--error',
      fullWidth && 'chaimir-textarea--full',
      `chaimir-textarea--resize-${resize}`,
      disabled && 'chaimir-textarea--disabled',
      className
    )

    return (
      <textarea
        ref={ref}
        className={classes}
        disabled={disabled}
        aria-invalid={error}
        {...props}
      />
    )
  }
)

Textarea.displayName = 'Textarea'
