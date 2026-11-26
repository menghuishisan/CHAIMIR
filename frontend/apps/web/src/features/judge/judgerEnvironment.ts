import { JudgerSelftestStatus, JudgerType } from '@chaimir/api-client'

/** 一个状态的语义色与文字(与 StatusIndicator 的 tone 同名) */
export interface JudgerStateText {
  tone: 'neutral' | 'success' | 'warning' | 'danger'
  label: string
}

/** 已冻结与自检通过分别展示；自检失败不撤销冻结事实。 */
export function judgerEnvironment({
  runtimeRequired,
  frozen,
  selftest,
}: {
  runtimeRequired: boolean
  frozen: boolean
  selftest: number
}): JudgerStateText | undefined {
  if (!runtimeRequired && !frozen) return undefined
  if (!frozen) return { tone: 'warning', label: '判题环境尚未准备完成' }
  if (selftest === JudgerSelftestStatus.FAILED) return { tone: 'danger', label: '判题环境已冻结,自测失败' }
  if (selftest === JudgerSelftestStatus.PASSED) return { tone: 'success', label: '判题环境已冻结' }
  return { tone: 'neutral', label: '判题环境已冻结,还没有自测' }
}

/** judgerSelftest 上次自测的结果;人工评分不需要自测 */
export function judgerSelftest({
  type,
  selftest,
}: {
  type: number
  selftest: number
}): JudgerStateText {
  if (type === JudgerType.MANUAL) return { tone: 'neutral', label: '人工评分不需要自测' }
  if (selftest === JudgerSelftestStatus.PASSED) return { tone: 'success', label: '自测通过' }
  if (selftest === JudgerSelftestStatus.FAILED) return { tone: 'danger', label: '自测失败' }
  return { tone: 'neutral', label: '还没有自测' }
}
