export type DynamicSpec = Record<string, unknown>

export function asRecord(value: unknown): DynamicSpec | undefined {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as DynamicSpec)
    : undefined
}

export function recordOrEmpty(value: unknown): DynamicSpec {
  return asRecord(value) ?? {}
}

/** stringValue 保留空字符串；仅在类型不匹配时使用缺省值。 */
export function stringValue(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback
}

export function numberText(value: unknown, fallback = ''): string {
  return typeof value === 'number' && Number.isFinite(value) ? String(value) : fallback
}

export function stringArray(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string')
    : []
}

export function validCommand(command: string[]): boolean {
  return (
    command.length > 0 &&
    command.every((argument) => argument.trim() !== '' && !/[\0\r\n]/.test(argument))
  )
}
