// contentVersion.ts 题目版本号工具:已发布题目「建新版本并编辑」时自动取下一个空闲版本号。
// 已发布版本不可变(后端 UpdateDraftItem 只改草稿),所以改题 = 复制出新版本草稿再改。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/**
 * nextFreeVersion 从 current 的修订位加一开始,找第一个不在 existing 里的版本号;
 * current 不是「主.次.修订」三段数字时从 1.0.1 起找。大版本变更由出题人在「版本与复用」里自己定。
 */
export function nextFreeVersion(current: string, existing: readonly string[]): string {
  const taken = new Set(existing)
  const match = /^(\d+)\.(\d+)\.(\d+)$/.exec(current)
  const [major, minor, patch] = match ? [match[1], match[2], Number(match[3])] : ['1', '0', 0]
  for (let next = patch + 1; ; next += 1) {
    const candidate = `${major}.${minor}.${next}`
    if (!taken.has(candidate)) return candidate
  }
}
