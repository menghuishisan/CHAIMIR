// 题目正文附件编辑器(题库内容表单内)。
//
// 附件不进正文文本框:正文只能存统一文件服务返回的 object_ref(后端 validateContentBodyRefs
// 会拒绝 data: 与外部直链),所以这里做的是「选文件 → 上传换 object_ref → 记进 attachments」。
// 取件同理必须换短时授权再走统一下载入口,页面不碰对象存储地址。
//
// 三种题目类型共用一个 attachments 字段(M5 架构设计 §2.2),故本组件不按类型分叉。

import { useCallback, useState } from 'react'
import { FileUp, Paperclip, Trash2, Download } from 'lucide-react'
import type { ContentAttachment } from '@chaimir/api-client'
import { Button, Callout, IconButton } from '@chaimir/ui'
import { api } from '../../app/api'
import { formatFileSize } from '../../utils/formatters'
import { userFacingErrorMessage } from '../../utils/userFacingError'

export interface ContentAttachmentsFieldProps {
  /** 已保存在正文里的附件列表 */
  attachments: ContentAttachment[]
  onChange: (attachments: ContentAttachment[]) => void
  /** 编辑已有题目时传该题 id;新建时省略(对象落在草稿前缀下) */
  resourceId?: string
}

/**
 * ContentAttachmentsField 渲染附件清单与上传入口。
 */
export function ContentAttachmentsField({
  attachments,
  onChange,
  resourceId,
}: ContentAttachmentsFieldProps) {
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState<number>()
  const [error, setError] = useState<string>()

  /** upload 上传一个文件并把返回的对象引用追加进清单。 */
  const upload = useCallback(
    async (file: File) => {
      setError(undefined)
      setUploading(true)
      try {
        const saved = await api.content.uploadAttachment(file, resourceId, setProgress)
        onChange([...attachments, saved])
      } catch (uploadError) {
        setError(userFacingErrorMessage(uploadError, '附件没有上传成功,请稍后重试。'))
      } finally {
        setUploading(false)
        setProgress(undefined)
      }
    },
    [attachments, onChange, resourceId],
  )

  /**
   * download 换取短时授权后取件。
   * 授权是一次性的,每次点下载都要重新签发 —— 这是统一文件服务的口径,不缓存 token。
   */
  const download = useCallback(
    async (attachment: ContentAttachment) => {
      setError(undefined)
      try {
        const grant = await api.content.issueAttachmentDownloadGrant({
          resource_id: resourceId ?? '',
          object_ref: attachment.object_ref,
        })
        const file = await api.storage.consumeGrant(grant.token)
        const url = URL.createObjectURL(file.blob)
        const link = document.createElement('a')
        link.href = url
        link.download = file.fileName || attachment.file_name
        link.click()
        URL.revokeObjectURL(url)
      } catch (downloadError) {
        setError(userFacingErrorMessage(downloadError, '附件没能下载,请稍后重试。'))
      }
    },
    [resourceId],
  )

  return (
    <div className="flex flex-col gap-2">
      {attachments.length > 0 ? (
        <ul className="flex flex-col gap-1">
          {attachments.map((attachment) => (
            <li
              key={attachment.object_ref}
              className="flex items-center gap-2 rounded-md border border-line px-2 py-1.5"
            >
              <Paperclip aria-hidden="true" className="size-4 shrink-0 text-ink-faint" />
              <span className="min-w-0 flex-1 truncate text-sm text-ink">{attachment.file_name}</span>
              <span className="shrink-0 font-mono text-xs tabular-nums text-ink-sub">
                {formatFileSize(attachment.size)}
              </span>
              {/* 新建题目时还没有题目 id,拿不到取件授权;此时只能删除刚上传的附件 */}
              {resourceId ? (
                <IconButton
                  aria-label={`下载 ${attachment.file_name}`}
                  icon={Download}
                  size="sm"
                  onClick={() => void download(attachment)}
                />
              ) : null}
              <IconButton
                aria-label={`移除 ${attachment.file_name}`}
                icon={Trash2}
                size="sm"
                onClick={() =>
                  onChange(attachments.filter((item) => item.object_ref !== attachment.object_ref))
                }
              />
            </li>
          ))}
        </ul>
      ) : null}

      <div className="flex items-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          leftIcon={FileUp}
          loading={uploading}
          onClick={() => document.getElementById('content-attachment-input')?.click()}
        >
          添加附件
        </Button>
        {progress !== undefined ? (
          <span className="font-mono text-xs tabular-nums text-ink-sub">已上传 {progress}%</span>
        ) : null}
        <input
          id="content-attachment-input"
          type="file"
          className="hidden"
          onChange={(event) => {
            const file = event.target.files?.[0]
            event.target.value = ''
            if (file) void upload(file)
          }}
        />
      </div>

      {error ? <Callout tone="danger">{error}</Callout> : null}
    </div>
  )
}
