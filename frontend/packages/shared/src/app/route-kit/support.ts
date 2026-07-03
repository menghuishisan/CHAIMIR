// 路由支撑工具：隐藏资源页、空态、查询参数和默认时间范围。

import type { AppDefinition, DataColumn, ResourceResult } from '../types'

export function hiddenResourceRoute(
  path: string,
  label: string,
  description: string,
  icon: AppDefinition['routes'][number]['icon'],
  load: AppDefinition['routes'][number]['load'],
  group?: string
): AppDefinition['routes'][number] {
  return { path, label, description, icon, group, hidden: true, load }
}

export function resourceRoute(
  path: string,
  label: string,
  description: string,
  icon: AppDefinition['routes'][number]['icon'],
  load: AppDefinition['routes'][number]['load'],
  group: string
): AppDefinition['routes'][number] {
  return { path, label, description, icon, group, load }
}

export function emptyResult(columns: DataColumn[], emptyTitle: string, emptyDescription: string): ResourceResult {
  return { metrics: [{ label: '记录数量', value: '0', tone: 'secondary' }], columns, rows: [], emptyTitle, emptyDescription }
}

export function routeParam(params: URLSearchParams, ...keys: string[]): string {
  for (const key of keys) {
    const value = params.get(key)
    if (value) return value
  }
  return ''
}

export function defaultRange(): { from: string; to: string } {
  const to = new Date()
  const from = new Date(to.getTime() - 30 * 24 * 60 * 60 * 1000)
  return { from: from.toISOString(), to: to.toISOString() }
}
