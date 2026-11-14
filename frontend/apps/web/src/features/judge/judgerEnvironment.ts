// judgerEnvironment.ts 判题器列表上的「判题环境」与「上次自测」(设计 §7.6 判题器;规范:执行环境事实)。
// 判题环境:需要链环境的判题器,只有带非空组合摘要的冻结快照才算准备好(与 getFrozenJudgerSnapshot 同一判定);
// 未冻结写「尚未准备完成」;已冻结但自检失败时,冻结的事实照样成立,失败单独标出。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 自检状态取值,与 @chaimir/api-client 的 JudgerSelftestStatus 一致 */
const SELFTEST_PASSED = 2
const SELFTEST_FAILED = 3
/** 人工评分类型,与 JudgerType.MANUAL 一致 */
const TYPE_MANUAL = 6

/** 一个状态的语义色与文字(与 StatusIndicator 的 tone 同名) */
export interface JudgerStateText {
  tone: 'neutral' | 'success' | 'warning' | 'danger'
  label: string
}

/** judgerEnvironment 判题环境的状态;不需要链环境时返回 undefined */
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
  if (selftest === SELFTEST_FAILED) return { tone: 'danger', label: '判题环境已冻结,自测失败' }
  if (selftest === SELFTEST_PASSED) return { tone: 'success', label: '判题环境已冻结' }
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
  if (type === TYPE_MANUAL) return { tone: 'neutral', label: '人工评分不需要自测' }
  if (selftest === SELFTEST_PASSED) return { tone: 'success', label: '自测通过' }
  if (selftest === SELFTEST_FAILED) return { tone: 'danger', label: '自测失败' }
  return { tone: 'neutral', label: '还没有自测' }
}
