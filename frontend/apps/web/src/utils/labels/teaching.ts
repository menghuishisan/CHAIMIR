// teaching labels 文件维护 M6 教学模块枚举的用户向文案。

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

/** courseStatusLabel 返回课程状态文案。 */
export function courseStatusLabel(status: CourseStatus): string {
  return COURSE_STATUS_LABELS[status]
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

/** progressStatusLabel 返回课时学习状态文案。 */
export function progressStatusLabel(status: ProgressStatus): string {
  return PROGRESS_STATUS_LABELS[status]
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

/** submissionStatusLabel 返回提交状态文案。 */
export function submissionStatusLabel(status: SubmissionStatus): string {
  return SUBMISSION_STATUS_LABELS[status]
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
