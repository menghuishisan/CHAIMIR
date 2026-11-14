// 作业题面与结构化答案的统一读写规则,供学生作答和教师批改复用。
import type { AssignmentItem, TheoryAnswer } from '@chaimir/api-client'

export type AssignmentAnswers = Record<string, TheoryAnswer>

/** isAssignmentAnswer 校验恢复记录,保留布尔值与按位置组织的答案数组。 */
export function isAssignmentAnswer(value: unknown): value is TheoryAnswer {
  return (
    typeof value === 'string' ||
    typeof value === 'boolean' ||
    (Array.isArray(value) && value.every((entry) => typeof entry === 'string'))
  )
}

/** isAnswerEmpty 判断是否实际作答,判断题的 false 是有效答案。 */
export function isAnswerEmpty(value: TheoryAnswer | undefined): boolean {
  if (value === undefined) return true
  if (typeof value === 'boolean') return false
  if (Array.isArray(value)) return value.every((entry) => entry.trim() === '')
  return value.trim() === ''
}

/** itemStatement 只读取理论题约定的题干字段。 */
export function itemStatement(item: AssignmentItem): string | undefined {
  const statement = item.body?.statement
  return typeof statement === 'string' && statement.trim() !== '' ? statement : undefined
}

/** itemChoices 只读取理论题约定的选项,不推测旧字段或丢弃非法成员。 */
export function itemChoices(item: AssignmentItem): string[] {
  const options = item.body?.options
  return Array.isArray(options) && options.every((entry) => typeof entry === 'string')
    ? options
    : []
}

/** draftAnswers 恢复当前作业的答案,不让旧题号进入后续保存请求。 */
export function draftAnswers(
  content: Record<string, unknown>,
  items: AssignmentItem[],
  localAnswers?: AssignmentAnswers
): AssignmentAnswers {
  const answers: AssignmentAnswers = {}
  for (const item of items) {
    const value =
      localAnswers && Object.prototype.hasOwnProperty.call(localAnswers, item.id)
        ? localAnswers[item.id]
        : content[item.id]
    if (isAssignmentAnswer(value)) answers[item.id] = Array.isArray(value) ? [...value] : value
  }
  return answers
}

/** serializeAnswers 保留题型结构和填空位置,只省略整题未作答的键。 */
export function serializeAnswers(
  items: AssignmentItem[],
  answers: AssignmentAnswers
): AssignmentAnswers {
  const output: AssignmentAnswers = {}
  for (const item of items) {
    const value = answers[item.id]
    if (value !== undefined && !isAnswerEmpty(value))
      output[item.id] = Array.isArray(value) ? [...value] : value
  }
  return output
}

/** formatAssignmentAnswer 为批改呈现学生原始作答,空格保留序号。 */
export function formatAssignmentAnswer(item: AssignmentItem, value: unknown): string {
  if (!isAssignmentAnswer(value) || isAnswerEmpty(value)) return '未作答'
  if (typeof value === 'boolean') return value ? '正确' : '错误'
  if (Array.isArray(value)) {
    return value
      .map((entry, index) =>
        item.body?.q_type === 'fill_blank' ? `第 ${index + 1} 空：${entry || '未填写'}` : entry
      )
      .join('\n')
  }
  return value
}
