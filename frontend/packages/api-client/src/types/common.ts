// ===== 通用类型 =====

export interface PaginatedResponse<T> {
  list: T[]
  total: number
  page: number
  size: number
}
