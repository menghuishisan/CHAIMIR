// TextField：公共认证表单的带图标受控输入框。

import React from 'react'
import { FormField, Input } from '@chaimir/ui'
import type { FormState } from '../types'
import { updateField } from '../form-state'

/**
 * TextField 渲染带 Lucide 图标的受控输入框。
 */
export function TextField({
  icon,
  name,
  label,
  value,
  onChange,
  type = 'text',
  autoComplete,
  required,
}: {
  icon: React.ReactNode
  name: string
  label: string
  value?: string
  onChange: React.Dispatch<React.SetStateAction<FormState>>
  type?: string
  autoComplete?: string
  required?: boolean
}): React.ReactElement {
  const fieldId = `auth-${name}`

  return (
    <FormField className="public-field" label={label} htmlFor={fieldId} required={required}>
      <Input
        id={fieldId}
        name={name}
        type={type}
        value={value ?? ''}
        required={required}
        autoComplete={autoComplete}
        leftIcon={icon}
        fullWidth
        onChange={(event) => updateField(onChange, name, event.target.value)}
      />
    </FormField>
  )
}
