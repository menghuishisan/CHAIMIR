// 身份契约常量：维护前端需要与后端 identity 模块枚举编号对齐的值。

/**
 * 用户角色编号与后端 identity 模块保持一致。
 */
export enum UserRole {
  PLATFORM_ADMIN = 1,
  SCHOOL_ADMIN = 2,
  TEACHER = 3,
  STUDENT = 4,
}

/**
 * 审计主体角色与后端 audit.ActorRoleFromAccount/ActorRoleSystem 保持一致。
 */
export enum AuditActorRole {
  PLATFORM_ADMIN = 1,
  SCHOOL_ADMIN = 2,
  TEACHER = 3,
  STUDENT = 4,
  SYSTEM = 5,
}

/**
 * 租户状态与后端 identity 模块保持一致。
 */
export enum TenantStatus {
  ACTIVE = 1,
  DISABLED = 2,
  EXPIRED = 3,
}

/**
 * 租户部署模式与后端 identity 模块保持一致。
 */
export enum DeployMode {
  SAAS = 1,
  SCHOOL = 2,
}

/**
 * 租户认证模式与后端 identity 模块保持一致。
 */
export enum AuthMode {
  LOCAL = 1,
  CAS = 2,
  LDAP = 3,
}

/**
 * 入驻申请状态与后端 identity 模块保持一致。
 */
export enum ApplicationStatus {
  PENDING = 1,
  APPROVED = 2,
  REJECTED = 3,
}

/**
 * 账号基础身份与后端 identity 模块保持一致。
 */
export enum BaseIdentity {
  STUDENT = 1,
  TEACHER = 2,
}

/**
 * 账号状态与后端 identity 模块保持一致。
 */
export enum AccountStatus {
  PENDING = 1,
  ACTIVE = 2,
  DISABLED = 3,
  ARCHIVED = 4,
  CANCELLED = 5,
}

/**
 * 登录会话状态与后端 identity 模块保持一致。
 */
export enum SessionStatus {
  ACTIVE = 1,
  REVOKED = 2,
}

/**
 * 短信验证码场景与后端 identity 模块保持一致。
 */
export enum SmsScene {
  LOGIN = 1,
  RESET = 2,
  CHANGE_PHONE = 3,
}

/**
 * SSO 类型与后端 identity 模块保持一致。
 */
export enum SsoType {
  CAS = 1,
  LDAP = 2,
}

/**
 * SSO 匹配字段与后端 identity 模块保持一致。
 */
export enum SsoMatchField {
  NO = 1,
  PHONE = 2,
}

/**
 * 导入目标类型与后端 identity 模块保持一致。
 */
export enum ImportTarget {
  TEACHER = 1,
  STUDENT = 2,
  ORG = 3,
}

/**
 * 导入模板下载格式与后端 OpenAPI FormatQuery 保持一致。
 */
export const IMPORT_TEMPLATE_FORMAT = {
  XLSX: 'xlsx',
  CSV: 'csv',
} as const

export type ImportTemplateFormat = typeof IMPORT_TEMPLATE_FORMAT[keyof typeof IMPORT_TEMPLATE_FORMAT]

/**
 * 导入批次状态与后端 identity 模块保持一致。
 */
export enum ImportBatchStatus {
  PROCESSING = 1,
  COMPLETED = 2,
  FAILED = 3,
}

/**
 * 班级状态与后端 identity 模块保持一致。
 */
export enum ClassStatus {
  ACTIVE = 1,
  ARCHIVED = 2,
}
