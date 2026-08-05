// ===== M6 Teaching 模块 =====

import type { SnowflakeID } from './common'
import type { ContentDifficulty, ContentType } from '../constants/content'
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
  id: SnowflakeID
  tenant_id: SnowflakeID
  teacher_id: SnowflakeID
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
  id: SnowflakeID
  course_id: SnowflakeID
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
  id: SnowflakeID
  chapter_id: SnowflakeID
  title: string
  content_type: LessonContentType
  content_ref: Record<string, unknown>
  sort: number
  created_at: string
  updated_at: string
}

/**
 * LessonMaterialAccess 是课时材料的投放授权。
 * mode 由课时 content_type 决定(视频回 stream、附件回 download),不接受客户端指定;
 * token 交给统一文件服务 GET /storage/download 消费。
 */
export interface LessonMaterialAccess {
  token: string
  mode: 'download' | 'stream'
  file_name: string
  size: number
  content_type: string
  expires_at: string
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
  lesson_id: SnowflakeID
  student_id: SnowflakeID
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
  id: SnowflakeID
  course_id: SnowflakeID
  title: string
  chapter_id?: SnowflakeID
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
  chapter_id: SnowflakeID
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
  id: SnowflakeID
  item_code: string
  item_version: string
  score: number
  seq: number
  grading_mode: GradingMode
  judger_code?: string
  /**
   * 以下四个字段是 M6 从 M5 题面视角展开的题目信息(见 service_assignment.go
   * GetAssignmentForStudent),因此 type / difficulty 是 M5 的内容枚举而非教学枚举。
   * body 已由 M5 剥离答案、判题配置与 flag。
   */
  title?: string
  type?: ContentType
  difficulty?: ContentDifficulty
  body?: Record<string, unknown>
}

export interface AssignmentDetail {
  assignment: Assignment
  items: AssignmentItem[]
}

export interface Draft {
  assignment_id: SnowflakeID
  student_id: SnowflakeID
  content: Record<string, unknown>
  updated_at: string
  exists: boolean
}

export interface Submission {
  id: SnowflakeID
  assignment_id: SnowflakeID
  student_id: SnowflakeID
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
  id: SnowflakeID
  course_id: SnowflakeID
  student_id: SnowflakeID
  student_name: string
  student_no?: string
  join_mode: JoinMode
  joined_at: string
}

/** 按班级批量添加课程成员:学生编号由服务端按班级解析，客户端只给班级。 */
export interface BatchMembersRequest {
  class_id: SnowflakeID
}

export interface TeachingPostRequest {
  parent_id?: SnowflakeID
  content: string
}

export interface TeachingPost {
  id: SnowflakeID
  course_id: SnowflakeID
  parent_id?: SnowflakeID
  author_id: SnowflakeID
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
  id: SnowflakeID
  course_id: SnowflakeID
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
  id: SnowflakeID
  course_id: SnowflakeID
  student_id: SnowflakeID
  rating: number
  comment: string
  created_at: string
}

export interface ProgressStats {
  course_id: SnowflakeID
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
  id: SnowflakeID
  source_type: GradeSource
  source_ref: string
  weight: number
}

export interface OverrideGradeRequest {
  total: number
}

export interface TeachingCourseGrade {
  course_id: SnowflakeID
  student_id: SnowflakeID
  auto_total: number
  override_total?: number
  final_total: number
  is_overridden: boolean
  is_locked: boolean
  credits: number
  updated_at: string
}
