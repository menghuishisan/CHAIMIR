// json.ts 提供页面表单与后端 JSON 对象字段之间的安全转换。

/**
 * parseJsonObject 把文本框中的 JSON 转换为后端可接收的对象。
 */
export function parseJsonObject<T = Record<string, unknown>>(value: string): T {
  const parsed = JSON.parse(value)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('请输入 JSON 对象。')
  }
  return parsed as T
}

/**
 * parseJsonArray 把文本框中的 JSON 转换为数组字段，并保留调用方的错误文案。
 */
export function parseJsonArray<T = unknown>(value: string, errorMessage: string, emptyValue?: T[]): T[] {
  if (emptyValue && !value.trim()) {
    return emptyValue
  }
  const parsed = JSON.parse(value)
  if (!Array.isArray(parsed)) {
    throw new Error(errorMessage)
  }
  return parsed as T[]
}

/**
 * stringifyJsonObject 把后端对象字段转换为易编辑的缩进 JSON。
 */
export function stringifyJsonObject<T = Record<string, unknown>>(value?: T | null): string {
  return JSON.stringify(value || {}, null, 2)
}

/** parseDelimitedList 把逗号或换行分隔的表单值转换为去重有序列表。 */
export function parseDelimitedList(value: string): string[] {
  return Array.from(new Set(value.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean)))
}
