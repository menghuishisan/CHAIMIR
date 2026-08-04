// teaching labels 文件维护 M6 教学模块枚举的用户向文案与语义色。
// 语义色与文案同源:同一个后端枚举的界面表达只在这里登记一次,
// 避免每个页面各自决定"进行中该用什么颜色"而出现同一状态多种视觉。

import type { BadgeTone, StatusTone } from '@chaimir/ui'
import {
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
} from '@chaimir/api-client'

/** 课程状态文案:学生只会见到已发布之后的状态,草稿态仍登记以保证枚举全覆盖。 */
const COURSE_STATUS_LABELS: Record<CourseStatus, string> = {
  [CourseStatus.DRAFT]: '未发布',
  [CourseStatus.PUBLISHED]: '待开课',
  [CourseStatus.RUNNING]: '进行中',
  [CourseStatus.ENDED]: '已结课',
  [CourseStatus.ARCHIVED]: '已归档',
}

const COURSE_STATUS_TONES: Record<CourseStatus, StatusTone> = {
  [CourseStatus.DRAFT]: 'neutral',
  [CourseStatus.PUBLISHED]: 'info',
  [CourseStatus.RUNNING]: 'primary',
  [CourseStatus.ENDED]: 'success',
  [CourseStatus.ARCHIVED]: 'neutral',
}

/** courseStatusLabel 返回课程状态文案。 */
export function courseStatusLabel(status: CourseStatus): string {
  return COURSE_STATUS_LABELS[status]
}

/** courseStatusTone 返回课程状态语义色。 */
export function courseStatusTone(status: CourseStatus): StatusTone {
  return COURSE_STATUS_TONES[status]
}

const COURSE_TYPE_LABELS: Record<CourseType, string> = {
  [CourseType.THEORY]: '理论课',
  [CourseType.LAB]: '实验课',
  [CourseType.MIXED]: '理论与实验',
  [CourseType.PROJECT]: '项目实践',
}

/** courseTypeLabel 返回课程类型文案。 */
export function courseTypeLabel(type: CourseType): string {
  return COURSE_TYPE_LABELS[type]
}

const DIFFICULTY_LABELS: Record<TeachingDifficulty, string> = {
  [TeachingDifficulty.INTRO]: '入门',
  [TeachingDifficulty.ADVANCED]: '进阶',
  [TeachingDifficulty.EXPERT]: '高阶',
  [TeachingDifficulty.RESEARCH]: '研究型',
}

/** teachingDifficultyLabel 返回难度文案。 */
export function teachingDifficultyLabel(difficulty: TeachingDifficulty): string {
  return DIFFICULTY_LABELS[difficulty]
}

const LESSON_CONTENT_TYPE_LABELS: Record<LessonContentType, string> = {
  [LessonContentType.VIDEO]: '视频',
  [LessonContentType.MARKDOWN]: '图文',
  [LessonContentType.ATTACHMENT]: '资料',
  [LessonContentType.EXPERIMENT]: '实验',
  [LessonContentType.SIMULATION]: '仿真',
}

/** lessonContentTypeLabel 返回课时内容形态文案。 */
export function lessonContentTypeLabel(type: LessonContentType): string {
  return LESSON_CONTENT_TYPE_LABELS[type]
}

const PROGRESS_STATUS_LABELS: Record<ProgressStatus, string> = {
  [ProgressStatus.NOT_STARTED]: '未开始',
  [ProgressStatus.IN_PROGRESS]: '学习中',
  [ProgressStatus.DONE]: '已完成',
}

const PROGRESS_STATUS_TONES: Record<ProgressStatus, StatusTone> = {
  [ProgressStatus.NOT_STARTED]: 'neutral',
  [ProgressStatus.IN_PROGRESS]: 'info',
  [ProgressStatus.DONE]: 'success',
}

/** progressStatusLabel 返回课时学习状态文案。 */
export function progressStatusLabel(status: ProgressStatus): string {
  return PROGRESS_STATUS_LABELS[status]
}

/** progressStatusTone 返回课时学习状态语义色。 */
export function progressStatusTone(status: ProgressStatus): StatusTone {
  return PROGRESS_STATUS_TONES[status]
}

const ASSIGNMENT_STATUS_LABELS: Record<AssignmentStatus, string> = {
  [AssignmentStatus.DRAFT]: '未发布',
  [AssignmentStatus.PUBLISHED]: '已发布',
}

/** assignmentStatusLabel 返回作业发布状态文案。 */
export function assignmentStatusLabel(status: AssignmentStatus): string {
  return ASSIGNMENT_STATUS_LABELS[status]
}

const LATE_POLICY_LABELS: Record<LatePolicy, string> = {
  [LatePolicy.REJECT]: '截止后不接收',
  [LatePolicy.PENALIZE]: '迟交按规则扣分',
  [LatePolicy.NO_PENALTY]: '迟交不扣分',
}

/** latePolicyLabel 返回迟交策略文案。 */
export function latePolicyLabel(policy: LatePolicy): string {
  return LATE_POLICY_LABELS[policy]
}

const SUBMISSION_STATUS_LABELS: Record<SubmissionStatus, string> = {
  [SubmissionStatus.SUBMITTED]: '已提交',
  [SubmissionStatus.PENDING]: '批改中',
  [SubmissionStatus.GRADED]: '已出分',
}

const SUBMISSION_STATUS_TONES: Record<SubmissionStatus, StatusTone> = {
  [SubmissionStatus.SUBMITTED]: 'info',
  [SubmissionStatus.PENDING]: 'warning',
  [SubmissionStatus.GRADED]: 'success',
}

/** submissionStatusLabel 返回提交状态文案。 */
export function submissionStatusLabel(status: SubmissionStatus): string {
  return SUBMISSION_STATUS_LABELS[status]
}

/** submissionStatusTone 返回提交状态语义色。 */
export function submissionStatusTone(status: SubmissionStatus): StatusTone {
  return SUBMISSION_STATUS_TONES[status]
}

/** ASSIGNMENT_DUE_TONE 是作业截止时间临近程度对应的徽标色。 */
export const ASSIGNMENT_DUE_TONE: Record<'overdue' | 'urgent' | 'normal', BadgeTone> = {
  overdue: 'danger',
  urgent: 'warning',
  normal: 'neutral',
}

/**
 * 课时材料形态:视频与附件都把对象引用存在 content_ref 里,
 * 由 content_type 区分投放方式(视频流式续播 / 附件取件)。
 */
const MATERIAL_CONTENT_TYPES: ReadonlySet<LessonContentType> = new Set([
  LessonContentType.VIDEO,
  LessonContentType.ATTACHMENT,
])

/** isLessonMaterialType 判断课时是否为需要文件投放的形态。 */
export function isLessonMaterialType(type: LessonContentType): boolean {
  return MATERIAL_CONTENT_TYPES.has(type)
}

/** formatFileSize 把字节数换成用户向文件大小文案。 */
export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '大小未知'
  if (bytes < 1024) return `${bytes} 字节`
  const kb = bytes / 1024
  if (kb < 1024) return `${kb.toFixed(1)} KB`
  const mb = kb / 1024
  if (mb < 1024) return `${mb.toFixed(1)} MB`
  return `${(mb / 1024).toFixed(2)} GB`
}

/**
 * 题目难度文案由 M5 题库负责(`utils/labels/content.ts` 的 contentDifficultyLabel)。
 * 课程难度与题目难度是两套枚举:课程为 TeachingDifficulty(入门/进阶/高阶/研究型),
 * 题目为 ContentDifficulty(入门/基础/进阶/挑战),不可混用。
 */

const GRADING_MODE_LABELS: Record<GradingMode, string> = {
  [GradingMode.AUTO]: '自动判分',
  [GradingMode.MANUAL]: '教师批改',
}

/** gradingModeLabel 返回判分方式文案。 */
export function gradingModeLabel(mode: GradingMode): string {
  return GRADING_MODE_LABELS[mode]
}

const GRADE_SOURCE_LABELS: Record<GradeSource, string> = {
  [GradeSource.ASSIGNMENT]: '作业',
  [GradeSource.EXPERIMENT]: '实验',
  [GradeSource.EXAM]: '考试',
}

/** gradeSourceLabel 返回成绩来源文案。 */
export function gradeSourceLabel(source: GradeSource): string {
  return GRADE_SOURCE_LABELS[source]
}

/** GRADE_SOURCES 供权重表单按登记顺序渲染。 */
export const GRADE_SOURCES = [
  GradeSource.ASSIGNMENT,
  GradeSource.EXPERIMENT,
  GradeSource.EXAM,
] as const

const COURSE_VISIBILITY_LABELS: Record<CourseVisibility, string> = {
  [CourseVisibility.PRIVATE]: '仅本校',
  [CourseVisibility.SHARED]: '已共享到课程库',
}

/** courseVisibilityLabel 返回课程可见范围文案。 */
export function courseVisibilityLabel(visibility: CourseVisibility): string {
  return COURSE_VISIBILITY_LABELS[visibility]
}

const JOIN_MODE_LABELS: Record<JoinMode, string> = {
  [JoinMode.INVITE]: '邀请码加入',
  [JoinMode.TEACHER]: '教师添加',
}

/** joinModeLabel 返回成员加入方式文案。 */
export function joinModeLabel(mode: JoinMode): string {
  return JOIN_MODE_LABELS[mode]
}

/** COURSE_TYPES 供课程表单按登记顺序渲染类型选项。 */
export const COURSE_TYPES = [
  CourseType.THEORY,
  CourseType.LAB,
  CourseType.MIXED,
  CourseType.PROJECT,
] as const

/** TEACHING_DIFFICULTIES 供课程表单渲染难度选项。 */
export const TEACHING_DIFFICULTIES = [
  TeachingDifficulty.INTRO,
  TeachingDifficulty.ADVANCED,
  TeachingDifficulty.EXPERT,
  TeachingDifficulty.RESEARCH,
] as const

/** LATE_POLICIES 供作业表单渲染迟交策略选项。 */
export const LATE_POLICIES = [
  LatePolicy.REJECT,
  LatePolicy.PENALIZE,
  LatePolicy.NO_PENALTY,
] as const

/** LESSON_CONTENT_TYPES 供课时表单渲染内容形态选项。 */
export const LESSON_CONTENT_TYPES = [
  LessonContentType.VIDEO,
  LessonContentType.MARKDOWN,
  LessonContentType.ATTACHMENT,
  LessonContentType.EXPERIMENT,
  LessonContentType.SIMULATION,
] as const
