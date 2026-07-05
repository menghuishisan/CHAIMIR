// 路由分页工具：集中维护四端资源页默认分页参数。

const DEFAULT_ROUTE_PAGE = 1
const DEFAULT_ROUTE_PAGE_SIZE = 20

/**
 * defaultPageParams 统一四端列表页的默认分页，避免页面散落页码常量。
 */
export function defaultPageParams(): { page: number; size: number } {
  return { page: DEFAULT_ROUTE_PAGE, size: DEFAULT_ROUTE_PAGE_SIZE }
}
