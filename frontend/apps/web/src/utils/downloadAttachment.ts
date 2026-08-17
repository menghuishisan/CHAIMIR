// downloadAttachment 统一触发后端附件响应的浏览器下载。

import type { AttachmentResponse } from '@chaimir/api-client'

/**
 * downloadAttachment 把 api-client 已验证的附件响应交给浏览器保存。
 * 文件名只能使用后端 Content-Disposition 中已解析出的 fileName；页面只负责签发授权和交互反馈。
 */
export function downloadAttachment({ blob, fileName }: AttachmentResponse): void {
  const url = URL.createObjectURL(blob)
  try {
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = fileName
    anchor.click()
  } finally {
    URL.revokeObjectURL(url)
  }
}
