// Content API：题库、模板中心
// 对应后端 M5 模块

import { ApiClient } from '../client'
import type {
  ContentItem,
  ContentItemSnapshot,
  CreateItemRequest,
  UpdateItemRequest,
  ItemRef,
  PaginatedResponse,
} from '../types'

export class ContentApi {
  constructor(private client: ApiClient) {}

  /**
   * 获取内容列表
   */
  async getItems(params?: {
    type?: number
    category_id?: string
    difficulty?: number
    tags?: string[]
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<ContentItem>> {
    return this.client.get('/content/items', params)
  }

  /**
   * 获取内容详情（仅元信息）
   */
  async getItem(itemId: string): Promise<ContentItem> {
    return this.client.get(`/content/items/${itemId}`)
  }

  /**
   * 获取内容快照（含题面，学生视角）
   */
  async getItemSnapshot(code: string, version: string): Promise<ContentItemSnapshot> {
    return this.client.get(`/content/items/${code}/versions/${version}/snapshot`)
  }

  /**
   * 获取内容全量（含答案，教师视角）
   */
  async getItemFull(code: string, version: string): Promise<ContentItemSnapshot> {
    return this.client.get(`/content/items/${code}/versions/${version}/full`)
  }

  /**
   * 创建内容草稿
   */
  async createItem(data: CreateItemRequest): Promise<ContentItem> {
    return this.client.post('/content/items', data)
  }

  /**
   * 更新内容草稿
   */
  async updateItem(itemId: string, data: UpdateItemRequest): Promise<ContentItem> {
    return this.client.put(`/content/items/${itemId}`, data)
  }

  /**
   * 发布内容
   */
  async publishItem(itemId: string): Promise<void> {
    return this.client.post(`/content/items/${itemId}/publish`)
  }

  /**
   * 删除内容
   */
  async deleteItem(itemId: string): Promise<void> {
    return this.client.delete(`/content/items/${itemId}`)
  }

  /**
   * 创建新版本
   */
  async createNewVersion(
    code: string,
    data: { source_version: string; new_version: string }
  ): Promise<ContentItem> {
    return this.client.post(`/content/items/${code}/versions`, data)
  }

  /**
   * 克隆内容
   */
  async cloneItem(
    code: string,
    version: string,
    data: { new_code: string; new_version: string }
  ): Promise<ContentItem> {
    return this.client.post(`/content/items/${code}/versions/${version}/clone`, data)
  }

  /**
   * 批量获取题面（内部接口）
   */
  async batchGetItems(items: ItemRef[]): Promise<ContentItemSnapshot[]> {
    return this.client.post('/content/internal/batch-items', { items })
  }
}
