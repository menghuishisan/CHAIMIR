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
