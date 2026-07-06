// 管理契约常量：维护前端需要与后端 admin 模块枚举编号对齐的值。

export enum AdminScope {
  GLOBAL = 1,
  TENANT = 2,
}

export enum AlertStatus {
  PENDING = 1,
  HANDLED = 2,
  IGNORED = 3,
}

export enum BackupType {
  FULL = 1,
}

export enum BackupStatus {
  RUNNING = 1,
  SUCCEEDED = 2,
  FAILED = 3,
}
