// 仿真契约常量：维护 M4 用户向字符串枚举，来源为后端 sim 转换层。

/**
 * 仿真执行位置。后端按包的作者类型派生，不是提交时的可选项：
 * 平台内置包在浏览器 Worker 内运行，教师与第三方扩展包在后端隔离容器内运行。
 */
export const SIM_COMPUTE = {
  BROWSER: 'browser',
  ISOLATED: 'isolated',
} as const

export type SimCompute = (typeof SIM_COMPUTE)[keyof typeof SIM_COMPUTE]

export const SIM_PACKAGE_STATUS = {
  DRAFT: 'draft',
  REVIEWING: 'reviewing',
  PUBLISHED: 'published',
  ARCHIVED: 'archived',
  REJECTED: 'rejected',
} as const

export type SimPackageStatus = (typeof SIM_PACKAGE_STATUS)[keyof typeof SIM_PACKAGE_STATUS]

export const SIM_REVIEW_RESULT = {
  PENDING: 'pending',
  APPROVED: 'approved',
  REJECTED: 'rejected',
} as const

export type SimReviewResult = (typeof SIM_REVIEW_RESULT)[keyof typeof SIM_REVIEW_RESULT]

/** 仿真包自动校验子项的封闭结果值。 */
export const SIM_VALIDATION_STATUS = {
  PASSED: 'passed',
  FAILED: 'failed',
} as const

export type SimValidationStatusValue =
  (typeof SIM_VALIDATION_STATUS)[keyof typeof SIM_VALIDATION_STATUS]

/**
 * 隔离执行 WebSocket 的客户端命令类型（后端 `modules/sim/enum.go` 的 BackendCommandKind 同源）。
 * 四种之外服务端一律拒绝：推进一个推演时刻、注入一次包内声明的交互、回退一步、回到初始状态。
 */
export const SIM_STREAM_COMMAND = {
  STEP: 'step',
  EVENT: 'event',
  BACK: 'back',
  RESTART: 'restart',
} as const

export type SimStreamCommand = (typeof SIM_STREAM_COMMAND)[keyof typeof SIM_STREAM_COMMAND]

/**
 * 隔离执行 WebSocket 的服务端帧类型：首帧带包自描述信息，其后只带教学快照。
 */
export const SIM_STREAM_FRAME = {
  READY: 'ready',
  SNAPSHOT: 'snapshot',
} as const

export type SimStreamFrame = (typeof SIM_STREAM_FRAME)[keyof typeof SIM_STREAM_FRAME]
