import type { PageFacets } from '@chaimir/api-client'

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
 * segmentCounts 把一组分段换成各自的条数:分段取值由 valueOf 给出(「全部」返回 undefined,条数是该组之和)。
 * 计数来自列表接口的 facets(接口总览「分段计数」),不为每个分段各发一次取总数的请求;
 * 列表还没读到或接口没返回这一组时,每个分段都是未知。
 */
export function segmentCounts<S extends { value: string }>(
  facets: PageFacets | undefined,
  key: string,
  segments: readonly S[],
  valueOf: (segment: S) => string | number | boolean | undefined,
): Record<string, number | undefined> {
  const out: Record<string, number | undefined> = {}
  for (const segment of segments) {
    const value = valueOf(segment)
    out[segment.value] = value === undefined ? facetSum(facets, key) : facetCount(facets, key, value)
  }
  return out
}
