// contentFace.ts 把题面正文换成可读的小节(共享资源库预览用):只输出已登记的题面字段,按固定顺序,带用户看得懂的名称。
// 白名单而不是黑名单:答案、解析、判题配置、漏洞题的攻击步骤与判定断言,以及任何没登记的字段都不显示,
// 即使接口因为作者漏声明敏感字段而把它们带了出来;也不把内部字段名显示给用户。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 可以出现在题面里的字段与展示名,顺序即展示顺序 */
const FACE_FIELDS: readonly { key: string; label: string }[] = [
  { key: 'summary', label: '概述' },
  { key: 'steps', label: '操作步骤' },
  { key: 'scenario', label: '场景' },
  { key: 'description', label: '漏洞说明' },
  { key: 'statement', label: '题干' },
  { key: 'options', label: '选项' },
  { key: 'contract_source', label: '合约源码' },
  { key: 'fork_block', label: '分叉区块' },
]

/** 题面里的一个小节 */
export interface FaceSection {
  key: string
  label: string
  /** 每行一段;文字里自带的换行由展示方按 pre-wrap 保留 */
  lines: string[]
}

/** linesOf 把字段值换成文字行:字符串一行,数字一行,字符串数组每项一行;其余形状不显示 */
function linesOf(value: unknown): string[] {
  if (typeof value === 'string') return value.trim() === '' ? [] : [value]
  if (typeof value === 'number' && Number.isFinite(value)) return [String(value)]
  if (Array.isArray(value)) {
    return value.filter((item): item is string => typeof item === 'string' && item.trim() !== '')
  }
  return []
}

/** contentFaceSections 按登记顺序输出有内容的题面小节 */
export function contentFaceSections(body: Record<string, unknown>): FaceSection[] {
  const sections: FaceSection[] = []
  for (const field of FACE_FIELDS) {
    const lines = linesOf(body[field.key])
    if (lines.length > 0) sections.push({ key: field.key, label: field.label, lines })
  }
  return sections
}
