// SelectField：公共认证表单的受控选择控件。

import React from 'react'
import { FormField, Select } from '@chaimir/ui'
import type { SelectOption } from '@chaimir/ui'
import type { FormState } from '../types'
import { updateField } from '../form-state'

/**
 * SelectField 渲染公共认证页的语义选择项，避免把后端编号直接暴露给用户。
 */
export function SelectField({
  name,
  label,
  value,
  options,
  onChange,
  placeholder,
  helperText,
  required,
}: {
  name: string
  label: string
  value?: string
  options: SelectOption[]
  onChange: React.Dispatch<React.SetStateAction<FormState>>
  placeholder?: string
  helperText?: string
  required?: boolean
}): React.ReactElement {
  const fieldId = `auth-${name}`

  return (
    <FormField className="public-field" label={label} htmlFor={fieldId} helperText={helperText} required={required}>
      <Select
        id={fieldId}
        className="public-select"
        value={value}
        options={options}
        placeholder={placeholder}
        fullWidth
        onChange={(nextValue) => updateField(onChange, name, nextValue)}
      />
    </FormField>
  )
}
