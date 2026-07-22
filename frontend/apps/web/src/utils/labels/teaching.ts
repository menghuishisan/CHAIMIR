// teaching labels 文件维护课程、作业和课时领域文案。

import { AssignmentStatus, CourseStatus, CourseType, GradingMode, JoinMode, LatePolicy, LessonContentType, TeachingDifficulty } from '@chaimir/api-client'
import { labelFromMap } from './map'

/** courseTypeLabel 返回课程类型文案。 */
export function courseTypeLabel(type: CourseType): string {
  return labelFromMap(type, {
    [CourseType.THEORY]: '理论课', [CourseType.LAB]: '实验课', [CourseType.MIXED]: '混合课',
    [CourseType.PROJECT]: '项目课',
  }, '未识别的课程类型')
}

/** teachingDifficultyLabel 返回教学课程难度文案。 */
export function teachingDifficultyLabel(difficulty: TeachingDifficulty): string {
  return labelFromMap(difficulty, {
    [TeachingDifficulty.INTRO]: '入门', [TeachingDifficulty.ADVANCED]: '进阶',
    [TeachingDifficulty.EXPERT]: '高阶', [TeachingDifficulty.RESEARCH]: '研究型',
  }, '未识别的课程难度')
}

/** courseStatusLabel 返回课程状态文案。 */
export function courseStatusLabel(status: CourseStatus): string {
  return labelFromMap(status, {
    [CourseStatus.DRAFT]: '草稿', [CourseStatus.PUBLISHED]: '已发布', [CourseStatus.RUNNING]: '进行中',
    [CourseStatus.ENDED]: '已结课', [CourseStatus.ARCHIVED]: '已归档',
  }, '未知')
}

/** assignmentStatusLabel 返回作业发布状态文案。 */
export function assignmentStatusLabel(status: AssignmentStatus): string {
  return labelFromMap(status, { [AssignmentStatus.DRAFT]: '草稿', [AssignmentStatus.PUBLISHED]: '已发布' }, '未知')
}

/** joinModeLabel 返回课程成员加入方式文案。 */
export function joinModeLabel(mode: JoinMode): string {
  return labelFromMap(mode, { [JoinMode.INVITE]: '邀请码加入', [JoinMode.TEACHER]: '教师添加' }, '未知')
}

/** lessonContentTypeLabel 返回课程小节内容类型文案。 */
export function lessonContentTypeLabel(type: LessonContentType): string {
  return labelFromMap(type, {
    [LessonContentType.VIDEO]: '视频', [LessonContentType.MARKDOWN]: '图文',
    [LessonContentType.ATTACHMENT]: '附件', [LessonContentType.EXPERIMENT]: '实验',
    [LessonContentType.SIMULATION]: '仿真',
  }, '未识别的内容类型')
}

/** latePolicyLabel 返回作业逾期策略文案。 */
export function latePolicyLabel(policy: LatePolicy): string {
  return labelFromMap(policy, {
    [LatePolicy.REJECT]: '不允许逾期提交', [LatePolicy.PENALIZE]: '逾期扣分',
    [LatePolicy.NO_PENALTY]: '允许补交',
  }, '未识别的逾期策略')
}

/** gradingModeLabel 返回作业评分方式文案。 */
export function gradingModeLabel(mode: GradingMode): string {
  return labelFromMap(mode, { [GradingMode.AUTO]: '自动评分', [GradingMode.MANUAL]: '人工评分' }, '未识别的评分方式')
}
