// reorder.ts 章节与课时的「上移 / 下移」(设计 §7.4:顺序用按钮调整,键盘可用,改写现有序号字段)。
// 先按当前顺序(序号,再按原顺序)重新编为 1..n,再与相邻一项互换;只返回序号与已存值不同的项,
// 调用方逐条写回。旧数据里序号重复或有空档时,重新编号会把它们一并理顺。纯函数,供 node:test 直接运行。

/** 可排序的一项 */
export interface SortableItem {
  id: string
  sort: number
}

/** moveItem 把 id 对应的一项按 step(-1 上移 / +1 下移)与相邻一项互换,返回需要写回的 { id, sort } */
export function moveItem(items: readonly SortableItem[], id: string, step: 1 | -1): SortableItem[] {
  const ordered = items.map((item, index) => ({ item, index })).sort((a, b) => a.item.sort - b.item.sort || a.index - b.index)
  const position = ordered.findIndex((entry) => entry.item.id === id)
  const target = position + step
  if (position < 0 || target < 0 || target >= ordered.length) return []
  const order = ordered.map((entry) => entry.item)
  ;[order[position], order[target]] = [order[target], order[position]]
  const stored = new Map(items.map((item) => [item.id, item.sort]))
  const changed = order
    .map((item, index) => ({ id: item.id, sort: index + 1 }))
    .filter((item) => stored.get(item.id) !== item.sort)
  // 按传入顺序返回,便于调用方与测试预期一致
  const inputIndex = new Map(items.map((item, index) => [item.id, index]))
  return changed.sort((a, b) => (inputIndex.get(a.id) ?? 0) - (inputIndex.get(b.id) ?? 0))
}
