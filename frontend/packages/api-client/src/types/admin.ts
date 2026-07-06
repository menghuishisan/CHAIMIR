// ===== M9 Admin 模块 =====

import type { AdminScope, AlertStatus, BackupStatus, BackupType } from '../constants/admin'
import type { ApplicationStatus, AuditActorRole, DeployMode, TenantStatus } from '../constants/identity'

export interface SystemConfig {
  id: string
  scope: AdminScope
  tenant_id?: string
  key: string
  value: Record<string, unknown>
  version: number
  updated_by: string
  updated_at: string
}

export interface ConfigUpdateRequest {
  scope: AdminScope
  tenant_id?: string
  value: Record<string, unknown>
  version: number
  change_log_id?: string
}

export interface ConfigRollbackRequest {
  scope: AdminScope
  tenant_id?: string
  version: number
  change_log_id: string
}

export interface ConfigChangeLog {
  id: string
  config_id: string
  tenant_id?: string
  old_value: Record<string, unknown>
  new_value: Record<string, unknown>
  operator_id: string
  created_at: string
}

export interface AlertRule {
  id: string
  scope: AdminScope
  tenant_id?: string
  name: string
  metric: string
  condition: Record<string, unknown>
  level: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface AlertRuleRequest {
  scope: AdminScope
  tenant_id?: string
  name: string
  metric: string
  condition: Record<string, unknown>
  level: number
  enabled: boolean
}

export interface AlertEvent {
  id: string
  rule_id: string
  tenant_id?: string
  level: number
  message: string
  status: AlertStatus
  handler_id?: string
  triggered_at: string
  handled_at?: string
}

export interface AlertEventRequest {
  status: AlertStatus
}

export interface Statistics {
  scope: AdminScope
  tenant_id?: string
  date: string
  metrics: Record<string, unknown>
}

export interface BackupRecord {
  id: string
  type: BackupType
  size_bytes: number
  status: BackupStatus
  started_at: string
  finished_at?: string
}

export interface Dashboard {
  scope: AdminScope
  tenant_id?: string
  tenant_count?: number
  account_count: number
  teacher_count: number
  student_count: number
  active_account_count: number
  course_count: number
  active_course_count: number
  experiment_count: number
  active_instance_count: number
  contest_count: number
  active_contest_count: number
  active_sandbox_count: number
  pending_apply_count?: number
  resource_quota_snapshot?: Record<string, unknown>
  generated_at: string
}

export interface MonitoringPanel {
  name: string
  url: string
}

export interface TenantSummary {
  tenant_id: string
  code: string
  name: string
  type: number
  status: TenantStatus
  deploy_mode: DeployMode
  expire_at?: string
  created_at: string
  updated_at: string
}

export interface TenantApplicationSummary {
  application_id: string
  school_name: string
  school_type: number
  contact_name: string
  contact_phone: string
  contact_email: string
  status: ApplicationStatus
  submitted_at: string
  reviewed_at?: string
}

export interface AuditLogEntry {
  id: string
  tenant_id?: string
  actor_id: string
  actor_role: AuditActorRole
  action: string
  target_type: string
  target_id?: string
  detail?: string
  ip?: string
  trace_id?: string
  created_at: string
}

export interface AuditQueryParams {
  actor_id?: string
  action?: string
  target_type?: string
  from?: string
  to?: string
  page?: number
  size?: number
}

export interface AuditQueryResult {
  list: AuditLogEntry[]
  total: number
  page: number
  size: number
}
