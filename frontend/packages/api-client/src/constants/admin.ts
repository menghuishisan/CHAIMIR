export enum AdminScope {
  GLOBAL = 1,
  TENANT = 2,
}

export enum AlertLevel {
  NOTICE = 1,
  WARNING = 2,
  SEVERE = 3,
  CRITICAL = 4,
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
  SUCCEEDED = 1,
  FAILED = 2,
}
