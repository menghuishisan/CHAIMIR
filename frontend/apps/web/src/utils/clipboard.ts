import { toast } from '@chaimir/ui'
import { errorDiagnostics } from './userFacingError'

export interface CopyTextOptions {
  /** 被复制内容的用户向名称,用于拼失败文案(如「邀请码」「操作编号」) */
  what: string
  /** 结构化日志用的操作标识(如 teaching.course.copyInviteCode) */
  operation: string
}

/**
 * copyText 复制文本，失败时提示并记录脱敏原因，返回是否成功。
 * 成功反馈由调用方决定；前端自身失败不生成报障编号。
 */
export async function copyText(text: string, { what, operation }: CopyTextOptions): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch (error) {
    toast.error(`复制没有成功,请手动选中${what}后复制。`)
    console.error('剪贴板写入失败', { operation, reason: 'clipboard-write-failed', error: errorDiagnostics(error) })
    return false
  }
}
