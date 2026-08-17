// 判题契约常量：维护 M3 用户向状态字符串，来源为后端 judge statusText。

export const JUDGE_TASK_STATUS = {
  QUEUED: 'queued',
  JUDGING: 'judging',
  DONE: 'done',
  TIMEOUT: 'timeout',
  FAILED: 'failed',
  ERROR: 'error',
  CANCELLED: 'cancelled',
} as const

export type JudgeTaskStatus = (typeof JUDGE_TASK_STATUS)[keyof typeof JUDGE_TASK_STATUS]

/**
 * 判题任务的运维分组:与后端 judge enum.go 的 TaskState* 一一对应。
 * 一个分组覆盖多个状态,故列表筛选与计数用它而不是单个 status
 * (「需要处理」= 超时 + 失败 + 出错,分三次查会既慢又难对齐口径)。
 */
export const JUDGE_TASK_STATE = {
  ACTIVE: 'active',
  ABNORMAL: 'abnormal',
} as const

export type JudgeTaskState = (typeof JUDGE_TASK_STATE)[keyof typeof JUDGE_TASK_STATE]

export enum JudgerType {
  TESTCASE = 1,
  ONCHAIN_ASSERT = 2,
  FLAG = 3,
  STATIC_SCAN = 4,
  SIM_CHECKPOINT = 5,
  MANUAL = 6,
}

export enum JudgerStatus {
  AVAILABLE = 1,
  DISABLED = 2,
}

export enum JudgerSelftestStatus {
  PENDING = 1,
  PASSED = 2,
  FAILED = 3,
}

/** 判题任务的沙箱使用方式,与后端 contracts.JudgeSandboxMode 保持一致。 */
export const JUDGE_SANDBOX_MODE = {
  FRESH: 'fresh',
  REUSE: 'reuse',
} as const

export type JudgeSandboxMode = (typeof JUDGE_SANDBOX_MODE)[keyof typeof JUDGE_SANDBOX_MODE]
