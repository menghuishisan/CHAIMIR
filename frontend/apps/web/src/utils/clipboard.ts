// clipboard 提供全站唯一的复制入口:邀请码、分享链接、操作编号等都走这里。
// 剪贴板是浏览器能力,权限被拒或非安全上下文都会失败,因此必须显式处理失败而不是静默(CLAUDE.md §8);
// 失败属前端自身错误,只给场景化用户向文案、不生成编号,技术原因进 console(规范 §6.7 B)。

import { toast } from '@chaimir/ui'

export interface CopyTextOptions {
  /** 被复制内容的用户向名称,用于拼失败文案(如「邀请码」「操作编号」) */
  what: string
  /** 结构化日志用的操作标识(如 teaching.course.copyInviteCode) */
  operation: string
}

/**
 * copyText 把文本写入剪贴板。
 * 失败时给用户向 Toast 并留结构化日志;成功与否由返回值交回调用方,
 * 让调用方自行决定成功反馈(就近的「已复制」标记或 Toast),不在这里替它选。
 */
export async function copyText(text: string, { what, operation }: CopyTextOptions): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch (error) {
    toast.error(`复制没有成功,请手动选中${what}后复制。`)
    console.error('剪贴板写入失败', { operation, reason: 'clipboard-write-failed', error })
    return false
  }
}
