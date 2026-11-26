// ===== 通用类型 =====

/** SnowflakeID 是浏览器公开契约中的十进制字符串资源标识。 */
export type SnowflakeID = string

/** 按维度统计全量可见记录。每组套用其他筛选，跳过本组筛选。 */
export type PageFacets = Record<string, Record<string, number>>

export interface PaginatedResponse<T> {
  list: T[]
  total: number
  page: number
  size: number
  /** 仅在该接口声明分段计数时出现。 */
  facets?: PageFacets
}
