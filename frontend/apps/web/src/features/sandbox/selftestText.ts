// features/sandbox/selftestText 把服务端自检明细里的稳定原因码转成用户向文案。
//
// 后端 runtimeSelftestPublicDetail 只下发 result / stage / reason / trace_id 四个字段。
// 其中 result 与 stage 是固定取值,reason 既可能是目录同步写入的中文说明,
// 也可能是服务端写入的稳定原因码(如 runtime_execution_contract_changed)——
// 原因码不能直接抛到界面上(CLAUDE.md §8:不暴露开发术语),故这里按码表转换,
// 未登记的码给一句通用说明,已是自然语言的原样呈现。

/** 自检结果的固定取值,与后端写入的 result 一致。 */
const RESULT_TEXT: Record<string, string> = {
  passed: '通过',
  failed: '未通过',
  running: '正在自检',
  pending: '待自检',
  blocked: '被阻断,暂时不能自检',
}

/** 自检停在哪一步,与后端写入的 stage 一致。 */
const STAGE_TEXT: Record<string, string> = {
  selftest: '平台自检',
  start: '启动环境',
  init: '初始化环境',
  create_audit: '记录操作审计',
  recycle: '回收环境',
  disabled: '已停用',
  invalidated: '声明变更后已失效',
  debounced_file_save: '保存工作区文件',
}

/** 稳定原因码到用户向说明。 */
const REASON_TEXT: Record<string, string> = {
  runtime_execution_contract_changed: '运行时的执行声明改过了,需要重新自检',
  tool_prepull_contract_changed: '关联组件的镜像清单改过了,需要重新预拉取',
  native_fixture_missing: '镜像里缺少自检要用的样例数据',
  'requires-runtime-prepull': '还没有为已发布组合完成镜像预拉取',
}

/** 判断一段文本是否是内部原因码(全小写字母、数字、下划线或连字符)。 */
function looksLikeCode(value: string): boolean {
  return /^[a-z0-9_-]+$/.test(value)
}

/** selftestResultText 返回自检结果的用户向文案。 */
export function selftestResultText(result: string): string {
  const value = result.trim()
  return RESULT_TEXT[value] ?? (looksLikeCode(value) ? '状态未知' : value)
}

/** selftestStageText 返回自检停在哪一步的用户向文案。 */
export function selftestStageText(stage: string): string {
  const value = stage.trim()
  return STAGE_TEXT[value] ?? (looksLikeCode(value) ? '平台内部步骤' : value)
}

/**
 * selftestReasonText 返回原因的用户向文案。
 * 目录同步写入的原因本身就是中文说明,原样呈现;未登记的原因码给一句通用说明并提示按编号报障。
 */
export function selftestReasonText(reason: string): string {
  const value = reason.trim()
  if (value === '') return ''
  return (
    REASON_TEXT[value] ??
    (looksLikeCode(value) ? '平台判定这条运行时暂时不能使用,请按下方编号联系运维' : value)
  )
}
