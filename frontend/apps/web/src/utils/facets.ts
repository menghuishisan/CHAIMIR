// facets 工具:读取后端分页响应里的全量分组计数。
//
// 为什么需要它:指标带的口径必须是服务端全量,不能用当前页切片数(规范 §6.5.4)。
// 后端为几张列表声明了聚合契约,把分组计数放在 `data.facets` 里一并返回 ——
// 这样一次请求就拿到「总量 + 各分组数」,不用为每个分组再打一次 total 探测请求。
//
// 键名与取值形状由后端固定(数值枚举按十进制字符串、布尔按 "true"/"false"、
// 字符串列按原值),故这里只做安全读取与求和,不猜测键名:
// 没有声明聚合契约的接口不会有 facets,读不到就按 0 展示。

import type { PageFacets } from '@chaimir/api-client'

/** facetGroup 取出某一列的分组计数;缺失即空对象。 */
export function facetGroup(facets: PageFacets | undefined, key: string): Record<string, number> {
  return facets?.[key] ?? {}
}

/** facetCount 取出某一列某个取值的计数;缺失即 0。 */
export function facetCount(
  facets: PageFacets | undefined,
  key: string,
  value: string | number | boolean,
): number {
  return facetGroup(facets, key)[String(value)] ?? 0
}

/** facetSum 把某一列的全部分组计数相加,用于「已声明该列的记录总数」。 */
export function facetSum(facets: PageFacets | undefined, key: string): number {
  return Object.values(facetGroup(facets, key)).reduce((sum, item) => sum + item, 0)
}

/**
 * facetTopEntries 按计数降序取出分组,供「最常见的几类」这种摘要使用。
 * limit 只裁剪展示条数,不改变计数口径。
 */
export function facetTopEntries(
  facets: PageFacets | undefined,
  key: string,
  limit: number,
): { value: string; count: number }[] {
  return Object.entries(facetGroup(facets, key))
    .map(([value, count]) => ({ value, count }))
    .sort((a, b) => b.count - a.count || a.value.localeCompare(b.value))
    .slice(0, limit)
}
