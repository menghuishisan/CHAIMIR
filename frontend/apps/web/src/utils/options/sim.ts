// sim 定义仿真计算与审核页面使用的选择项。

import { SIM_COMPUTE, SIM_REVIEW_RESULT } from '@chaimir/api-client'
import { simComputeLabel, simReviewResultLabel } from '../labels'
import { option, withAllOption } from './shared'

export const simComputeOptions = [option(SIM_COMPUTE.FRONTEND, simComputeLabel(SIM_COMPUTE.FRONTEND)), option(SIM_COMPUTE.BACKEND, simComputeLabel(SIM_COMPUTE.BACKEND))]
export const simReviewResultOptions = withAllOption('全部结果', [option(SIM_REVIEW_RESULT.PENDING, simReviewResultLabel(SIM_REVIEW_RESULT.PENDING)), option(SIM_REVIEW_RESULT.APPROVED, simReviewResultLabel(SIM_REVIEW_RESULT.APPROVED)), option(SIM_REVIEW_RESULT.REJECTED, simReviewResultLabel(SIM_REVIEW_RESULT.REJECTED))])
