// ===== M11 Grade 模块 =====

import type { GradeAppealStatus, GradeReviewStatus, GradeWarningType, TranscriptScope } from '../constants/grade'

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
  id: string
  tenant_id: string
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
  id: string
  tenant_id: string
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
  id: string
  tenant_id: string
  course_id: string
  semester_id?: string
  submitter_id: string
  reviewer_id?: string
  status: GradeReviewStatus
  is_locked: boolean
  comment?: string
  submitted_at: string
  reviewed_at?: string
}

export interface GradeReviewRequest {
  course_id: string
  semester_id?: string
  comment?: string
}

export interface ReviewDecision {
  semester_id?: string
  comment?: string
}

export interface CourseGrade {
  course_id: string
  student_id: string
  final_total: number
  credits: number
}

export interface GradeSummary {
  student_id: string
  semester_id?: string
  total_credits: number
  gpa: number
  cumulative_gpa: number
  course_grades: CourseGrade[]
  computed_at: string
}

export interface GradeAppeal {
  id: string
  tenant_id: string
  student_id: string
  course_id: string
  reason: string
  status: GradeAppealStatus
  handler_id?: string
  result_comment?: string
  created_at: string
  handled_at?: string
}

export interface GradeAppealRequest {
  course_id: string
  reason: string
}

export interface GradeWarning {
  id: string
  tenant_id: string
  student_id: string
  semester_id: string
  type: GradeWarningType
  detail: Record<string, unknown>
  status: number
  created_at: string
}

export interface WarningScanResult {
  scanned: number
  created: number
}

export interface GradeTranscript {
  id: string
  tenant_id: string
  student_id: string
  scope: TranscriptScope
  semester_id?: string
  generated_at: string
}

export interface TranscriptRequest {
  student_id?: string
  scope: TranscriptScope
  semester_id?: string
}

export interface TranscriptDownloadGrant {
  token: string
  transcript: GradeTranscript
  expires_at: string
}
