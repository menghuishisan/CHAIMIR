// merkle 提供内置教学算法共享的成对折叠规则。

/** merkleRoot 逐层成对合并摘要，奇数层复制最后一个摘要。 */
export function merkleRoot(hashes: string[], parentHash: (left: string, right: string) => string): string {
  let level = hashes
  while (level.length > 1) {
    const padded = level.length % 2 === 0 ? level : [...level, level[level.length - 1]]
    const next: string[] = []
    for (let index = 0; index < padded.length; index += 2) next.push(parentHash(padded[index], padded[index + 1]))
    level = next
  }
  return level[0] || ''
}
