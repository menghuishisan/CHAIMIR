const RESULT_TEXT: Record<string, string> = {
  passed: '通过',
  failed: '未通过',
  running: '正在自检',
  pending: '待自检',
  blocked: '被阻断,暂时不能自检',
}

const STAGE_TEXT: Record<string, string> = {
  selftest: '平台自检',
  prepare: '准备环境',
  workspace: '准备初始工作区',
  toolchain: '检查运行工具',
  capability: '检查链上操作',
  recycle: '回收环境',
}

const REASON_TEXT: Record<string, string> = {
  runtime_execution_contract_changed: '运行时的执行声明改过了,需要重新自检',
}

function looksLikeCode(value: string): boolean {
  return /^[a-z0-9_-]+$/.test(value)
}

export function selftestResultText(result: string): string {
  const value = result.trim()
  return RESULT_TEXT[value] ?? (looksLikeCode(value) ? '状态未知' : value)
}

export function selftestStageText(stage: string): string {
  const value = stage.trim()
  return STAGE_TEXT[value] ?? (looksLikeCode(value) ? '平台内部步骤' : value)
}

/** 目录提供的中文原因原样呈现；未知内部码用通用提示替代。 */
export function selftestReasonText(reason: string): string {
  const value = reason.trim()
  if (value === '') return ''
  return (
    REASON_TEXT[value] ??
    (looksLikeCode(value) ? '平台判定这条运行时暂时不能使用,请按下方编号联系运维' : value)
  )
}
