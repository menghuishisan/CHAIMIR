// 组合连接预览结果只对生成它的声明键有效。

export interface CompositionConnectionCheck {
  key: string
  complete: boolean
}

/** currentConnectionCheck 丢弃与当前声明键不匹配的预览结果。 */
export function currentConnectionCheck(
  check: CompositionConnectionCheck | undefined,
  currentKey: string,
): CompositionConnectionCheck | undefined {
  return check?.key === currentKey ? check : undefined
}
