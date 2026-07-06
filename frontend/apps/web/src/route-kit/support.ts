// 路由支撑工具：隐藏资源页、空态、查询参数和默认时间范围。

import type { AppDefinition, DataColumn, ResourceResult } from '../app/types'

/**
 * hiddenResourceRoute 创建不进入侧栏的详情、流程或工作台入口。
 */
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

/**
 * resourceRoute 创建进入侧栏的角色主任务入口。
 */
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

/**
 * emptyResult 为缺少上级资源上下文的深页提供可解释空态。
 */
export function emptyResult(columns: DataColumn[], emptyTitle: string, emptyDescription: string): ResourceResult {
  return { metrics: [{ label: '记录数量', value: '0', tone: 'secondary' }], columns, rows: [], emptyTitle, emptyDescription }
}

/**
 * routeParam 按别名顺序读取 URL 参数，兼容同一深页的不同入口来源。
 */
export function routeParam(params: URLSearchParams, ...keys: string[]): string {
  for (const key of keys) {
    const value = params.get(key)
    if (value) return value
  }
  return ''
}

/**
 * defaultRange 返回统计页默认三十天查询窗口。
 */
export function defaultRange(): { from: string; to: string } {
  const to = new Date()
  const from = new Date(to.getTime() - 30 * 24 * 60 * 60 * 1000)
  return { from: from.toISOString(), to: to.toISOString() }
}
