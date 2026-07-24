// theme 读取 IDE 外部依赖所需的设计令牌值。

/** cssColor 返回颜色令牌的计算值，缺失时立即暴露配置错误。 */
export function cssColor(token: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(token).trim()
  if (!value) throw new Error(`缺少前端颜色令牌 ${token}`)
  return value
}
