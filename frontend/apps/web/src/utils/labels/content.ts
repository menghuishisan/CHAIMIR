// content labels 文件维护内容资源、题目和组卷领域文案。

import { ContentDifficulty, ContentStatus, ContentType, ContentVisibility, PaperMode } from '@chaimir/api-client'
import { labelFromMap } from './map'

/** contentTypeLabel 返回内容中心资源类型文案。 */
export function contentTypeLabel(type: ContentType): string {
  return labelFromMap(type, {
    [ContentType.EXPERIMENT_TEMPLATE]: '实验模板', [ContentType.CONTEST_PROBLEM]: '竞赛题',
    [ContentType.THEORY_QUESTION]: '理论题',
  }, '资源')
}

/** contentDifficultyLabel 返回内容难度文案。 */
export function contentDifficultyLabel(value: ContentDifficulty): string {
  return labelFromMap(value, {
    [ContentDifficulty.INTRO]: '入门', [ContentDifficulty.BASIC]: '基础',
    [ContentDifficulty.ADVANCED]: '进阶', [ContentDifficulty.CHALLENGE]: '挑战',
  }, '未知')
}

/** contentVisibilityLabel 返回内容可见范围文案。 */
export function contentVisibilityLabel(value: ContentVisibility): string {
  return labelFromMap(value, {
    [ContentVisibility.PRIVATE]: '仅自己可见', [ContentVisibility.TENANT]: '校内可见',
    [ContentVisibility.SHARED]: '共享库可见',
  }, '未识别的可见范围')
}

/** contentStatusLabel 返回内容发布状态文案。 */
export function contentStatusLabel(status: ContentStatus): string {
  return labelFromMap(status, {
    [ContentStatus.DRAFT]: '草稿', [ContentStatus.PUBLISHED]: '已发布', [ContentStatus.DEPRECATED]: '已停用',
  }, '未识别的内容状态')
}

/** paperModeLabel 返回试卷组卷模式文案。 */
export function paperModeLabel(mode: PaperMode): string {
  return labelFromMap(mode, { [PaperMode.MANUAL]: '手动组卷', [PaperMode.RANDOM]: '规则组卷' }, '未识别的组卷模式')
}
