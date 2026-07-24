// teaching 定义教学领域页面使用的选择项。

import { CourseStatus, CourseType, ExperimentCollabMode, GradingMode, LatePolicy, LessonContentType, TeachingDifficulty } from '@chaimir/api-client'
import { courseStatusLabel, courseTypeLabel, experimentCollabModeLabel, gradingModeLabel, latePolicyLabel, lessonContentTypeLabel, teachingDifficultyLabel } from '../labels'
import { option, withAllOption } from './shared'

export const courseStatusOptions = [
  option(CourseStatus.DRAFT, courseStatusLabel(CourseStatus.DRAFT)),
  option(CourseStatus.PUBLISHED, courseStatusLabel(CourseStatus.PUBLISHED)),
  option(CourseStatus.RUNNING, courseStatusLabel(CourseStatus.RUNNING)),
  option(CourseStatus.ENDED, courseStatusLabel(CourseStatus.ENDED)),
  option(CourseStatus.ARCHIVED, courseStatusLabel(CourseStatus.ARCHIVED)),
]
export const studentCourseStatusFilterOptions = withAllOption('全部课程', [
  option(CourseStatus.RUNNING, courseStatusLabel(CourseStatus.RUNNING)),
  option(CourseStatus.ENDED, courseStatusLabel(CourseStatus.ENDED)),
])
export const courseTypeOptions = Object.values(CourseType).filter((value): value is CourseType => typeof value === 'number').map((value) => option(value, courseTypeLabel(value)))
export const teachingDifficultyOptions = Object.values(TeachingDifficulty).filter((value): value is TeachingDifficulty => typeof value === 'number').map((value) => option(value, teachingDifficultyLabel(value)))
export const lessonContentTypeOptions = Object.values(LessonContentType).filter((value): value is LessonContentType => typeof value === 'number').map((value) => option(value, lessonContentTypeLabel(value)))
export const latePolicyOptions = Object.values(LatePolicy).filter((value): value is LatePolicy => typeof value === 'number').map((value) => option(value, latePolicyLabel(value)))
export const gradingModeOptions = Object.values(GradingMode).filter((value): value is GradingMode => typeof value === 'number').map((value) => option(value, gradingModeLabel(value)))
export const experimentCollabModeOptions = Object.values(ExperimentCollabMode).filter((value): value is ExperimentCollabMode => typeof value === 'number').map((value) => option(value, experimentCollabModeLabel(value)))
