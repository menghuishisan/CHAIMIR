// 路由分页工具：集中维护四端资源页默认分页参数。

const DEFAULT_ROUTE_PAGE = 1
const DEFAULT_ROUTE_PAGE_SIZE = 20

/**
 * defaultPageParams 统一四端列表页的默认分页，并从当前路由读取页码。
 */
export function defaultPageParams(params?: URLSearchParams): { page: number; size: number } {
  const source = params ?? currentRouteParams()
  return {
    page: positiveInteger(source.get('page'), DEFAULT_ROUTE_PAGE),
    size: positiveInteger(source.get('size'), DEFAULT_ROUTE_PAGE_SIZE),
  }
}

/**
 * currentRouteParams 提取当前 hash 路由查询参数，供现有列表路由无侵入接入分页。
 */
function currentRouteParams(): URLSearchParams {
  if (typeof window === 'undefined') {
    return new URLSearchParams()
  }
  const [, queryPart] = window.location.hash.replace(/^#\/?/, '').split('?')
  return new URLSearchParams(queryPart ?? '')
}

/**
 * positiveInteger 只接受正整数页码和页大小，非法输入回到统一默认值。
 */
function positiveInteger(value: string | null, fallback: number): number {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback
}
