// ===== M1 Identity 模块 =====

import type { SnowflakeID } from './common'
import type {
  AccountStatus,
  ApplicationStatus,
  AuthMode,
  AuditActorRole,
  BaseIdentity,
  ClassStatus,
  DeployMode,
  ImportBatchStatus,
  ImportTarget,
  SessionStatus,
  SmsScene,
  SsoMatchField,
  SsoType,
  TenantStatus,
  UserRole,
} from '../constants/identity'

export interface LoginPlatformRequest {
  username: string
  password: string
}

export interface LoginPhoneRequest {
  phone: string
  password: string
  tenant_id?: SnowflakeID
}

export interface LoginNoRequest {
  tenant_code: string
  no: string
  password: string
}

export interface LoginSMSRequest {
  phone: string
  code: string
  tenant_id?: SnowflakeID
}

export interface SendSMSRequest {
  phone: string
  scene: SmsScene
  tenant_id?: SnowflakeID
}

export interface RefreshRequest {
  refresh_token: string
}

export interface WebSocketTicketRequest {
  path: string
}

export interface WebSocketTicketResponse {
  ticket: string
  expires_at: string
}

export interface PasswordResetRequest {
  phone: string
  code: string
  new_password: string
  tenant_id: SnowflakeID
}

export interface ActivateRequest {
  activation_code: string
  password: string
}

export interface LoginResponse {
  access_token?: string
  refresh_token?: string
  must_change_pwd?: boolean
  need_select_tenant?: boolean
  tenants?: TenantOption[]
  account?: Account
}

export interface TenantOption {
  tenant_id: SnowflakeID
  name: string
  code: string
}

export interface Account {
  id: SnowflakeID
  tenant_id?: SnowflakeID
  name: string
  phone_masked?: string
  no?: string
  base_identity: BaseIdentity
  roles: UserRole[]
  status: AccountStatus
  title?: string
  created_at?: string
}

export interface MeResponse {
  account: Account
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

export interface ChangePhoneRequest {
  phone: string
  code: string
}

export interface Session {
  id: SnowflakeID
  device_info?: string
  ip?: string
  status: SessionStatus
  expire_at: string
  created_at: string
}

export interface AuditLog {
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

export interface CreateApplicationRequest {
  school_name: string
  school_type: number
  contact_name: string
  contact_phone: string
  contact_email: string
}

export interface Tenant {
  id: SnowflakeID
  code: string
  name: string
  type: number
  status: TenantStatus
  deploy_mode: DeployMode
  expire_at?: string
  logo_url?: string
  display_name?: string
  auth_mode: AuthMode
  enable_activation_code: boolean
}

export interface TenantApplication {
  application_id: SnowflakeID
  school_name: string
  school_type: number
  contact_name: string
  contact_phone: string
  contact_email: string
  status: ApplicationStatus
  reject_reason?: string
  reviewed_by?: SnowflakeID
  tenant_id?: SnowflakeID
  submitted_at: string
  reviewed_at?: string
}

export interface ReviewApplicationRequest {
  tenant_code?: string
  admin_name?: string
  admin_phone?: string
  reason?: string
}

export interface UpdateTenantStatusRequest {
  status: TenantStatus
  expire_at?: string
}

export interface TenantConfigRequest {
  logo_url: string
  display_name: string
  auth_mode: AuthMode
  enable_activation_code: boolean
}

export interface SSOConfig {
  id: SnowflakeID
  tenant_id: SnowflakeID
  type: SsoType
  config: Record<string, unknown>
  match_field: SsoMatchField
  enabled: boolean
}

export interface SSOConfigRequest {
  type: SsoType
  config: Record<string, unknown>
  match_field: SsoMatchField
  enabled: boolean
}

export interface LDAPLoginRequest {
  username: string
  password: string
}

export interface DepartmentRequest {
  name: string
  code: string
}

export interface Department {
  id: SnowflakeID
  tenant_id: SnowflakeID
  name: string
  code: string
}

export interface MajorRequest {
  department_id: SnowflakeID
  name: string
}

export interface Major {
  id: SnowflakeID
  tenant_id: SnowflakeID
  department_id: SnowflakeID
  name: string
}

export interface ClassRequest {
  major_id: SnowflakeID
  name: string
  enrollment_year: number
  status: ClassStatus
}

export interface Class {
  id: SnowflakeID
  tenant_id: SnowflakeID
  major_id: SnowflakeID
  name: string
  enrollment_year: number
  status: ClassStatus
}

export interface ArchiveClassesRequest {
  enrollment_year: number
}

export interface PromoteClassesRequest {
  class_ids: SnowflakeID[]
  target_year: number
}

export interface CreateAccountRequest {
  phone: string
  name: string
  no: string
  base_identity: BaseIdentity
  org_id: SnowflakeID
  enrollment_year?: number
  title?: string
  initial_password?: string
  use_activation: boolean
}

export interface UpdateAccountRequest {
  name: string
  org_id: SnowflakeID
  enrollment_year?: number
  title?: string
}

export interface CreateAccountResponse {
  account: Account
  activation_code?: string
}

export interface AdminResetPasswordRequest {
  new_password: string
  must_change_pwd: boolean
}

export interface BatchAccountIDsRequest {
  account_ids: SnowflakeID[]
}

export interface ImportPreviewResponse {
  preview_id: SnowflakeID
  total: number
  valid: number
  invalid: number
  rows: ImportRowResult[]
}

export interface ImportRowResult {
  line: number
  error?: string
}

export interface ImportCommitRequest {
  preview_id: SnowflakeID
}

export interface ImportBatch {
  id: SnowflakeID
  tenant_id: SnowflakeID
  operator_id: SnowflakeID
  target_type: ImportTarget
  file_name: string
  total: number
  success: number
  failed: number
  status: ImportBatchStatus
  created_at: string
}

export interface ImportActivationCode {
  account_id: SnowflakeID
  no: string
  name: string
  activation_code: string
}

export interface AccountImportCommitResponse {
  batch: ImportBatch
  activation_codes?: ImportActivationCode[]
}
