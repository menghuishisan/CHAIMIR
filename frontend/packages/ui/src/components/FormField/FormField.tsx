// FormField 组件：表单字段容器（带标签和错误提示）
// 符合 FE-2（label[for] 显式关联）、FE-4（错误文案面向用户）

import React from 'react'
import { clsx } from 'clsx'
import './FormField.css'

export interface FormFieldProps extends React.HTMLAttributes<HTMLDivElement> {
  /** 字段标签 */
  label?: React.ReactNode
  /** 是否必填 */
  required?: boolean
  /** 错误信息 */
  error?: string
  /** 帮助文本 */
  helperText?: string
  /** 输入框 ID（用于 label[for]） */
  htmlFor?: string
  /** 子元素 */
  children?: React.ReactNode
}

export const FormField = React.forwardRef<HTMLDivElement, FormFieldProps>(
  (
    {
      label,
      required = false,
      error,
      helperText,
      htmlFor,
      children,
      className,
      ...props
    },
    ref
  ) => {
    const classes = clsx('chaimir-form-field', className)

    return (
      <div ref={ref} className={classes} {...props}>
        {label && (
          <label htmlFor={htmlFor} className="chaimir-form-field__label">
            {label}
            {required && <span className="chaimir-form-field__required" aria-label="必填">*</span>}
          </label>
        )}
        <div className="chaimir-form-field__control">
          {children}
        </div>
        {error && (
          <div className="chaimir-form-field__error" role="alert">
            {error}
          </div>
        )}
        {!error && helperText && (
          <div className="chaimir-form-field__helper">
            {helperText}
          </div>
        )}
      </div>
    )
  }
)

FormField.displayName = 'FormField'
