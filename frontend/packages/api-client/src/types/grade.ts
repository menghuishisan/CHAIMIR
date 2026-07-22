// ===== M11 Grade 模块 =====

import type { GradeAppealStatus, GradeReviewStatus, GradeWarningStatus, GradeWarningType, TranscriptScope } from '../constants/grade'
import type { SnowflakeID } from './common'

export interface LevelRule {
  min: number
  grade: string
  gpa: number
}

export interface WarningRules {
  fail_count: number
  min_gpa: number
}

export interface LevelConfig {
  id: SnowflakeID
  tenant_id: SnowflakeID
  name: string
  mapping: LevelRule[]
  warning_rules: WarningRules
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface LevelConfigRequest {
  name: string
  mapping: LevelRule[]
  warning_rules: WarningRules
  is_default: boolean
}

export interface Semester {
  id: SnowflakeID
  tenant_id: SnowflakeID
  name: string
  start_date: string
  end_date: string
  is_current: boolean
}

export interface SemesterRequest {
  name: string
  start_date: string
  end_date: string
  is_current: boolean
}

export interface GradeReview {
  id: SnowflakeID
  tenant_id: SnowflakeID
  course_id: SnowflakeID
  semester_id?: SnowflakeID
  submitter_id: SnowflakeID
  reviewer_id?: SnowflakeID
  status: GradeReviewStatus
  is_locked: boolean
  comment?: string
  submitted_at: string
  reviewed_at?: string
}

export interface GradeReviewRequest {
  course_id: SnowflakeID
  semester_id?: SnowflakeID
  comment?: string
}

export interface ReviewDecision {
  semester_id?: SnowflakeID
  comment?: string
}

export interface CourseGrade {
  course_id: SnowflakeID
  student_id: SnowflakeID
  final_total: number
  credits: number
}

export interface GradeSummary {
  student_id: SnowflakeID
  semester_id?: SnowflakeID
  total_credits: number
  gpa: number
  cumulative_gpa: number
  course_grades: CourseGrade[]
  computed_at: string
}

export interface GradeAppeal {
  id: SnowflakeID
  tenant_id: SnowflakeID
  student_id: SnowflakeID
  course_id: SnowflakeID
  reason: string
  status: GradeAppealStatus
  handler_id?: SnowflakeID
  result_comment?: string
  created_at: string
  handled_at?: string
}

export interface GradeAppealRequest {
  course_id: SnowflakeID
  reason: string
}

export interface GradeWarning {
  id: SnowflakeID
  tenant_id: SnowflakeID
  student_id: SnowflakeID
  semester_id: SnowflakeID
  type: GradeWarningType
  detail: Record<string, unknown>
  status: GradeWarningStatus
  created_at: string
}

export interface WarningScanResult {
  scanned: number
  created: number
}

export interface GradeTranscript {
  id: SnowflakeID
  tenant_id: SnowflakeID
  student_id: SnowflakeID
  scope: TranscriptScope
  semester_id?: SnowflakeID
  generated_at: string
}

export interface TranscriptRequest {
  student_id?: SnowflakeID
  scope: TranscriptScope
  semester_id?: SnowflakeID
}

export interface TranscriptDownloadGrant {
  token: string
  transcript: GradeTranscript
  expires_at: string
}
