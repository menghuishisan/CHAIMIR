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
  body: Record<string, unknown>
  sensitive_fields?: string[]
}

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
  body: Record<string, unknown>
  sensitive_fields: string[]
}

export interface UpdateItemRequest {
  title: string
  category_id: SnowflakeID
  difficulty: ContentDifficulty
  tags: string[]
  knowledge_points: string[]
  visibility: ContentVisibility
  body: Record<string, unknown>
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
  body: Record<string, unknown>
}

export interface PaperDetail {
  paper: Paper
  items: PaperItemFace[]
}
