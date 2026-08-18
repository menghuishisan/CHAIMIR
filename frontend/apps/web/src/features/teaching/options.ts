// teaching options 文件维护 M6 表单使用的封闭枚举顺序。

import {
  CourseType,
  GradeSource,
  LatePolicy,
  LessonContentType,
  TeachingDifficulty,
} from '@chaimir/api-client'

/** GRADE_SOURCES 按成绩权重表单的固定顺序排列。 */
export const GRADE_SOURCES = [GradeSource.ASSIGNMENT, GradeSource.EXPERIMENT, GradeSource.EXAM] as const

/** COURSE_TYPES 按课程表单的固定顺序排列。 */
export const COURSE_TYPES = [CourseType.THEORY, CourseType.LAB, CourseType.MIXED, CourseType.PROJECT] as const

/** TEACHING_DIFFICULTIES 按课程表单的固定顺序排列。 */
export const TEACHING_DIFFICULTIES = [
  TeachingDifficulty.INTRO,
  TeachingDifficulty.ADVANCED,
  TeachingDifficulty.EXPERT,
  TeachingDifficulty.RESEARCH,
] as const

/** LATE_POLICIES 按作业表单的固定顺序排列。 */
export const LATE_POLICIES = [LatePolicy.REJECT, LatePolicy.PENALIZE, LatePolicy.NO_PENALTY] as const

/** LESSON_CONTENT_TYPES 按课时表单的固定顺序排列。 */
export const LESSON_CONTENT_TYPES = [
  LessonContentType.VIDEO,
  LessonContentType.MARKDOWN,
  LessonContentType.ATTACHMENT,
  LessonContentType.EXPERIMENT,
  LessonContentType.SIMULATION,
] as const
