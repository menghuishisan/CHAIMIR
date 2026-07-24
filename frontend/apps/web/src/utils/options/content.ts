// content 定义内容中心页面使用的选择项。

import { ContentDifficulty, ContentType, ContentVisibility, PaperMode } from '@chaimir/api-client'
import { contentDifficultyLabel, contentTypeLabel, contentVisibilityLabel, paperModeLabel } from '../labels'
import { option } from './shared'

export const contentTypeOptions = [option(ContentType.EXPERIMENT_TEMPLATE, contentTypeLabel(ContentType.EXPERIMENT_TEMPLATE)), option(ContentType.CONTEST_PROBLEM, contentTypeLabel(ContentType.CONTEST_PROBLEM)), option(ContentType.THEORY_QUESTION, contentTypeLabel(ContentType.THEORY_QUESTION))]
export const contentDifficultyOptions = [option(ContentDifficulty.INTRO, contentDifficultyLabel(ContentDifficulty.INTRO)), option(ContentDifficulty.BASIC, contentDifficultyLabel(ContentDifficulty.BASIC)), option(ContentDifficulty.ADVANCED, contentDifficultyLabel(ContentDifficulty.ADVANCED)), option(ContentDifficulty.CHALLENGE, contentDifficultyLabel(ContentDifficulty.CHALLENGE))]
export const contentVisibilityOptions = [option(ContentVisibility.PRIVATE, contentVisibilityLabel(ContentVisibility.PRIVATE)), option(ContentVisibility.TENANT, contentVisibilityLabel(ContentVisibility.TENANT)), option(ContentVisibility.SHARED, contentVisibilityLabel(ContentVisibility.SHARED))]
export const paperModeOptions = [option(PaperMode.MANUAL, paperModeLabel(PaperMode.MANUAL)), option(PaperMode.RANDOM, paperModeLabel(PaperMode.RANDOM))]
