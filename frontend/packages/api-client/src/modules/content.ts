// 封装题库、共享资源、分类和组卷接口。
import { ApiClient, encodePathSegment } from '../client'
import type {
  ContentDifficulty,
  ContentStatus,
  ContentType,
  ContentVisibility,
} from '../constants/content'
import type { PaginatedResponse, SnowflakeID } from '../types/common'
import type {
  CloneItemRequest,
  ContentAttachmentDownloadGrant,
  ContentAttachmentDownloadGrantRequest,
  ContentAttachmentUpload,
  ContentCategory,
  ContentCategoryRequest,
  ContentItem,
  ContentItemSnapshot,
  CreateItemRequest,
  CreatePaperRequest,
  NewVersionRequest,
  Paper,
  PaperDetail,
  UpdateItemRequest,
} from '../types/content'

/**
 * ContentApi 封装 M5 内容中心的前端调用。
 */
export class ContentApi {
  /**
   * constructor 注入统一 ApiClient,确保题库接口共用鉴权、trace_id 和错误处理。
   */
  constructor(private client: ApiClient) {}

  /** getItems 按条件查询题目；author 指向作者账号，不是来源枚举。 */
  async getItems(params?: {
    type?: ContentType
    category?: SnowflakeID
    difficulty?: ContentDifficulty
    tag?: string
    kp?: string
    keyword?: string
    visibility?: ContentVisibility
    status?: ContentStatus
    author?: SnowflakeID
    page?: number
    size?: number
  }): Promise<PaginatedResponse<ContentItem>> {
    return this.client.get('/content/items', params)
  }

  async getItemFace(code: string, version: string): Promise<ContentItemSnapshot> {
    return this.client.get(
      `/content/items/${encodePathSegment(code)}/${encodePathSegment(version)}`
    )
  }

  async getItemFull(code: string, version: string): Promise<ContentItemSnapshot> {
    return this.client.get(
      `/content/items/${encodePathSegment(code)}/${encodePathSegment(version)}/full`
    )
  }

  async createItem(data: CreateItemRequest): Promise<ContentItemSnapshot> {
    return this.client.post('/content/items', data)
  }

  async updateItem(itemId: string, data: UpdateItemRequest): Promise<ContentItemSnapshot> {
    return this.client.patch(`/content/items/${encodePathSegment(itemId)}`, data)
  }

  async publishItem(itemId: string): Promise<ContentItem> {
    return this.client.post(`/content/items/${encodePathSegment(itemId)}/publish`)
  }

  async deprecateItem(itemId: string): Promise<ContentItem> {
    return this.client.post(`/content/items/${encodePathSegment(itemId)}/deprecate`)
  }

  async deleteItem(itemId: string): Promise<void> {
    return this.client.delete(`/content/items/${encodePathSegment(itemId)}`)
  }

  async getVersions(code: string): Promise<ContentItem[]> {
    return this.client.get(`/content/items/${encodePathSegment(code)}/versions`)
  }

  async createNewVersion(code: string, data: NewVersionRequest): Promise<ContentItemSnapshot> {
    return this.client.post(`/content/items/${encodePathSegment(code)}/new-version`, data)
  }

  /** cloneItem 按所选内容 ID 复制，避免同名版本混淆来源。 */
  async cloneItem(itemId: SnowflakeID, data: CloneItemRequest): Promise<ContentItemSnapshot> {
    return this.client.post(`/content/items/${encodePathSegment(itemId)}/clone`, data)
  }

  async shareItem(itemId: string): Promise<ContentItem> {
    return this.client.post(`/content/items/${encodePathSegment(itemId)}/share`)
  }

  async unshareItem(itemId: string): Promise<ContentItem> {
    return this.client.post(`/content/items/${encodePathSegment(itemId)}/unshare`)
  }

  /** getSharedItemFace 精确预览仍在共享库中的题面，不读取答案。 */
  async getSharedItemFace(itemId: SnowflakeID): Promise<ContentItemSnapshot> {
    return this.client.get(`/content/shared/${encodePathSegment(itemId)}`)
  }

  async listShared(params?: {
    type?: ContentType
    keyword?: string
    page?: number
    size?: number
  }): Promise<PaginatedResponse<ContentItem>> {
    return this.client.get('/content/shared', params)
  }

  async uploadAttachment(
    file: File,
    resourceId?: string,
    onProgress?: (progress: number) => void
  ): Promise<ContentAttachmentUpload> {
    const formData = new FormData()
    formData.append('file', file)
    if (resourceId) {
      formData.append('resource_id', resourceId)
    }
    return this.client.sendFormData('POST', '/content/attachments', formData, onProgress)
  }

  async issueAttachmentDownloadGrant(
    data: ContentAttachmentDownloadGrantRequest
  ): Promise<ContentAttachmentDownloadGrant> {
    return this.client.post('/content/attachments/download-grant', data)
  }

  async listCategories(): Promise<ContentCategory[]> {
    return this.client.get('/content/categories')
  }

  async createCategory(data: ContentCategoryRequest): Promise<ContentCategory> {
    return this.client.post('/content/categories', data)
  }

  async updateCategory(id: string, data: ContentCategoryRequest): Promise<ContentCategory> {
    return this.client.patch(`/content/categories/${encodePathSegment(id)}`, data)
  }

  async deleteCategory(id: string): Promise<ContentCategory> {
    return this.client.delete(`/content/categories/${encodePathSegment(id)}`)
  }

  async listPapers(params?: { page?: number; size?: number }): Promise<PaginatedResponse<Paper>> {
    return this.client.get('/content/papers', params)
  }

  async createPaper(data: CreatePaperRequest): Promise<PaperDetail> {
    return this.client.post('/content/papers', data)
  }

  async getPaper(id: string): Promise<PaperDetail> {
    return this.client.get(`/content/papers/${encodePathSegment(id)}`)
  }

  async regeneratePaper(id: string): Promise<PaperDetail> {
    return this.client.post(`/content/papers/${encodePathSegment(id)}/regenerate`)
  }
}
