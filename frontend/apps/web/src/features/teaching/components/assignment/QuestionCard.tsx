// QuestionCard 作业里的一道题(设计 §7.2 作业作答):题号与题名、题型 · 分值 · 难度一行、题干、按题型渲染的作答控件。
// 题型按题面里的 q_type 渲染(单选、多选、判断、填空、简答);题面已由后端剥离答案,前端不做二次过滤。
// 选项是整行可点的描边行,选中时玉色描边与浅底;填空逐格输入,选择答案不通过换行字符串中转。

import type { AssignmentItem, TheoryAnswer } from '@chaimir/api-client'
import { Callout } from '@chaimir/ui'
import { TheoryAnswerControl } from '../../../../components/TheoryAnswerControl'
import { contentDifficultyLabel, theoryQuestionTypeLabel } from '../../../../utils/labels/content'
import { itemStatement } from '../../assignment-answers'

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
        <TheoryAnswerControl
          id={item.id}
          body={item.body}
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
