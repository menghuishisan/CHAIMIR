// ===== 通用类型 =====

/** SnowflakeID 是浏览器公开契约中的十进制字符串资源标识。 */
export type SnowflakeID = string

export interface PaginatedResponse<T> {
  list: T[]
  total: number
  page: number
  size: number
}
