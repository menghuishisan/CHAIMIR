// Content API：题库与模板中心,对应后端 M5 模块。

import { ApiClient } from '../client'
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
  PaginatedResponse,
  Paper,
  PaperDetail,
  UpdateItemRequest,
} from '../types'

/**
 * ContentApi 封装 M5 内容中心的前端调用。
 */
export class ContentApi {
  /**
   * constructor 注入统一 ApiClient,确保题库接口共用鉴权、trace_id 和错误处理。
   */
  constructor(private client: ApiClient) {}

  // getItems 查询内容列表。
  async getItems(params?: {
    type?: number
    category?: number
    difficulty?: number
    tag?: string
    kp?: string
    keyword?: string
    visibility?: number
    status?: number
    author?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<ContentItem>> {
    return this.client.get('/content/items', params)
  }

  // getItemFace 查询题面视角内容。
  async getItemFace(code: string, version: string): Promise<ContentItemSnapshot> {
    return this.client.get(`/content/items/${code}/${version}`)
  }

  // getItemFull 查询全量内容。
  async getItemFull(code: string, version: string): Promise<ContentItemSnapshot> {
    return this.client.get(`/content/items/${code}/${version}/full`)
  }

  // createItem 创建内容草稿。
  async createItem(data: CreateItemRequest): Promise<ContentItemSnapshot> {
    return this.client.post('/content/items', data)
  }

  // updateItem 更新内容草稿。
  async updateItem(itemId: string, data: UpdateItemRequest): Promise<ContentItemSnapshot> {
    return this.client.patch(`/content/items/${itemId}`, data)
  }

  // publishItem 发布内容。
  async publishItem(itemId: string): Promise<ContentItem> {
    return this.client.post(`/content/items/${itemId}/publish`)
  }

  // deprecateItem 弃用已发布内容。
  async deprecateItem(itemId: string): Promise<ContentItem> {
    return this.client.post(`/content/items/${itemId}/deprecate`)
  }

  // deleteItem 删除草稿内容。
  async deleteItem(itemId: string): Promise<void> {
    return this.client.delete(`/content/items/${itemId}`)
  }

  // getVersions 查询版本列表。
  async getVersions(code: string): Promise<ContentItem[]> {
    return this.client.get(`/content/items/${code}/versions`)
  }

  // createNewVersion 基于现有版本创建新草稿。
  async createNewVersion(code: string, data: NewVersionRequest): Promise<ContentItemSnapshot> {
    return this.client.post(`/content/items/${code}/new-version`, data)
  }

  // cloneItem 克隆内容为独立草稿。
  async cloneItem(code: string, version: string, data: CloneItemRequest): Promise<ContentItemSnapshot> {
    return this.client.post(`/content/items/${code}/${version}/clone`, data)
  }

  // shareItem 设为共享库可见。
  async shareItem(itemId: string): Promise<ContentItem> {
    return this.client.post(`/content/items/${itemId}/share`)
  }

  // unshareItem 取消共享库可见。
  async unshareItem(itemId: string): Promise<ContentItem> {
    return this.client.post(`/content/items/${itemId}/unshare`)
  }

  // listShared 浏览共享库。
  async listShared(params?: {
    type?: number
    keyword?: string
    page?: number
    size?: number
  }): Promise<PaginatedResponse<ContentItem>> {
    return this.client.get('/content/shared', params)
  }

  // uploadAttachment 上传题库附件。
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
    return this.client.postFormData('/content/attachments', formData, onProgress)
  }

  // issueAttachmentDownloadGrant 签发附件短时下载授权。
  async issueAttachmentDownloadGrant(
    data: ContentAttachmentDownloadGrantRequest
  ): Promise<ContentAttachmentDownloadGrant> {
    return this.client.post('/content/attachments/download-grant', data)
  }

  // listCategories 查询分类树。
  async listCategories(): Promise<ContentCategory[]> {
    return this.client.get('/content/categories')
  }

  // createCategory 创建分类。
  async createCategory(data: ContentCategoryRequest): Promise<ContentCategory> {
    return this.client.post('/content/categories', data)
  }

  // updateCategory 更新分类。
  async updateCategory(id: string, data: ContentCategoryRequest): Promise<ContentCategory> {
    return this.client.patch(`/content/categories/${id}`, data)
  }

  // deleteCategory 删除分类。
  async deleteCategory(id: string): Promise<ContentCategory> {
    return this.client.delete(`/content/categories/${id}`)
  }

  // listPapers 查询试卷分页。
  async listPapers(params?: { page?: number; size?: number }): Promise<PaginatedResponse<Paper>> {
    return this.client.get('/content/papers', params)
  }

  // createPaper 创建试卷。
  async createPaper(data: CreatePaperRequest): Promise<PaperDetail> {
    return this.client.post('/content/papers', data)
  }

  // getPaper 查询试卷详情。
  async getPaper(id: string): Promise<PaperDetail> {
    return this.client.get(`/content/papers/${id}`)
  }

  // regeneratePaper 重新随机组卷。
  async regeneratePaper(id: string): Promise<PaperDetail> {
    return this.client.post(`/content/papers/${id}/regenerate`)
  }
}
