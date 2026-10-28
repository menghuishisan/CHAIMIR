// useFormValidation 统一字段校验的展示范围,错误始终由当前输入计算,避免保留已修正的旧提示。

import { useCallback, useState } from 'react'

export type FormErrors = Record<string, string | null>

interface ValidationVisibility {
  scope: string | number
  /** null表示已校验全部字段,数组表示仅校验过其中字段。 */
  fields: string[] | null
}

/** useFormValidation 按失焦/提交激活提示,关联字段每次渲染同步重算;check只包含纯校验规则。 */
export function useFormValidation(check: () => Partial<FormErrors>, scope: string | number = '') {
  const [visibility, setVisibility] = useState<ValidationVisibility>({ scope, fields: [] })
  let currentVisibility = visibility
  // 切换表单或步骤时立即清除展示范围,返回旧步骤也不会复活先前错误。
  if (visibility.scope !== scope) {
    currentVisibility = { scope, fields: [] }
    setVisibility(currentVisibility)
  }

  const current = check()
  const errors: FormErrors = Object.fromEntries(
    (currentVisibility.fields ?? Object.keys(current)).map((key) => [key, current[key] ?? null]),
  )

  /** validate 校验全部或指定前置字段,合并展示范围并返回本次校验是否通过。 */
  const validate = useCallback((fields?: string[]): boolean => {
    const next = check()
    setVisibility((previous) => {
      const previousFields = previous.scope === scope ? previous.fields : []
      return {
        scope,
        fields: fields === undefined || previousFields === null
          ? null
          : [...new Set([...previousFields, ...fields])],
      }
    })
    return (fields ?? Object.keys(next)).every((key) => next[key] == null)
  }, [check, scope])

  /** reset 在提交成功清空输入等场景恢复尚未参与校验的展示状态。 */
  const reset = useCallback(() => setVisibility({ scope, fields: [] }), [scope])

  return { errors, validate, reset }
}
