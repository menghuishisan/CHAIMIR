import { useId } from 'react'
import { Button, Input } from '@chaimir/ui'

/** 保留每个参数的完整输入，避免重新分词改变命令语义。 */
export function CommandArgvFields({
  label, value, onChange, onBlur, helper, error, required = false, onDark = false, disabled = false,
}: {
  label: string
  value: string[]
  onChange: (value: string[]) => void
  onBlur?: () => void
  helper?: string
  error?: string | null
  required?: boolean
  onDark?: boolean
  disabled?: boolean
}) {
  const id = useId()
  const message = error || helper
  const messageId = `${id}-message`

  return (
    <fieldset disabled={disabled} onBlur={onBlur} className="flex min-w-0 flex-col gap-2"
      aria-describedby={message ? messageId : undefined} aria-invalid={error ? true : undefined}>
      <legend className={`text-sm font-medium ${onDark ? 'text-on-dark' : 'text-ink'}`}>
        {label}
        {required ? <>
          <span aria-hidden="true" className={onDark ? 'text-on-dark-danger' : 'text-danger'}> *</span>
          <span className="sr-only">必填</span>
        </> : null}
      </legend>
      {value.map((item, index) => (
        <div key={index} className="flex items-center gap-2">
          <Input variant={onDark ? 'dark' : 'boxed'} aria-label={`${label}第 ${index + 1} 个参数`} className="min-w-0 flex-1 font-mono text-sm"
            value={item} placeholder={index === 0 ? '命令' : '参数'}
            onChange={(event) => onChange(value.map((part, position) =>
              position === index ? event.target.value : part))} />
          <Button type="button" variant={onDark ? 'on-dark' : 'ghost'} size="sm" disabled={value.length <= 1}
            onClick={() => onChange(value.filter((_, position) => position !== index))}>移除</Button>
        </div>
      ))}
      <Button type="button" variant={onDark ? 'on-dark' : 'outline'} size="sm" onClick={() => onChange([...value, ''])}>
        添加参数
      </Button>
      {message ? <p id={messageId} role={error ? 'alert' : undefined}
        className={`text-xs ${error ? (onDark ? 'text-on-dark-danger' : 'text-danger') : (onDark ? 'text-on-dark-sub' : 'text-ink-sub')}`}>{message}</p> : null}
    </fieldset>
  )
}
