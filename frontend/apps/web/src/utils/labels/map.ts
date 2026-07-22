// map labels 文件提供领域文案映射的唯一内部辅助函数。

/** labelFromMap 按字符串键读取文案，并返回用户可理解的兜底提示。 */
export function labelFromMap(value: unknown, labels: Record<string, string>, fallback: string): string {
  const key = String(value)
  return Object.prototype.hasOwnProperty.call(labels, key) ? labels[key] : fallback
}
