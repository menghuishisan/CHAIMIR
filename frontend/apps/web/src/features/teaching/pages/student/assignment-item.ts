// assignment-item 解析 M5 题面视角(ContentItemSnapshot.body)中前端需要呈现的字段。
//
// 题面 body 按内容类型有不同结构(docs/05-题库与模板中心/02-架构设计.md §2.2):
//   实验模板 → description / summary / steps
//   竞赛题   → statement / scenario
//   理论题   → statement / question + options / choices
// 答案、判题配置与 flag 已由后端题面视角剥离(content 模块 defaultSensitivePaths),
// 前端不做二次过滤、也不请求 full 内容。
//
// 这里只做"读已知键、读不到就说读不到",不猜测未登记的键名:
// 猜错会把内部结构以错位文案的形式呈现给学生。

import type { AssignmentItem } from '@chaimir/api-client'

/** 题面正文的候选键,按内容类型的实际用法排列(先专有、后通用)。 */
const STATEMENT_KEYS = ['statement', 'question', 'scenario', 'description', 'summary'] as const

/** 选项列表的候选键。 */
const CHOICE_KEYS = ['options', 'choices'] as const

/**
 * itemStatement 取题面正文。
 * body 可能整体缺失(题目引用失效时后端返回空),此时返回 undefined 让页面给出说明。
 */
export function itemStatement(item: AssignmentItem): string | undefined {
  const body = item.body
  if (!body) return undefined
  for (const key of STATEMENT_KEYS) {
    const value = body[key]
    if (typeof value === 'string' && value.trim() !== '') return value
  }
  return undefined
}

/**
 * itemChoices 取选择型题目的选项。
 * 只接受字符串数组:选项要渲染成可点的单选项,非字符串元素无法作为选项文字,
 * 强行转换会把对象打印成 [object Object]。
 */
export function itemChoices(item: AssignmentItem): string[] {
  const body = item.body
  if (!body) return []
  for (const key of CHOICE_KEYS) {
    const value = body[key]
    if (Array.isArray(value)) {
      const choices = value.filter((choice): choice is string => typeof choice === 'string')
      if (choices.length > 0) return choices
    }
  }
  return []
}
