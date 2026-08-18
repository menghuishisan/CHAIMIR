// content 领域维护题库表单使用的枚举选项顺序。

import { ContentDifficulty, ContentType, ContentVisibility } from '@chaimir/api-client'

/** CONTENT_TYPES 供内容表单按登记顺序渲染类型选项。 */
export const CONTENT_TYPES = [
  ContentType.EXPERIMENT_TEMPLATE,
  ContentType.CONTEST_PROBLEM,
  ContentType.THEORY_QUESTION,
] as const

/** CONTENT_DIFFICULTIES 供内容表单渲染难度选项。 */
export const CONTENT_DIFFICULTIES = [
  ContentDifficulty.INTRO,
  ContentDifficulty.BASIC,
  ContentDifficulty.ADVANCED,
  ContentDifficulty.CHALLENGE,
] as const

/** CONTENT_VISIBILITIES 供内容表单渲染可见范围选项。 */
export const CONTENT_VISIBILITIES = [
  ContentVisibility.PRIVATE,
  ContentVisibility.TENANT,
  ContentVisibility.SHARED,
] as const
