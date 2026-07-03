// 四端应用类型：定义角色、导航、页面和后端资源渲染契约。

import type { ChaimirApi } from '@chaimir/api-client'
import type { LucideIcon } from 'lucide-react'

export type AppRole = 'student' | 'teacher' | 'school-admin' | 'platform-admin'

export interface MetricItem {
  label: string
  value: string
  tone?: 'primary' | 'secondary' | 'success' | 'warning' | 'danger'
}

export interface DataColumn {
  key: string
  title: string
  priority?: 'primary' | 'secondary' | 'optional'
  align?: 'start' | 'center' | 'end'
}

export interface DataRow extends Record<string, unknown> {
  id: string
}

export type ActionFieldType = 'text' | 'number' | 'password' | 'textarea' | 'datetime-local' | 'file'

export interface ActionField {
  name: string
  label: string
  type: ActionFieldType
  required?: boolean
  placeholder?: string
  helper?: string
}

export type ActionValues = Record<string, string | File>

export interface PageAction {
  key: string
  label: string
  description: string
  fields: ActionField[]
  submitLabel: string
  execute: (values: ActionValues) => Promise<string>
}

export interface RowAction {
  key: string
  label: string
  description: string
  execute: (row: DataRow) => Promise<string>
}

export interface ResourceResult {
  metrics?: MetricItem[]
  columns: DataColumn[]
  rows: DataRow[]
  actions?: PageAction[]
  rowActions?: RowAction[]
  emptyTitle: string
  emptyDescription: string
}

export interface WorkspaceResult {
  title: string
  description: string
  details: MetricItem[]
  panels: Array<{
    title: string
    body: string
  }>
}

export interface AppRoute {
  path: string
  label: string
  description: string
  icon: LucideIcon
  immersive?: boolean
  hidden?: boolean
  load: (api: ChaimirApi, params: URLSearchParams) => Promise<ResourceResult | WorkspaceResult>
}

export interface AppDefinition {
  role: AppRole
  title: string
  subtitle: string
  homePath: string
  routes: AppRoute[]
}
