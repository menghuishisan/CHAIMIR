import { RuntimeStatus, RuntimeSelftestStatus } from '@chaimir/api-client'

/** 接入四步的名称(进度链与流程条共用) */
export const ONBOARDING_STEPS = ['填写环境配置', '登记镜像版本', '自检通过', '开放给学校'] as const

const SELFTEST_STEP = 2

export interface RuntimeOnboardingInput {
  status: number
  selftest: number
  /** 可用镜像版本数;还没读到时不传 */
  availableImages?: number
  /** 服务端写明的不可用原因 */
  disabledReason?: string
}

/** 接入进度:已完成几步、哪一步失败、下一步写什么(走完时没有 next) */
export interface RuntimeOnboarding {
  done: number
  failedIndex?: number
  next?: string
}

export function runtimeOnboarding({
  status,
  selftest,
  availableImages,
  disabledReason,
}: RuntimeOnboardingInput): RuntimeOnboarding {
  const imagesKnown = availableImages !== undefined
  const hasImage = imagesKnown && availableImages > 0
  const disabled = status === RuntimeStatus.DISABLED

  if (selftest === RuntimeSelftestStatus.PASSED && hasImage) {
    if (status === RuntimeStatus.AVAILABLE && !disabledReason)
      return { done: ONBOARDING_STEPS.length }
    const next = disabled
      ? '已停用,不会分配给学校'
      : disabledReason || '自检已通过,还没有开放给学校'
    return { done: 3, next }
  }
  if (!hasImage) {
    if (disabled) return { done: 1, next: '已停用,不会分配给学校' }
    return { done: 1, next: imagesKnown ? '下一步:登记镜像版本' : '先确认镜像版本是否可用' }
  }
  if (selftest === RuntimeSelftestStatus.FAILED) {
    return {
      done: 2,
      failedIndex: SELFTEST_STEP,
      next: disabled ? '已停用,不会分配给学校' : '自检没有通过,查看原因后重新自检',
    }
  }
  return { done: 2, next: disabled ? '已停用,不会分配给学校' : '下一步:运行自检' }
}
