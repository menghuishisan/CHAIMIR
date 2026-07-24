// ===== M4 Sim 模块 =====

import type { SimCompute, SimPackageStatus, SimReviewResult, SimShareStatus } from '../constants/sim'
import type { SnowflakeID } from './common'

export interface SimPackageMeta {
  id: SnowflakeID
  code: string
  version: string
  name: string
  category: string
  compute: SimCompute
  scale_limit?: Record<string, unknown>
  bundle_hash?: string
  backend_adapter?: string
  status: SimPackageStatus
  created_at: string
  updated_at: string
}

export interface SimPackageSubmit {
  bundle: File
  code: string
  version: string
  name: string
  category: string
  compute: SimCompute
  scale_limit?: Record<string, unknown>
  backend_adapter?: string
  backend_config?: Record<string, unknown>
}

export interface SimBackendAdapterDescriptor {
  code: string
  name: string
  protocol: string
  description: string
}

export interface SimBackendCapabilities {
  backend_compute: boolean
  adapters: SimBackendAdapterDescriptor[]
}

export interface SimBundleDownloadGrant {
  token?: string
  bundle_hash: string
  expires_at: string
  module_url?: string
  builtin_code?: string
}

export interface SimPackageSubmissionResult extends SimPackageMeta {
  review: SimPackageReview
}

export interface SimReviewDecision {
  package: SimPackageMeta
  review: SimPackageReview
}

export interface SimValidationStatus {
  status?: string
  message?: string
}

export interface SimStaticScanReport {
  status?: string
  findings?: string[]
}

export interface SimValidationReport {
  bundle_hash?: string
  metadata_validation?: SimValidationStatus
  static_scan?: SimStaticScanReport
  determinism_check?: SimValidationStatus
  worker_preview?: SimValidationStatus
  details?: Record<string, string>
}

export interface SimValidationReportRequest {
  determinism_check: SimValidationStatus
  worker_preview: SimValidationStatus
  details: Record<string, string>
}

export interface SimPackageReview {
  id: SnowflakeID
  package_id: SnowflakeID
  submitter_id: SnowflakeID
  submitter_name: string
  preview_report: SimValidationReport
  reviewer_id?: SnowflakeID
  reviewer_name?: string
  result: SimReviewResult
  comment?: string
  created_at: string
  updated_at?: string
  package: {
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
  status: SimShareStatus
}
