// configValue 收敛平台系统配置值的读取与比较口径。
// 配置值是开放 JSONB:凭据字段按键名识别并脱敏、任意值给一个短可读表达、
// 改动前后的差异按键比较。系统配置页与配置变更历史页共用这一套判定,
// 避免同一口径在两处各写一遍(键名判定错一处就等于把密钥打到界面上)。

/** 后端脱敏后填回的占位文案(secretmap.MaskedValue),识别它是为了绝不把它提交回去。 */
export const MASKED_VALUE = '已配置'

/**
 * 凭据类字段的词根,与后端 privacy.credentialKeyMarkers 一致。
 * 键名里包含任一词根即被后端加密保存并脱敏返回,故前端按同一口径识别 ——
 * 判据不一致会导致某个字段的掩码值被当成普通值提交,把真实密钥覆盖掉。
 */
const CREDENTIAL_KEY_MARKERS = [
  'password',
  'passwd',
  'private_key',
  'privatekey',
  'access_key',
  'accesskey',
  'signing_key',
  'signingkey',
  'session_secret',
  'sessionsecret',
  'secret',
  'token',
  'credential',
  'authorization',
  'api_key',
  'apikey',
] as const

/** isCredentialKey 判断键名是否带凭据语义,口径与后端一致。 */
export function isCredentialKey(key: string): boolean {
  const normalized = key.trim().toLowerCase()
  return CREDENTIAL_KEY_MARKERS.some((marker) => normalized.includes(marker))
}

/** credentialKeysOf 列出配置里所有凭据字段的键名(升序,便于稳定渲染)。 */
export function credentialKeysOf(value: Record<string, unknown>): string[] {
  return Object.keys(value).filter(isCredentialKey).sort()
}

/** withoutKeys 去掉指定键,得到可以安全放进编辑器的部分。 */
export function withoutKeys(
  value: Record<string, unknown>,
  keys: string[],
): Record<string, unknown> {
  const drop = new Set(keys)
  return Object.fromEntries(Object.entries(value).filter(([key]) => !drop.has(key)))
}

/** describeValue 给任意配置值一个短的可读表达,不打印整段结构。 */
export function describeValue(raw: unknown): string {
  if (typeof raw === 'string') return raw === '' ? '(空)' : raw
  if (typeof raw === 'number' || typeof raw === 'boolean') return String(raw)
  if (Array.isArray(raw)) return `${raw.length} 项`
  if (raw === null || raw === undefined) return '未设置'
  return `${Object.keys(raw as Record<string, unknown>).length} 个子项`
}

/** summarizeValue 把配置值摊成可读条目;凭据字段只说「已配置」。 */
export function summarizeValue(value: Record<string, unknown>): Array<{
  term: string
  description: string
  mono?: boolean
}> {
  return Object.entries(value).map(([key, raw]) => {
    if (isCredentialKey(key)) return { term: key, description: '已配置(不显示)' }
    return { term: key, description: describeValue(raw), mono: true }
  })
}

/** changedKeys 比较改动前后的键值,列出真正变化的键。 */
export function changedKeys(
  oldValue: Record<string, unknown>,
  newValue: Record<string, unknown>,
): string[] {
  const keys = new Set([...Object.keys(oldValue), ...Object.keys(newValue)])
  return [...keys]
    .filter((key) => JSON.stringify(oldValue[key]) !== JSON.stringify(newValue[key]))
    .sort()
}

/** ParsedValue 是配置内容文本的解析结果。 */
export interface ParsedValue {
  value?: Record<string, unknown>
  error?: string
}

/**
 * parseValue 解析配置内容。
 * 后端要求 value 是一个对象且非空(nil 会被拒),故这两条在本地先判定。
 */
export function parseValue(text: string): ParsedValue {
  const trimmed = text.trim()
  if (trimmed === '') return { error: '配置内容不能为空。' }

  let raw: unknown
  try {
    raw = JSON.parse(trimmed)
  } catch {
    return { error: '内容不是合法的配置格式,检查是否漏了逗号或引号。' }
  }
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
    return { error: '配置内容最外层要是一个对象。' }
  }
  return { value: raw as Record<string, unknown> }
}
