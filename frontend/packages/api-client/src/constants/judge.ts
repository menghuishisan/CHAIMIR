export const JUDGE_TASK_STATUS = {
  QUEUED: 'queued',
  JUDGING: 'judging',
  DONE: 'done',
  FAILED: 'failed',
  CANCELLED: 'cancelled',
} as const

export type JudgeTaskStatus = (typeof JUDGE_TASK_STATUS)[keyof typeof JUDGE_TASK_STATUS]

/** 业务结果投影使用 M3 的数值状态，任务列表继续使用字符串状态。 */
export enum JudgeTaskStatusCode {
  QUEUED = 1,
  JUDGING = 2,
  DONE = 3,
  FAILED = 4,
  CANCELLED = 5,
}

/** 运维筛选按活跃或失败分组，计数由服务端提供。 */
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
  THEORY_QUESTION = 7,
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
