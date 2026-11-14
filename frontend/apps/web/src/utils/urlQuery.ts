// urlQuery.ts 把列表页的筛选、搜索、标签页与地址参数互相转换(纯函数,供 useUrlState 与 usePagedResource 使用)。
// 地址参数来自用户可以手改的地址栏,属于外部输入:不认识的值一律回到默认值,而不是让页面拿着它去请求后端。
// 本文件供 node:test 直接运行,不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 一个地址参数的规则:默认值,以及可选的合法取值清单(不给则任意文本都接受,如搜索词)。 */
export interface QueryFieldSpec {
  defaultValue: string
  allowed?: readonly string[]
}

/** 一个页面全部地址参数的规则,键即参数名(英文小写短词,如 status、q、tab)。 */
export type QuerySchema = Record<string, QueryFieldSpec>

/** 按规则读出的参数值,每个键都有值(缺失时是默认值)。 */
export type QueryValues<S extends QuerySchema> = { [K in keyof S]: string }

/** normalizeValue 去掉首尾空白;空值或不在合法清单里的值视为默认值。 */
function normalizeValue(raw: string | null, spec: QueryFieldSpec): string {
  const value = (raw ?? '').trim()
  if (value === '') return spec.defaultValue
  if (spec.allowed && !spec.allowed.includes(value)) return spec.defaultValue
  return value
}

/** readQuery 按规则读取地址参数;不认识的值回到默认值。 */
export function readQuery<S extends QuerySchema>(params: URLSearchParams, schema: S): QueryValues<S> {
  const values = {} as QueryValues<S>
  for (const key of Object.keys(schema) as (keyof S & string)[]) {
    values[key] = normalizeValue(params.get(key), schema[key])
  }
  return values
}

/**
 * writeQuery 把筛选的变化写回地址参数,返回新的参数对象(不改动传入的那一个)。
 * 等于默认值的参数从地址里去掉,地址保持最短;不属于本规则的参数(如另一个组件的 tab)原样保留。
 * resetKeys 用于筛选变化时清掉页码:筛选后停在第 5 页往往越界,用户看到的是「没有数据」。
 */
export function writeQuery<S extends QuerySchema>(
  params: URLSearchParams,
  schema: S,
  patch: Partial<QueryValues<S>>,
  resetKeys: readonly string[] = [],
): URLSearchParams {
  const next = new URLSearchParams(params)
  for (const key of Object.keys(patch) as (keyof S & string)[]) {
    const spec = schema[key]
    if (!spec) continue
    const value = normalizeValue(patch[key] ?? null, spec)
    if (value === spec.defaultValue) next.delete(key)
    else next.set(key, value)
  }
  for (const key of resetKeys) next.delete(key)
  return next
}

/** readPage 读取页码:只接受正整数,其余(缺失、0、负数、小数、科学计数法、文字)都回到第一页。 */
export function readPage(params: URLSearchParams, key: string): number {
  const raw = params.get(key) ?? ''
  return /^[1-9]\d*$/.test(raw) ? Number(raw) : 1
}
