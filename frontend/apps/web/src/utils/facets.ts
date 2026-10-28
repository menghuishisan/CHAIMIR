// facets 工具:读取后端分页响应里的全量分组计数。
//
// 为什么需要它:指标带的口径必须是服务端全量,不能用当前页切片数(规范 §6.5.4)。
// 后端为几张列表声明了聚合契约,把分组计数放在 `data.facets` 里一并返回 ——
// 这样一次请求就拿到「总量 + 各分组数」,不用为每个分组再打一次 total 探测请求。
//
// 键名与取值形状由后端固定(数值枚举按十进制字符串、布尔按 "true"/"false"、
// 字符串列按原值),故这里只做安全读取与求和,不猜测键名:
// 整个分组未返回时数量未知,只有已返回分组中的缺失取值才按 0 展示。

import type { PageFacets } from '@chaimir/api-client'

/** facetGroup 取出某一列的分组计数;未读取或未声明时保持未知。 */
export function facetGroup(facets: PageFacets | undefined, key: string): Record<string, number> | undefined {
  return facets?.[key]
}

/** facetCount 读取某个取值的计数;已知分组内缺失为0,整个分组缺失为未知。 */
export function facetCount(
  facets: PageFacets | undefined,
  key: string,
  value: string | number | boolean,
): number | undefined {
  const group = facetGroup(facets, key)
  return group ? group[String(value)] ?? 0 : undefined
}

/** facetSum 把某一列的全部分组计数相加,用于「已声明该列的记录总数」。 */
export function facetSum(facets: PageFacets | undefined, key: string): number | undefined {
  const group = facetGroup(facets, key)
  return group ? Object.values(group).reduce((sum, item) => sum + item, 0) : undefined
}

/**
 * facetTopEntries 按计数降序取出分组,供「最常见的几类」这种摘要使用。
 * limit 只裁剪展示条数,不改变计数口径。
 */
export function facetTopEntries(
  facets: PageFacets | undefined,
  key: string,
  limit: number,
): { value: string; count: number }[] | undefined {
  const group = facetGroup(facets, key)
  if (!group) return undefined
  return Object.entries(group)
    .map(([value, count]) => ({ value, count }))
    .sort((a, b) => b.count - a.count || a.value.localeCompare(b.value))
    .slice(0, limit)
}
