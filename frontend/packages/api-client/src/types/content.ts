// ===== M5 Content 模块 =====

import type { SnowflakeID } from './common'
import type {
  ContentAuthorType,
  ContentDifficulty,
  ContentStatus,
  ContentType,
  ContentVisibility,
  PaperMode,
} from '../constants/content'

export interface ContentItem {
  id: SnowflakeID
  tenant_id: SnowflakeID
  code: string
  version: string
  type: ContentType
  title: string
  category_id?: SnowflakeID
  difficulty: ContentDifficulty
  tags: string[]
  knowledge_points: string[]
  author_id: SnowflakeID
  author_type: ContentAuthorType
  visibility: ContentVisibility
  status: ContentStatus
  usage_count: number
  version_hash: string
  created_at: string
  updated_at: string
}

export interface ContentItemSnapshot extends ContentItem {
  body: ContentBody
  sensitive_fields?: string[]
}

/** ChainAssertion 描述链上判题的一条可读断言。 */
export interface ChainAssertion {
  label: string
  target: string
  field: string
  op: 'eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'contains'
  value: string | number | boolean
  expected_label: string
}

/** ContentJudgeConfig 是内容中心唯一的判题配置结构。 */
export interface ContentJudgeExpectation extends Record<string, unknown> {
  public?: boolean
  assertions?: ChainAssertion[]
}

export interface ContentJudgeConfig {
  judger_code: string
  suite_ref?: string
  max_score: number
  expectation: ContentJudgeExpectation
}

/** ExperimentTemplateBody 是实验模板的固定内容体。 */
export interface ExperimentTemplateBody {
  runtime_code: string
  tools: string[]
  init_code_ref: string
  sim_package_ref: string
  judge_config: ContentJudgeConfig
  description: string
  init_script: string
}

/** ContestProblemBody 是竞赛题的固定内容体。 */
export interface ContestProblemBody {
  statement: string
  judge_config: ContentJudgeConfig
  init_contracts: string[]
  ad_config?: {
    runtime_code: string
    runtime_image_version: string
    tool_codes: string[]
  }
}

/** TheoryQuestionBody 是理论题的固定内容体。 */
export interface TheoryQuestionBody {
  statement: string
  q_type: 'single_choice' | 'multiple_choice' | 'true_false' | 'fill_blank' | 'short_answer'
  options: string[]
  answer: string | string[] | boolean
  explanation: string
}

/** ContentBody 汇总三类互斥的内容体，不接受任意对象。 */
export type ContentBody = ExperimentTemplateBody | ContestProblemBody | TheoryQuestionBody

export interface CreateItemRequest {
  code: string
  version: string
  type: ContentType
  title: string
  category_id: SnowflakeID
  difficulty: ContentDifficulty
  tags: string[]
  knowledge_points: string[]
  visibility: ContentVisibility
  body: ContentBody
  sensitive_fields: string[]
}

export interface UpdateItemRequest {
  title: string
  category_id: SnowflakeID
  difficulty: ContentDifficulty
  tags: string[]
  knowledge_points: string[]
  visibility: ContentVisibility
  body: ContentBody
  sensitive_fields: string[]
}

export interface NewVersionRequest {
  source_version: string
  new_version: string
}

export interface CloneItemRequest {
  new_code: string
  new_version: string
}

export interface ContentCategory {
  id: SnowflakeID
  parent_id?: SnowflakeID
  name: string
  sort: number
  created_at: string
  updated_at: string
}

export interface ContentCategoryRequest {
  parent_id?: SnowflakeID
  name: string
  sort: number
}

export interface ContentAttachmentUpload {
  object_ref: string
  file_name: string
  size: number
}

export interface ContentAttachmentDownloadGrantRequest {
  resource_id: SnowflakeID
  object_ref: string
}

export interface ContentAttachmentDownloadGrant {
  token: string
  expires_at: string
}

export interface PaperCriteria {
  type?: ContentType
  difficulty?: ContentDifficulty[]
  knowledge_points?: string[]
  count?: number
  default_score?: number
}

export interface PaperItemInput {
  code: string
  version: string
  score: number
}

export interface CreatePaperRequest {
  name: string
  gen_mode: PaperMode
  gen_criteria: PaperCriteria
  items: PaperItemInput[]
}

export interface Paper {
  id: SnowflakeID
  name: string
  author_id: SnowflakeID
  gen_mode: PaperMode
  gen_criteria: PaperCriteria
  created_at: string
  updated_at: string
}

export interface PaperItemFace {
  id: SnowflakeID
  code: string
  version: string
  score: number
  seq: number
  item: ContentItem
  body: ContentBody
}

export interface PaperDetail {
  paper: Paper
  items: PaperItemFace[]
}
