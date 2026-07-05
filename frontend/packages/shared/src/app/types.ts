// 四端应用类型：定义角色、导航、页面和后端资源渲染契约。

import type { ChaimirApi } from '@chaimir/api-client'
import type { ReactElement } from 'react'
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
  pagination?: {
    page: number
    size: number
    total: number
    totalPages: number
  }
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
  tools?: WorkspaceTool[]
  actions?: PageAction[]
}

export interface WorkspaceTool {
  key: string
  label: string
  description: string
  kind: 'terminal' | 'web' | 'command' | 'file' | 'chain' | 'sim' | 'status'
  href?: string
}

export interface AppRouteRenderContext {
  api: ChaimirApi
  params: URLSearchParams
  route: AppRoute
  app: AppDefinition
  refresh: () => void
}

export interface AppRoute {
  path: string
  label: string
  description: string
  icon: LucideIcon
  group?: string
  immersive?: boolean
  hidden?: boolean
  load: (api: ChaimirApi, params: URLSearchParams) => Promise<ResourceResult | WorkspaceResult>
  render?: (context: AppRouteRenderContext) => ReactElement
}

export interface AppDefinition {
  role: AppRole
  title: string
  subtitle: string
  homePath: string
  routes: AppRoute[]
}
