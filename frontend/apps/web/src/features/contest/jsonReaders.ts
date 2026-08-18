// 竞赛动态 JSON 字段读取工具。
//
// 竞赛题面、证据和漏洞源配置允许后端按协议扩展字段，页面只读取声明过的
// 基础类型。这里集中处理类型收窄，避免各竞赛页面复制同一套开放对象读取逻辑。

/** 从开放对象读取字符串字段；类型不符时返回空字符串。 */
export function readString(source: Record<string, unknown> | undefined, key: string): string {
  const value = source?.[key]
  return typeof value === 'string' ? value : ''
}

/** 从开放对象读取有限数字字段；类型不符时返回调用方指定的默认值。 */
export function readNumber(
  source: Record<string, unknown> | undefined,
  key: string,
  fallback = 0,
): number {
  const value = source?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

/** 从开放对象读取字符串数组；数组元素类型不符时过滤掉。 */
export function readStringArray(source: Record<string, unknown> | undefined, key: string): string[] {
  const value = source?.[key]
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
}
