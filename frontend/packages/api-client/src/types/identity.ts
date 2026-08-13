// ===== M1 Identity 模块 =====

import type { SnowflakeID } from './common'
import type {
  AccountStatus,
  ApplicationStatus,
  AuthMode,
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
  remember: boolean
}

export interface LoginPhoneRequest {
  phone: string
  password: string
  tenant_id?: SnowflakeID
  remember: boolean
}

export interface LoginNoRequest {
  tenant_code: string
  no: string
  password: string
  remember: boolean
}

export interface LoginSMSRequest {
  phone: string
  code: string
  tenant_id?: SnowflakeID
  remember: boolean
}

export interface SendSMSRequest {
  phone: string
  scene: SmsScene
  tenant_id?: SnowflakeID
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
  /** 省略时由服务端按手机号归属定位学校;一号多校才需显式指定(与 SendSMSRequest 同一约定) */
  tenant_id?: SnowflakeID
}

export interface ActivateRequest {
  activation_code: string
  password: string
}

export interface LoginResponse {
  access_token?: string
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
  /** 校徽的对象引用,由 POST /tenant/logo 上传取得;不是可直接显示的地址 */
  logo_ref?: string
  /** 校徽的 data URI,只在读取租户配置与学校品牌时返回;可直接作为 img src */
  logo_image?: string
  display_name?: string
  feature_flags: Record<string, unknown>
  auth_mode: AuthMode
  enable_activation_code: boolean
  created_at: string
  updated_at: string
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
  display_name: string
  feature_flags: Record<string, unknown>
  auth_mode: AuthMode
  enable_activation_code: boolean
}

/**
 * 学校品牌:登录页免鉴权读取。
 * 只有学校私有部署才有内容 —— 平台托管的登录页面对的是尚未确定的学校,没有校徽可显示。
 */
export interface TenantBrand {
  display_name: string
  /** 校徽的 data URI;没有校徽或平台托管部署时为空串 */
  logo_image: string
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

/**
 * ClassStudent 是班内学生名录条目：只有编号、姓名与学号。
 * 刻意不复用 Account —— 那带手机号掩码、账号状态与角色，属学校管理员的账号目录字段。
 */
export interface ClassStudent {
  id: SnowflakeID
  name: string
  no?: string
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
