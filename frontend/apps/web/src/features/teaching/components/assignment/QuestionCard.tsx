// QuestionCard 作业里的一道题(设计 §7.2 作业作答):题号与题名、题型 · 分值 · 难度一行、题干、按题型渲染的作答控件。
// 题型按题面里的 q_type 渲染(单选、多选、判断、填空、简答);题面已由后端剥离答案,前端不做二次过滤。
// 选项是整行可点的描边行,选中时玉色描边与浅底;填空逐格输入,选择答案不通过换行字符串中转。

import type { AssignmentItem, TheoryAnswer } from '@chaimir/api-client'
import { Callout, Checkbox, FormField, Input, RadioGroup, RadioItem, Textarea } from '@chaimir/ui'
import { contentDifficultyLabel, theoryQuestionTypeLabel } from '../../../../utils/labels/content'
import { itemChoices, itemStatement } from '../../assignment-answers'

/** 选项行:卡内强调块,整行可点;选中由控件本身的勾选表达,玉色浅底只是加强(纸面上不用边框圈盒) */
const OPTION_ROW_CLASS =
  'w-full rounded-md bg-surface-hover px-3.5 py-3 transition-colors duration-fast hover:bg-primary-soft has-aria-checked:bg-primary-soft'

/** questionAnchorId 是题目卡片的锚点,答题卡与确认弹窗据此跳转 */
export function questionAnchorId(itemId: string): string {
  return `assignment-item-${itemId}`
}

/** questionHeadingId 是题目标题:跳转后焦点落在这里,作答控件也用它做无障碍名称 */
export function questionHeadingId(itemId: string): string {
  return `assignment-item-${itemId}-title`
}

export interface QuestionCardProps {
  item: AssignmentItem
  seq: number
  value: TheoryAnswer | undefined
  onChange: (value: TheoryAnswer) => void
  onBlur: () => void
  disabled: boolean
}

/** QuestionCard 渲染单题题面与作答控件 */
export function QuestionCard({ item, seq, value, onChange, onBlur, disabled }: QuestionCardProps) {
  const statement = itemStatement(item)
  const questionType = String(item.body?.q_type ?? '')
  const headingId = questionHeadingId(item.id)
  const typeLabel = theoryQuestionTypeLabel(questionType)
  const facts: string[] = []
  if (typeLabel) facts.push(typeLabel)
  facts.push(`${item.score} 分`)
  if (item.difficulty !== undefined) facts.push(`难度${contentDifficultyLabel(item.difficulty)}`)

  return (
    // scroll-mt 让跳转后卡片顶部不贴着滚动区边缘
    <section
      id={questionAnchorId(item.id)}
      aria-labelledby={headingId}
      className="scroll-mt-4 rounded-lg bg-surface p-5 shadow-xs md:p-6"
    >
      <h2
        id={headingId}
        tabIndex={-1}
        className="flex flex-wrap items-baseline gap-x-2 text-md font-semibold text-ink outline-hidden"
      >
        <span className="tabular-nums">第 {seq} 题</span>
        {item.title ? <span className="min-w-0 break-words">{item.title}</span> : null}
      </h2>
      <p className="mt-1 text-sm text-ink-sub">{facts.join(' · ')}</p>

      {statement ? (
        <div className="mt-4 flex flex-col gap-2 text-base leading-relaxed text-ink">
          {statement
            .split('\n')
            .map((paragraph, index) =>
              paragraph.trim() === '' ? null : <p key={index}>{paragraph}</p>
            )}
        </div>
      ) : (
        <Callout tone="info" className="mt-4">
          这道题的题面暂未提供,请联系老师确认。
        </Callout>
      )}

      <div className="mt-5">
        <AnswerControl
          item={item}
          questionType={questionType}
          headingId={headingId}
          value={value}
          onChange={onChange}
          onBlur={onBlur}
          disabled={disabled}
        />
      </div>
    </section>
  )
}

interface AnswerControlProps extends Omit<QuestionCardProps, 'seq'> {
  questionType: string
  headingId: string
}

/** AnswerControl 按题型给出作答控件;无障碍名称取题目标题 */
function AnswerControl({
  item,
  questionType,
  headingId,
  value,
  onChange,
  onBlur,
  disabled,
}: AnswerControlProps) {
  const choices = itemChoices(item)
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
            key={index}
            value={choice}
            label={choice}
            disabled={disabled}
            onBlur={onBlur}
            className={OPTION_ROW_CLASS}
          />
        ))}
      </RadioGroup>
    )
  }

  if (questionType === 'multiple_choice') {
    const selected = Array.isArray(value) ? value : []
    return (
      <div role="group" aria-labelledby={headingId} className="flex flex-col gap-2">
        <p className="text-sm text-ink-sub">可以选多个</p>
        {choices.map((choice, index) => (
          <Checkbox
            key={index}
            checked={selected.includes(choice)}
            label={choice}
            disabled={disabled}
            onBlur={onBlur}
            className={OPTION_ROW_CLASS}
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
          value="true"
          label="正确"
          disabled={disabled}
          onBlur={onBlur}
          className={OPTION_ROW_CLASS}
        />
        <RadioItem
          value="false"
          label="错误"
          disabled={disabled}
          onBlur={onBlur}
          className={OPTION_ROW_CLASS}
        />
      </RadioGroup>
    )
  }

  if (questionType === 'fill_blank') {
    const blanks = Array.isArray(value)
      ? value
      : Array.from({ length: Number(item.body?.blank_count) }, () => '')
    if (blanks.length === 0) {
      return <Callout tone="danger">这道题的填空位置暂未提供,请联系老师确认。</Callout>
    }
    return (
      <div className="grid gap-3 sm:grid-cols-2">
        {blanks.map((blank, index) => (
          <FormField
            key={index}
            label={`第 ${index + 1} 空`}
            htmlFor={`answer-${item.id}-${index}`}
          >
            <Input
              id={`answer-${item.id}-${index}`}
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
