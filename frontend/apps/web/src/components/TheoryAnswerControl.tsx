import type { TheoryAnswer } from '@chaimir/api-client'
import { Callout, Checkbox, FormField, Input, RadioGroup, RadioItem, Textarea } from '@chaimir/ui'

interface TheoryAnswerControlProps {
  id: string
  body: Record<string, unknown> | undefined
  headingId: string
  value: TheoryAnswer | undefined
  onChange: (value: TheoryAnswer) => void
  onBlur?: () => void
  disabled: boolean
  onDark?: boolean
}

export function TheoryAnswerControl({
  id,
  body,
  headingId,
  value,
  onChange,
  onBlur,
  disabled,
  onDark = false,
}: TheoryAnswerControlProps) {
  const options = body?.options
  const choices =
    Array.isArray(options) && options.every((entry) => typeof entry === 'string') ? options : []
  const questionType = String(body?.q_type ?? '')
  const rowClass = onDark
    ? 'w-full rounded-md bg-dark-surface px-3.5 py-3 hover:bg-dark-elevated has-aria-checked:bg-dark-elevated'
    : 'w-full rounded-md bg-surface-hover px-3.5 py-3 transition-colors duration-fast hover:bg-primary-soft has-aria-checked:bg-primary-soft'
  const textValue = typeof value === 'string' ? value : ''

  if (
    (questionType === 'single_choice' || questionType === 'multiple_choice') &&
    choices.length === 0
  ) {
    return <Callout tone="danger">这道题的选项暂未提供,请联系老师确认。</Callout>
  }

  if (questionType === 'single_choice') {
    return (
      <RadioGroup
        aria-labelledby={headingId}
        value={textValue}
        onValueChange={onChange}
        disabled={disabled}
      >
        {choices.map((choice, index) => (
          <RadioItem
            onDark={onDark}
            key={index}
            value={choice}
            label={choice}
            disabled={disabled}
            onBlur={onBlur}
            className={rowClass}
          />
        ))}
      </RadioGroup>
    )
  }

  if (questionType === 'multiple_choice') {
    const selected = Array.isArray(value) ? value : []
    return (
      <div role="group" aria-labelledby={headingId} className="flex flex-col gap-2">
        <p className={onDark ? 'text-sm text-on-dark-sub' : 'text-sm text-ink-sub'}>可以选多个</p>
        {choices.map((choice, index) => (
          <Checkbox
            onDark={onDark}
            key={index}
            checked={selected.includes(choice)}
            label={choice}
            disabled={disabled}
            onBlur={onBlur}
            className={rowClass}
            onCheckedChange={(checked) => {
              const next = new Set(selected)
              if (checked) next.add(choice)
              else next.delete(choice)
              // 按选项顺序保存,不随点击顺序变化
              onChange(choices.filter((entry) => next.has(entry)))
            }}
          />
        ))}
      </div>
    )
  }

  if (questionType === 'true_false') {
    return (
      <RadioGroup
        aria-labelledby={headingId}
        value={typeof value === 'boolean' ? String(value) : ''}
        onValueChange={(choice) => onChange(choice === 'true')}
        disabled={disabled}
        className="grid grid-cols-2 gap-3"
      >
        <RadioItem
          onDark={onDark}
          value="true"
          label="正确"
          disabled={disabled}
          onBlur={onBlur}
          className={rowClass}
        />
        <RadioItem
          onDark={onDark}
          value="false"
          label="错误"
          disabled={disabled}
          onBlur={onBlur}
          className={rowClass}
        />
      </RadioGroup>
    )
  }

  if (questionType === 'fill_blank') {
    const blanks = Array.isArray(value)
      ? value
      : Array.from({ length: Number(body?.blank_count) }, () => '')
    if (blanks.length === 0) {
      return <Callout tone="danger">这道题的填空位置暂未提供,请联系老师确认。</Callout>
    }
    return (
      <div className="grid gap-3 sm:grid-cols-2">
        {blanks.map((blank, index) => (
          <FormField
            onDark={onDark}
            key={index}
            label={`第 ${index + 1} 空`}
            htmlFor={`answer-${id}-${index}`}
          >
            <Input
              variant={onDark ? 'dark' : 'boxed'}
              id={`answer-${id}-${index}`}
              value={blank}
              disabled={disabled}
              onChange={(event) =>
                onChange(
                  blanks.map((entry, position) => (position === index ? event.target.value : entry))
                )
              }
              onBlur={onBlur}
            />
          </FormField>
        ))}
      </div>
    )
  }

  if (questionType === 'short_answer') {
    return (
      <Textarea
        onDark={onDark}
        aria-labelledby={headingId}
        value={textValue}
        disabled={disabled}
        rows={6}
        placeholder="写下你的解答"
        onChange={(event) => onChange(event.target.value)}
        onBlur={onBlur}
      />
    )
  }

  return <Callout tone="danger">题目格式暂不可用,请联系老师确认。</Callout>
}
