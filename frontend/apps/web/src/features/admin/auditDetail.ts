// auditDetail.ts 把审计记录的 detail(各模块写入的 JSON 文本)整理成详情抽屉里的几行说明。
// detail 的键名由各模块自取,是开放结构:界面只展示登记过的项、按固定顺序,内部编号(课程编号、来源引用等)不展示;
// 状态、学生这类要查表才能说清的项,由调用方按动作给出文案,给不出就不展示,不把原始值抛到界面上。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

import { formatFileSize } from '../../utils/formatters.ts'

/** 详情里的一行 */
export interface AuditDetailRow {
  term: string
  value: string
  /** 编码类的值用等宽字体 */
  mono: boolean
}

/** 需要查表的项:键为 detail 里的键名;返回 undefined 表示这一项不展示 */
export type AuditDetailLookups = Partial<Record<string, (value: unknown) => string | undefined>>

type Format = (value: unknown) => string | undefined

/** 一个登记过的项:键名、显示名,以及值的整理方式(不给 format 的由调用方查表) */
interface DetailField {
  key: string
  term: string
  format?: Format
  mono?: boolean
}

/** text 非空文字,去掉首尾空白 */
const text: Format = (value) =>
  typeof value === 'string' && value.trim() !== '' ? value.trim() : undefined
/** count 非负整数,按千分位写出 */
const count: Format = (value) =>
  typeof value === 'number' && Number.isInteger(value) && value >= 0
    ? value.toLocaleString('zh-CN')
    : undefined
/** version 正整数写成「第 N 版」,文字版本号原样写出 */
const version: Format = (value) =>
  typeof value === 'number' && Number.isInteger(value) && value > 0 ? `第 ${value} 版` : text(value)
/** fileSize 字节数写成易读的文件大小 */
const fileSize: Format = (value) =>
  typeof value === 'number' && value > 0 ? formatFileSize(value) : undefined
/** year 年份写成「2026 级」 */
const year: Format = (value) =>
  typeof value === 'number' && Number.isInteger(value) && value >= 1900 && value <= 2999
    ? `${value} 级`
    : undefined
/** listSize 非空列表写成「共 N 个」 */
const listSize: Format = (value) =>
  Array.isArray(value) && value.length > 0 ? `共 ${value.length} 个` : undefined
/** yesNo 布尔写成「是 / 否」 */
const yesNo: Format = (value) => (typeof value === 'boolean' ? (value ? '是' : '否') : undefined)

/** 登记过的项,也是展示顺序 */
const FIELDS: readonly DetailField[] = [
  { key: 'name', term: '名称', format: text },
  { key: 'code', term: '编码', format: text, mono: true },
  { key: 'item_code', term: '题目编码', format: text, mono: true },
  { key: 'source_code', term: '来源编码', format: text, mono: true },
  { key: 'version', term: '版本', format: version },
  { key: 'file_name', term: '文件名', format: text },
  { key: 'size', term: '文件大小', format: fileSize },
  { key: 'enrollment_year', term: '入学年份', format: year },
  { key: 'target_year', term: '升届后年份', format: year },
  { key: 'class_ids', term: '涉及班级', format: listSize },
  { key: 'semester', term: '学期', format: text },
  { key: 'success', term: '成功条数', format: count },
  { key: 'failed', term: '失败条数', format: count },
  { key: 'count', term: '涉及条数', format: count },
  { key: 'enabled', term: '是否启用', format: yesNo },
  { key: 'note', term: '说明', format: text },
  { key: 'status', term: '变更后的状态' },
  { key: 'level', term: '级别' },
  { key: 'metric', term: '指标' },
  { key: 'key', term: '配置项' },
  { key: 'student_id', term: '学生' },
]

/** 依次尝试作为「对象名」的项:名称、编码、题目编码、文件名 */
const HINT_KEYS = ['name', 'code', 'item_code', 'file_name'] as const

/**
 * parseDetail 解析 detail 文本;不是 JSON 对象时返回 undefined。
 * detail 由后端写入,解析失败只可能是历史数据格式不同:此时详情里不展示附加项,其余字段照常显示。
 */
function parseDetail(detail: string | undefined): Record<string, unknown> | undefined {
  if (!detail) return undefined
  let raw: unknown
  try {
    raw = JSON.parse(detail)
  } catch {
    return undefined
  }
  return typeof raw === 'object' && raw !== null && !Array.isArray(raw)
    ? (raw as Record<string, unknown>)
    : undefined
}

/** auditDetailRows 把 detail 整理成详情行:只含登记过且值有效的项,顺序固定 */
export function auditDetailRows(
  detail: string | undefined,
  lookups: AuditDetailLookups = {}
): AuditDetailRow[] {
  const values = parseDetail(detail)
  if (!values) return []
  const rows: AuditDetailRow[] = []
  for (const field of FIELDS) {
    if (!(field.key in values)) continue
    const format = field.format ?? lookups[field.key]
    const value = format?.(values[field.key])
    if (value !== undefined && value !== '')
      rows.push({ term: field.term, value, mono: field.mono ?? false })
  }
  return rows
}

/** auditDetailText 取 detail 里某一项的文字值(只认非空字符串);没有或不是文字时返回 undefined */
export function auditDetailText(detail: string | undefined, key: string): string | undefined {
  return text(parseDetail(detail)?.[key])
}

/** auditObjectHint 取 detail 里能说明「是哪一个对象」的文字(如课程名、题目编码),没有时返回 undefined */
export function auditObjectHint(detail: string | undefined): string | undefined {
  const values = parseDetail(detail)
  if (!values) return undefined
  for (const key of HINT_KEYS) {
    const value = text(values[key])
    if (value) return value
  }
  return undefined
}
