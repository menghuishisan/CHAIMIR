// content labels 文件维护 M5 题库与模板中心枚举的用户向文案与语义色。

import type { StatusTone } from '@chaimir/ui'
import {
  ContentAuthorType,
  ContentDifficulty,
  ContentStatus,
  ContentType,
  ContentVisibility,
  PaperMode,
} from '@chaimir/api-client'

const CONTENT_TYPE_LABELS: Record<ContentType, string> = {
  [ContentType.EXPERIMENT_TEMPLATE]: '实验模板',
  [ContentType.CONTEST_PROBLEM]: '竞赛题',
  [ContentType.THEORY_QUESTION]: '理论题',
}

/** contentTypeLabel 返回内容类型文案。 */
export function contentTypeLabel(type: ContentType): string {
  return CONTENT_TYPE_LABELS[type]
}

/** CONTENT_TYPES 供表单按登记顺序渲染选项,避免页面再抄一份取值清单。 */
export const CONTENT_TYPES = [
  ContentType.EXPERIMENT_TEMPLATE,
  ContentType.CONTEST_PROBLEM,
  ContentType.THEORY_QUESTION,
] as const

const DIFFICULTY_LABELS: Record<ContentDifficulty, string> = {
  [ContentDifficulty.INTRO]: '入门',
  [ContentDifficulty.BASIC]: '基础',
  [ContentDifficulty.ADVANCED]: '进阶',
  [ContentDifficulty.CHALLENGE]: '挑战',
}

/** contentDifficultyLabel 返回题目难度文案。 */
export function contentDifficultyLabel(difficulty: ContentDifficulty): string {
  return DIFFICULTY_LABELS[difficulty]
}

/** CONTENT_DIFFICULTIES 供表单渲染难度选项。 */
export const CONTENT_DIFFICULTIES = [
  ContentDifficulty.INTRO,
  ContentDifficulty.BASIC,
  ContentDifficulty.ADVANCED,
  ContentDifficulty.CHALLENGE,
] as const

const STATUS_LABELS: Record<ContentStatus, string> = {
  [ContentStatus.DRAFT]: '草稿',
  [ContentStatus.PUBLISHED]: '已发布',
  [ContentStatus.DEPRECATED]: '已弃用',
}

const STATUS_TONES: Record<ContentStatus, StatusTone> = {
  [ContentStatus.DRAFT]: 'neutral',
  [ContentStatus.PUBLISHED]: 'success',
  [ContentStatus.DEPRECATED]: 'warning',
}

/** contentStatusLabel 返回内容状态文案。 */
export function contentStatusLabel(status: ContentStatus): string {
  return STATUS_LABELS[status]
}

/** contentStatusTone 返回内容状态语义色。 */
export function contentStatusTone(status: ContentStatus): StatusTone {
  return STATUS_TONES[status]
}

const VISIBILITY_LABELS: Record<ContentVisibility, string> = {
  [ContentVisibility.PRIVATE]: '仅自己可见',
  [ContentVisibility.TENANT]: '本校可见',
  [ContentVisibility.SHARED]: '共享资源库',
}

/** contentVisibilityLabel 返回可见范围文案。 */
export function contentVisibilityLabel(visibility: ContentVisibility): string {
  return VISIBILITY_LABELS[visibility]
}

/** CONTENT_VISIBILITIES 供表单渲染可见范围选项。 */
export const CONTENT_VISIBILITIES = [
  ContentVisibility.PRIVATE,
  ContentVisibility.TENANT,
  ContentVisibility.SHARED,
] as const

const AUTHOR_TYPE_LABELS: Record<ContentAuthorType, string> = {
  [ContentAuthorType.TEACHER]: '教师创建',
  [ContentAuthorType.SYSTEM]: '系统导入',
  [ContentAuthorType.EXTERNAL]: '外部来源',
}

/** contentAuthorTypeLabel 返回内容来源文案。 */
export function contentAuthorTypeLabel(type: ContentAuthorType): string {
  return AUTHOR_TYPE_LABELS[type]
}

const PAPER_MODE_LABELS: Record<PaperMode, string> = {
  [PaperMode.MANUAL]: '手动选题',
  [PaperMode.RANDOM]: '按条件抽题',
}

/** paperModeLabel 返回组卷方式文案。 */
export function paperModeLabel(mode: PaperMode): string {
  return PAPER_MODE_LABELS[mode]
}

/**
 * 题目正文按内容类型的字段说明。
 * 界面按类型渲染显式表单字段,不把 body 作为裸 JSON 文本域交给用户
 * (旧前端 16 处裸 JSON 是被审查列为 P0 的问题)。
 */
export const CONTENT_BODY_FIELD_LABELS = {
  statement: '题面正文',
  summary: '实验概述',
  description: '说明',
  scenario: '场景描述',
  question: '问题',
  explanation: '解析',
  steps: '操作步骤',
  options: '选项',
  runtime_code: '运行时',
  tools: '所需工具',
  submit_key: '答案提交字段',
} as const

/**
 * 竞赛题答案提交键在正文里的字段名(对齐清单 §6.19 定档)。
 * 它是表单字段名而不是答案,故不属敏感字段、随题面下发;取值必须与该题判题配置里的
 * flag_input_key 相同 —— 学生端按它构造提交体,判题器按同一个键取值。
 */
export const CONTENT_SUBMIT_KEY_FIELD = 'submit_key'
