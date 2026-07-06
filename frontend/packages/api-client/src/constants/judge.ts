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
