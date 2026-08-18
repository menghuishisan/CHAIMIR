// content labels 文件维护 M5 题库与模板中心枚举的用户向文案。

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

const STATUS_LABELS: Record<ContentStatus, string> = {
  [ContentStatus.DRAFT]: '草稿',
  [ContentStatus.PUBLISHED]: '已发布',
  [ContentStatus.DEPRECATED]: '已弃用',
}

/** contentStatusLabel 返回内容状态文案。 */
export function contentStatusLabel(status: ContentStatus): string {
  return STATUS_LABELS[status]
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
