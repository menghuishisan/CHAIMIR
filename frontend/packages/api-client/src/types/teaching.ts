// ===== M6 Teaching 模块 =====

import type {
  AssignmentStatus,
  CourseStatus,
  CourseType,
  CourseVisibility,
  GradeSource,
  GradingMode,
  JoinMode,
  LatePolicy,
  LessonContentType,
  ProgressStatus,
  SubmissionStatus,
  TeachingDifficulty,
} from '../constants/teaching'

export interface Course {
  id: number
  tenant_id: number
  teacher_id: number
  name: string
  description: string
  type: CourseType
  difficulty: TeachingDifficulty
  cover_url?: string
  semester: string
  credits: number
  schedule: Record<string, unknown>
  start_at: string
  end_at: string
  invite_code?: string
  status: CourseStatus
  visibility: CourseVisibility
  created_at: string
  updated_at: string
}

export interface CourseRequest {
  name: string
  description: string
  type: CourseType
  difficulty: TeachingDifficulty
  cover_url?: string
  semester: string
  credits: number
  schedule: Record<string, unknown>
  start_at: string
  end_at: string
}

export interface Chapter {
  id: number
  course_id: number
  title: string
  sort: number
  created_at: string
  updated_at: string
}

export interface ChapterRequest {
  title: string
  sort: number
}

export interface Lesson {
  id: number
  chapter_id: number
  title: string
  content_type: LessonContentType
  content_ref: Record<string, unknown>
  sort: number
  created_at: string
  updated_at: string
}

export interface LessonRequest {
  title: string
  content_type: LessonContentType
  content_ref: Record<string, unknown>
  sort: number
}

export interface CourseOutline {
  course: Course
  chapters: Chapter[]
  lessons: Lesson[]
  progress: Progress[]
}

export interface Progress {
  lesson_id: number
  student_id: number
  status: ProgressStatus
  video_pos: number
  duration_sec: number
  updated_at: string
}

export interface ProgressRequest {
  status: ProgressStatus
  video_pos: number
  duration_sec: number
}

export interface JoinCourseRequest {
  invite_code: string
}

export interface Assignment {
  id: number
  course_id: number
  title: string
  chapter_id?: number
  due_at: string
  max_attempts: number
  late_policy: LatePolicy
  late_penalty: Record<string, unknown>
  status: AssignmentStatus
  created_at: string
  updated_at: string
}

export interface AssignmentRequest {
  title: string
  chapter_id: number
  due_at: string
  max_attempts: number
  late_policy: LatePolicy
  late_penalty: Record<string, unknown>
  items: AssignmentItemInput[]
}

export interface AssignmentItemInput {
  item_code: string
  item_version: string
  score: number
  seq: number
  grading_mode: GradingMode
  judger_code: string
}

export interface AssignmentItem {
  id: number
  item_code: string
  item_version: string
  score: number
  seq: number
  grading_mode: GradingMode
  judger_code?: string
  title?: string
  type?: CourseType
  difficulty?: TeachingDifficulty
  body?: Record<string, unknown>
}

export interface AssignmentDetail {
  assignment: Assignment
  items: AssignmentItem[]
}

export interface Draft {
  assignment_id: number
  student_id: number
  content: Record<string, unknown>
  updated_at: string
  exists: boolean
}

export interface Submission {
  id: number
  assignment_id: number
  student_id: number
  attempt_no: number
  content: Record<string, unknown>
  judge_task_ref?: string
  auto_score?: number
  manual_score?: number
  final_score?: number
  comment?: string
  is_late: boolean
  status: SubmissionStatus
  submitted_at: string
}

export interface SubmitRequest {
  content_ref: Record<string, unknown>
}

export interface CourseMember {
  id: number
  course_id: number
  student_id: number
  join_mode: JoinMode
  joined_at: string
}

export interface BatchMembersRequest {
  student_ids: number[]
}

export interface TeachingPostRequest {
  parent_id?: number
  content: string
}

export interface TeachingPost {
  id: number
  course_id: number
  parent_id?: number
  author_id: number
  content: string
  is_pinned: boolean
  like_count: number
  created_at: string
}

export interface TeachingAnnouncementRequest {
  title: string
  content: string
  is_pinned: boolean
}

export interface TeachingAnnouncement {
  id: number
  course_id: number
  title: string
  content: string
  is_pinned: boolean
  created_at: string
}

export interface TeachingReviewRequest {
  rating: number
  comment: string
}

export interface TeachingReview {
  id: number
  course_id: number
  student_id: number
  rating: number
  comment: string
  created_at: string
}

export interface ProgressStats {
  course_id: number
  member_count: number
  lesson_count: number
  completed_count: number
  learning_duration_sec: number
}

export interface GradeWeightRequest {
  items: GradeWeightInput[]
}

export interface GradeWeightInput {
  source_type: GradeSource
  source_ref: string
  weight: number
}

export interface GradeWeight {
  id: number
  source_type: GradeSource
  source_ref: string
  weight: number
}

export interface OverrideGradeRequest {
  total: number
}

export interface TeachingCourseGrade {
  course_id: number
  student_id: number
  auto_total: number
  override_total?: number
  final_total: number
  is_overridden: boolean
  is_locked: boolean
  credits: number
  updated_at: string
}
