// shared 提供跨领域选择项所需的最小构造函数。

import type { SelectOption } from '@chaimir/ui'

/** option 将后端枚举值转成 Select 使用的字符串值。 */
export function option(value: string | number, label: string): SelectOption {
  return { value: String(value), label }
}

/** withAllOption 为列表筛选添加“全部”入口。 */
export function withAllOption(label: string, options: SelectOption[]): SelectOption[] {
  return [option('', label), ...options]
}
