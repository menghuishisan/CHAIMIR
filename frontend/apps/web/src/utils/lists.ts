// lists.ts 提供 apps/web 表单中分隔列表的统一归一化能力。

/** parseDelimitedList 把逗号或换行分隔的表单值转换为去重有序列表。 */
export function parseDelimitedList(value: string): string[] {
  return Array.from(new Set(value.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean)))
}
