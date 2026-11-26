export enum UserRole {
  PLATFORM_ADMIN = 1,
  SCHOOL_ADMIN = 2,
  TEACHER = 3,
  STUDENT = 4,
}

/** 审计主体包含后台系统任务。 */
export enum AuditActorRole {
  PLATFORM_ADMIN = 1,
  SCHOOL_ADMIN = 2,
  TEACHER = 3,
  STUDENT = 4,
  SYSTEM = 5,
}

export enum TenantStatus {
  ACTIVE = 1,
  DISABLED = 2,
  EXPIRED = 3,
}

export enum DeployMode {
  SAAS = 1,
  SCHOOL = 2,
}

export enum AuthMode {
  LOCAL = 1,
  CAS = 2,
  LDAP = 3,
}

export enum ApplicationStatus {
  PENDING = 1,
  APPROVED = 2,
  REJECTED = 3,
}

export enum BaseIdentity {
  STUDENT = 1,
  TEACHER = 2,
}

export enum AccountStatus {
  PENDING = 1,
  ACTIVE = 2,
  DISABLED = 3,
  ARCHIVED = 4,
  CANCELLED = 5,
}

export enum SessionStatus {
  ACTIVE = 1,
  REVOKED = 2,
}

export enum SmsScene {
  LOGIN = 1,
  RESET = 2,
  CHANGE_PHONE = 3,
}

export enum SsoType {
  CAS = 1,
  LDAP = 2,
}

export enum SsoMatchField {
  NO = 1,
  PHONE = 2,
}

export enum ImportTarget {
  TEACHER = 1,
  STUDENT = 2,
  ORG = 3,
}

export const IMPORT_TEMPLATE_FORMAT = {
  XLSX: 'xlsx',
  CSV: 'csv',
} as const

export type ImportTemplateFormat = typeof IMPORT_TEMPLATE_FORMAT[keyof typeof IMPORT_TEMPLATE_FORMAT]

export enum ImportBatchStatus {
  PROCESSING = 1,
  COMPLETED = 2,
  FAILED = 3,
}

export enum ClassStatus {
  ACTIVE = 1,
  ARCHIVED = 2,
}

export const TENANT_MODULE = {
  TEACHING: 'teaching',
  EXPERIMENT: 'experiment',
  CONTEST: 'contest',
  SIM: 'sim',
  GRADE: 'grade',
} as const

export type TenantModule = typeof TENANT_MODULE[keyof typeof TENANT_MODULE]

/** TENANT_MODULE_VALUES 是页面按契约顺序渲染模块开关的唯一取值清单。 */
export const TENANT_MODULE_VALUES = Object.values(TENANT_MODULE)
