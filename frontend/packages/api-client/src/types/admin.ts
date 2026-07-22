// ===== M9 Admin 模块 =====

import type { AdminScope, AlertStatus, BackupStatus, BackupType } from '../constants/admin'
import type { ApplicationStatus, AuditActorRole } from '../constants/identity'
import type { SnowflakeID } from './common'

export interface SystemConfig {
  id: SnowflakeID
  scope: AdminScope
  tenant_id?: SnowflakeID
  key: string
  value: Record<string, unknown>
  version: number
  updated_by: SnowflakeID
  updated_at: string
}

export interface ConfigUpdateRequest {
  scope: AdminScope
  tenant_id?: SnowflakeID
  value: Record<string, unknown>
  version: number
  change_log_id?: SnowflakeID
}

export interface ConfigRollbackRequest {
  scope: AdminScope
  tenant_id?: SnowflakeID
  version: number
  change_log_id: SnowflakeID
}

export interface ConfigChangeLog {
  id: SnowflakeID
  config_id: SnowflakeID
  tenant_id?: SnowflakeID
  old_value: Record<string, unknown>
  new_value: Record<string, unknown>
  operator_id: SnowflakeID
  created_at: string
}

export interface AlertRule {
  id: SnowflakeID
  scope: AdminScope
  tenant_id?: SnowflakeID
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
  tenant_id?: SnowflakeID
  name: string
  metric: string
  condition: Record<string, unknown>
  level: number
  enabled: boolean
}

export interface AlertEvent {
  id: SnowflakeID
  rule_id: SnowflakeID
  tenant_id?: SnowflakeID
  level: number
  message: string
  status: AlertStatus
  handler_id?: SnowflakeID
  triggered_at: string
  handled_at?: string
}

export interface AlertEventRequest {
  status: AlertStatus
}

export interface Statistics {
  scope: AdminScope
  tenant_id?: SnowflakeID
  date: string
  metrics: Record<string, unknown>
}

export interface BackupRecord {
  id: SnowflakeID
  type: BackupType
  size_bytes: number
  status: BackupStatus
  started_at: string
  finished_at?: string
}

export interface Dashboard {
  scope: AdminScope
  tenant_id?: SnowflakeID
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

export interface TenantApplicationSummary {
  application_id: SnowflakeID
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
  id: SnowflakeID
  tenant_id?: SnowflakeID
  actor_id: SnowflakeID
  actor_role: AuditActorRole
  action: string
  target_type: string
  target_id?: SnowflakeID
  detail?: string
  ip?: string
  trace_id?: string
  created_at: string
}

export interface AuditQueryParams {
  actor_id?: SnowflakeID
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
