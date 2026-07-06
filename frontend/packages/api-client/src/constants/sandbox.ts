// 沙箱契约常量：维护前端需要与后端 sandbox/contracts 枚举编号对齐的值。

export enum SandboxPhase {
  ALLOCATING = 1,
  READY = 2,
  INITIALIZING = 3,
  FULLY_READY = 4,
}

export enum SandboxStatus {
  CREATING = 1,
  RUNNING = 2,
  PAUSED = 3,
  RECYCLING = 4,
  DESTROYED = 5,
  FAILED = 6,
  READY = 7,
  IDLE = 8,
}

export enum SandboxToolKind {
  BUILTIN = 1,
  TERMINAL = 2,
  WEB_EMBED = 3,
  COMMAND = 4,
}

export enum RuntimeStatus {
  AVAILABLE = 1,
  ONBOARDING = 2,
  DISABLED = 3,
}

export enum RuntimeSelftestStatus {
  PENDING = 1,
  PASSED = 2,
  FAILED = 3,
}

export enum RuntimeImageStatus {
  AVAILABLE = 1,
  DISABLED = 2,
}

export enum ImagePrepullStatus {
  PENDING = 1,
  SUCCEEDED = 2,
  FAILED = 3,
  RUNNING = 4,
}

export enum ToolStatus {
  AVAILABLE = 1,
  DISABLED = 2,
}

export enum SandboxToolStatus {
  READY = 1,
  STARTING = 2,
  FAILED = 3,
}
