// 路由结果工具：把后端列表、对象和工作台状态转换为四端统一页面数据。

import { AccountStatus, SessionStatus } from '@chaimir/api-client'
import { formatDate } from '@chaimir/shared'
import type { DataColumn, DataRow, MetricItem, PageAction, ResourceResult, WorkspaceResult, WorkspaceTool } from '@chaimir/shared'
import { dashboardColumns } from './columns'

type ObjectItem = Record<string, unknown>

/**
 * listResult 将后端分页响应转换为共享资源页可渲染的表格结果。
 */
export function listResult<T extends object>(
  response: { list: T[]; total?: number },
  columns: DataColumn[],
  emptyTitle: string,
  emptyDescription: string
): ResourceResult {
  const rows = toRows(response.list, (item, index) => normalizeObject(item, index, columns))
  return {
    metrics: [{ label: '记录总数', value: String(response.total ?? rows.length), tone: 'primary' }],
    columns,
    rows,
    emptyTitle,
    emptyDescription,
  }
}

/**
 * arrayResult 将数组响应转换为共享资源页可渲染的表格结果。
 */
export function arrayResult<T extends object>(
  items: T[],
  columns: DataColumn[],
  emptyTitle: string,
  emptyDescription: string
): ResourceResult {
  return {
    metrics: [{ label: '记录数量', value: String(items.length), tone: 'primary' }],
    columns,
    rows: toRows(items, (item, index) => normalizeObject(item, index, columns)),
    emptyTitle,
    emptyDescription,
  }
}

/**
 * objectResult 将详情对象转换为单行结果，适配深层详情页。
 */
export function objectResult<T extends object>(item: T, columns: DataColumn[], title: string): ResourceResult {
  return {
    metrics: [{ label: title, value: '已读取', tone: 'success' }],
    columns,
    rows: [normalizeObject(item, 0, columns)],
    emptyTitle: '暂无配置',
    emptyDescription: '配置保存后会在这里显示。',
  }
}

/**
 * dashboardResult 将管理看板聚合对象转换为指标和一行明细。
 */
export function dashboardResult(item: object): ResourceResult {
  const record = item as ObjectItem
  const metrics: MetricItem[] = [
    metric('账号总数', record.account_count, 'primary'),
    metric('课程数量', record.course_count, 'secondary'),
    metric('实验数量', record.experiment_count, 'success'),
    metric('活跃沙箱', record.active_sandbox_count, 'warning'),
  ]
  return {
    metrics,
    columns: dashboardColumns(),
    rows: [normalizeObject(item, 0, dashboardColumns())],
    emptyTitle: '暂无看板数据',
    emptyDescription: '业务数据生成后会在这里显示。',
  }
}

/**
 * workspaceInfo 生成沉浸式工作台结果，页面可按真实业务传入差异化面板。
 */
export function workspaceInfo(
  title: string,
  description: string,
  details: MetricItem[],
  tools?: WorkspaceTool[],
  actions?: PageAction[],
  panels?: WorkspaceResult['panels']
): WorkspaceResult {
  return {
    title,
    description,
    details,
    tools,
    actions,
    panels: panels ?? [
      { title: '说明', body: '左侧展示实验背景和阶段说明，中间展示当前实例状态，右侧展示检查点、判题与资源状态。' },
      { title: '交互', body: '运行、判题和回放状态由平台同步，页面提供清晰的加载、失败和重试路径。' },
      { title: '安全', body: '学生侧只读取题面和实例状态，不显示答案、判题设置或受保护信息。' },
    ],
  }
}

/**
 * toRows 使用调用方提供的映射器把任意对象数组转成 DataRow。
 */
export function toRows<T extends object>(items: T[], mapper: (item: T, index: number) => DataRow): DataRow[] {
  return items.map(mapper)
}

/**
 * normalizeObject 按列定义抽取对象字段，并统一做用户向展示值转换。
 */
function normalizeObject(item: object, index: number, columns: DataColumn[]): DataRow {
  const record = item as ObjectItem
  const row: DataRow = { id: idOf(record, index) }
  for (const column of columns) {
    row[column.key] = displayValue(record[column.key])
  }
  return row
}

/**
 * idOf 从资源对象中选取稳定标识，供表格和行操作使用。
 */
export function idOf(item: object, index: number): string {
  const record = item as ObjectItem
  return text(record.id ?? record.tenant_id ?? record.application_id ?? record.task_id ?? record.code ?? record.name ?? index)
}

/**
 * metric 创建标准指标项。
 */
function metric(label: string, value: unknown, tone: MetricItem['tone']): MetricItem {
  return { label, value: text(value ?? 0), tone }
}

/**
 * displayValue 把后端值转换为用户可读文本，避免把内部空值直接暴露出来。
 */
function displayValue(value: unknown): string {
  if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}/.test(value)) {
    return dateText(value)
  }
  if (typeof value === 'boolean') {
    return value ? '是' : '否'
  }
  if (typeof value === 'number') {
    return Number.isInteger(value) ? String(value) : value.toFixed(2)
  }
  if (Array.isArray(value)) {
    return value.length > 0 ? value.map((item) => text(item)).join('、') : '无'
  }
  if (value && typeof value === 'object') {
    return '已配置'
  }
  return text(value)
}

/**
 * text 提供资源页兜底文本，空值统一展示为未设置。
 */
export function text(value: unknown): string {
  if (value === null || value === undefined || value === '') {
    return '未设置'
  }
  return String(value)
}

/**
 * dateText 使用共享日期格式输出列表时间。
 */
export function dateText(value: string): string {
  return formatDate(value, 'YYYY-MM-DD HH:mm')
}

/**
 * accountStatusText 将账号状态枚举转换为用户可理解的状态。
 */
export function accountStatusText(value: unknown): string {
  const status = Number(value)
  if (status === AccountStatus.PENDING) return '待激活'
  if (status === AccountStatus.ACTIVE) return '正常'
  if (status === AccountStatus.DISABLED) return '已停用'
  if (status === AccountStatus.ARCHIVED) return '已归档'
  if (status === AccountStatus.CANCELLED) return '已注销'
  return text(value)
}

/**
 * sessionStatusText 将登录会话状态转换为用户可理解的状态。
 */
export function sessionStatusText(value: unknown): string {
  const status = Number(value)
  if (status === SessionStatus.ACTIVE) return '有效'
  if (status === SessionStatus.REVOKED) return '已退出'
  return text(value)
}
