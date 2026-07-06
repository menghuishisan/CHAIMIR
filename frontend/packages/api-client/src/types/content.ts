// ===== M5 Content 模块 =====

import type {
  ContentAuthorType,
  ContentDifficulty,
  ContentStatus,
  ContentType,
  ContentVisibility,
  PaperMode,
} from '../constants/content'

export interface ContentItem {
  id: number
  tenant_id: number
  code: string
  version: string
  type: ContentType
  title: string
  category_id?: number
  difficulty: ContentDifficulty
  tags: string[]
  knowledge_points: string[]
  author_id: number
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
  category_id: number
  difficulty: ContentDifficulty
  tags: string[]
  knowledge_points: string[]
  visibility: ContentVisibility
  body: Record<string, unknown>
  sensitive_fields: string[]
}

export interface UpdateItemRequest {
  title: string
  category_id: number
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
  id: number
  parent_id?: number
  name: string
  sort: number
  created_at: string
  updated_at: string
}

export interface ContentCategoryRequest {
  parent_id: number
  name: string
  sort: number
}

export interface ContentAttachmentUpload {
  object_ref: string
  file_name: string
  size: number
}

export interface ContentAttachmentDownloadGrantRequest {
  resource_id: string
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
  id: number
  name: string
  author_id: number
  gen_mode: PaperMode
  gen_criteria: PaperCriteria
  created_at: string
  updated_at: string
}

export interface PaperItemFace {
  id: number
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
