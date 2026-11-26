// 为题目新版本计算未占用的修订号。

/**
 * nextFreeVersion 从 current 的修订位加一开始,找第一个不在 existing 里的版本号;
 * current 不是「主.次.修订」三段数字时从 1.0.1 起找。大版本变更由出题人在「版本与复用」里自己定。
 */
export function nextFreeVersion(current: string, existing: readonly string[]): string {
  const taken = new Set(existing)
  const match = /^(\d+)\.(\d+)\.(\d+)$/.exec(current)
  const prefix = match ? `${match[1]}.${match[2]}` : '1.0'
  // 大修订号也必须每次前进，避免浮点精度使候选值不变。
  for (let next = match ? BigInt(match[3]) + 1n : 1n; ; next += 1n) {
    const candidate = `${prefix}.${next}`
    if (!taken.has(candidate)) return candidate
  }
}
