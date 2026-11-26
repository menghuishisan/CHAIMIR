// 仿真包、审核记录和会话的接口数据。

import type {
  SimCompute,
  SimPackageStatus,
  SimReviewResult,
  SimStreamCommand,
  SimValidationStatusValue,
} from '../constants/sim'
import type { SnowflakeID } from './common'

/** 仿真包协议固定的三项规模上限,与 M4 manifest meta.scale_limit 一致。 */
interface SimScaleLimit {
  nodes: number
  max_tick: number
  max_events: number
}

export interface SimPackageMeta {
  /** 最近一次退回的审核意见;只在「我提交的」列表里已退回的包上给出 */
  reject_comment?: string
  id: SnowflakeID
  code: string
  version: string
  name: string
  category: string
  compute: SimCompute
  scale_limit: SimScaleLimit
  bundle_hash?: string
  status: SimPackageStatus
  created_at: string
  updated_at: string
}

/** 扩展包在隔离环境运行，由服务端决定执行位置。 */
export interface SimPackageSubmit {
  bundle: File
  code: string
  version: string
  name: string
  category: string
  scale_limit: SimScaleLimit
}

export interface SimPackageSubmissionResult extends SimPackageMeta {
  review: SimPackageReview
}

export interface SimPackageReviewDetail {
  package: SimPackageMeta
  review: SimPackageReview
}

export interface SimValidationStatus {
  status?: SimValidationStatusValue
  message?: string
}

/** 审核报告和隔离执行流共用同一套教学快照协议。 */
export interface SimTeachingSnapshot {
  tick: number
  state: Record<string, unknown>
  view: Record<string, unknown>
  current_step?: Record<string, unknown>
  interaction_availability: Record<string, boolean>
  checkpoint_results: Record<string, unknown>
}

/**
 * 隔离执行 WebSocket 的客户端命令报文。
 * 只有 `event` 带事件名与载荷:推进、回退与重来由服务端按会话过程自行推算。
 */
export interface SimStreamCommandMessage {
  type: SimStreamCommand
  event_type?: string
  payload?: Record<string, unknown>
}

interface SimStaticScanReport {
  status?: SimValidationStatusValue
  findings?: string[]
}

/** 样例帧供审核人判断教学内容，自动校验验证可运行性和确定性。 */
export interface SimValidationReport {
  bundle_hash?: string
  metadata_validation?: SimValidationStatus
  static_scan?: SimStaticScanReport
  determinism_check?: SimValidationStatus
  worker_preview?: SimValidationStatus
  preview_frames?: SimTeachingSnapshot[]
}

export interface SimPackageReview {
  id: SnowflakeID
  package_id: SnowflakeID
  submitter_id: SnowflakeID
  /** 提交教师姓名;只在平台审核列表里给出,账号已移除时省略 */
  submitter_name?: string
  preview_report: SimValidationReport
  reviewer_id?: SnowflakeID
  result: SimReviewResult
  comment?: string
  created_at: string
  updated_at?: string
  package?: {
    code: string
    version: string
    name: string
    category: string
    compute: SimCompute
    status: SimPackageStatus
  }
}

export interface SimActionLog {
  seq: number
  at_tick: number
  event_type: string
  payload: Record<string, unknown>
  created_at?: string
}

export interface SimReplay {
  read_only: boolean
  package_code: string
  version: string
  seed: number
  init_params: Record<string, unknown>
  actions: SimActionLog[]
}

export interface SimShareCreate {
  expire_at?: string
}

export interface SimShareResult {
  code: string
  expire_at: string
  status: 'active'
}
