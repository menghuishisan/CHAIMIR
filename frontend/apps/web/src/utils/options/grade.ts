// grade 定义成绩审核与申诉页面使用的筛选项。

import { GradeAppealStatus, GradeReviewStatus } from '@chaimir/api-client'
import { gradeAppealStatusLabel, gradeReviewStatusLabel } from '../labels'
import { option, withAllOption } from './shared'

export const gradeReviewStatusFilterOptions = withAllOption('全部状态', [option(GradeReviewStatus.PENDING, gradeReviewStatusLabel(GradeReviewStatus.PENDING)), option(GradeReviewStatus.APPROVED, gradeReviewStatusLabel(GradeReviewStatus.APPROVED)), option(GradeReviewStatus.REJECTED, gradeReviewStatusLabel(GradeReviewStatus.REJECTED))])
export const gradeAppealStatusFilterOptions = withAllOption('全部状态', [option(GradeAppealStatus.PENDING, gradeAppealStatusLabel(GradeAppealStatus.PENDING)), option(GradeAppealStatus.ACCEPTED, gradeAppealStatusLabel(GradeAppealStatus.ACCEPTED)), option(GradeAppealStatus.COMPLETED, gradeAppealStatusLabel(GradeAppealStatus.COMPLETED)), option(GradeAppealStatus.REJECTED, gradeAppealStatusLabel(GradeAppealStatus.REJECTED))])
