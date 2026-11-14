// runtimeOnboarding.ts 链运行时的接入进度(设计 §7.6:卡片上的接入进度链与「下一步」,详情页的接入流程条)。
// 四步沿用现有定义:填写环境配置 → 登记镜像(预拉取按组合进行)→ 自检通过 → 开放给学校。
// 后端口径:自检只要求有一个可用镜像版本(selectRuntimeSelftestImage),停用的运行时不能自检;
// 只有「开放 + 自检通过」才算教师可选。列表接口带可用镜像版本数;详情页在镜像列表读到之前不知道镜像数,
// 这时按自检结果推断镜像这一步。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 运行时状态取值,与 @chaimir/api-client 的 RuntimeStatus 一致 */
const STATUS_AVAILABLE = 1
const STATUS_DISABLED = 3
/** 自检状态取值,与 RuntimeSelftestStatus 一致 */
const SELFTEST_PENDING = 1
const SELFTEST_PASSED = 2
const SELFTEST_FAILED = 3

/** 接入四步的名称(进度链与流程条共用) */
export const ONBOARDING_STEPS = ['填写环境配置', '登记镜像版本', '自检通过', '开放给学校'] as const

/** 自检这一步在四步里的位置(0 起) */
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

/** runtimeOnboarding 由运行时状态、自检结果与镜像数算出接入进度 */
export function runtimeOnboarding({
  status,
  selftest,
  availableImages,
  disabledReason,
}: RuntimeOnboardingInput): RuntimeOnboarding {
  if (status === STATUS_AVAILABLE && selftest === SELFTEST_PASSED)
    return { done: ONBOARDING_STEPS.length }

  const imagesKnown = availableImages !== undefined
  // 自检跑过就说明当时有可用镜像;镜像数已知时以实际为准
  const hasImage = imagesKnown ? availableImages > 0 : selftest !== SELFTEST_PENDING
  const disabled = status === STATUS_DISABLED

  if (selftest === SELFTEST_PASSED && hasImage) {
    const next = disabled
      ? '已停用,不会分配给学校'
      : disabledReason || '自检已通过,还没有开放给学校'
    return { done: 3, next }
  }
  if (!hasImage) {
    if (disabled) return { done: 1, next: '已停用,不会分配给学校' }
    return { done: 1, next: imagesKnown ? '下一步:登记镜像版本' : '下一步:登记镜像版本并运行自检' }
  }
  if (selftest === SELFTEST_FAILED) {
    return {
      done: 2,
      failedIndex: SELFTEST_STEP,
      next: disabled ? '已停用,不会分配给学校' : '自检没有通过,查看原因后重新自检',
    }
  }
  return { done: 2, next: disabled ? '已停用,不会分配给学校' : '下一步:运行自检' }
}
