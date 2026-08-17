// ===== M4 Sim 模块 =====

import type {
  SimCompute,
  SimPackageStatus,
  SimReviewResult,
  SimStreamCommand,
  SimValidationStatusValue,
} from '../constants/sim'
import type { SnowflakeID } from './common'

/** 仿真包协议固定的三项规模上限,与 M4 manifest meta.scale_limit 一致。 */
export interface SimScaleLimit {
  nodes: number
  max_tick: number
  max_events: number
}

export interface SimPackageMeta {
  id: SnowflakeID
  code: string
  version: string
  name: string
  category: string
  compute: SimCompute
  scale_limit?: SimScaleLimit
  bundle_hash?: string
  status: SimPackageStatus
  created_at: string
  updated_at: string
}

/**
 * 仿真包提交表单。刻意不含 compute 与运行能力:执行位置由后端按作者类型派生
 * (教师与第三方提交一律在隔离容器内运行),客户端声明它只会产生平台无法运行的无效状态。
 */
export interface SimPackageSubmit {
  bundle: File
  code: string
  version: string
  name: string
  category: string
  scale_limit?: SimScaleLimit
}

export interface SimPackageSubmissionResult extends SimPackageMeta {
  review: SimPackageReview
}

/**
 * 仿真包审核前预览响应:后端 PackagePreview 返回包摘要与最新审核记录两段,
 * 不把审核记录并进包对象(与提交响应的形状不同,故单独声明)。
 */
export interface SimPackagePreview {
  package: SimPackageMeta
  review: SimPackageReview
}

export interface SimReviewDecision {
  package: SimPackageMeta
  review: SimPackageReview
}

export interface SimValidationStatus {
  status?: SimValidationStatusValue
  message?: string
}

/**
 * 隔离容器算出、经后端协议校验后下发的一帧教学快照。
 *
 * 字段名保持后端下划线口径:它同时出现在审核报告的样例帧与隔离执行 WebSocket 帧里,
 * 两处是同一个服务端结构,故只声明一次。state/view 由平台自己的教学帧渲染器消费。
 */
export interface SimTeachingSnapshot {
  tick: number
  state: Record<string, unknown>
  view: Record<string, unknown>
  current_step?: Record<string, unknown>
  interaction_availability?: Record<string, boolean>
  checkpoint_results?: Record<string, unknown>
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

export interface SimStaticScanReport {
  status?: SimValidationStatusValue
  findings?: string[]
}

/**
 * 审核报告。preview_frames 是隔离容器渲出的样例教学帧:自动校验只能回答"能不能跑、
 * 是否确定性",回答不了"这个算法实现对不对",故审核页必须把帧摊给平台管理员看。
 */
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
