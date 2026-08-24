// ===== 通用类型 =====

/** SnowflakeID 是浏览器公开契约中的十进制字符串资源标识。 */
export type SnowflakeID = string

/**
 * PageFacets 是分页响应里的全量分组计数。
 *
 * 后端在同一事务、同一可见性边界内算出,故它是**全量口径**而不是当前页的切片统计 ——
 * 这正是规范 §6.5.4 要求指标必须满足的条件。外层键是分组维度(如 `action`、`scope`),
 * 内层键是该维度的取值(枚举值按十进制字符串给出),值是条数。
 *
 * 只有后端明确声明了聚合契约的列表才会带这个字段(见 `docs/对齐-后端待补齐清单-2026-08-23.md` §6.2),
 * 其余分页接口保持原形状,故这里是可选的;前端不得对没有契约的接口猜测 facets。
 */
export type PageFacets = Record<string, Record<string, number>>

export interface PaginatedResponse<T> {
  list: T[]
  total: number
  page: number
  size: number
  /** 全量分组计数;仅在后端为该列表声明了聚合契约时出现 */
  facets?: PageFacets
}
