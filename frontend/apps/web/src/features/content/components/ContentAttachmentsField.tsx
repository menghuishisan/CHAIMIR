// 编辑正文附件；对象存取统一经过文件服务，上传与表单提交互斥。

import { useRef, useState, type Dispatch, type SetStateAction } from 'react'
import { FileUp, Paperclip, Trash2, Download } from 'lucide-react'
import type { ContentAttachment } from '@chaimir/api-client'
import { Button, Callout, IconButton } from '@chaimir/ui'
import { api } from '../../../app/api'
import { downloadAttachment } from '../../../utils/downloadAttachment'
import { formatFileSize } from '../../../utils/formatters'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

export interface ContentAttachmentsFieldProps {
  /** 已保存在正文里的附件列表 */
  attachments: ContentAttachment[]
  onChange: Dispatch<SetStateAction<ContentAttachment[]>>
  uploading: boolean
  onUploadingChange: (uploading: boolean) => void
  disabled?: boolean
  /** 编辑已有题目时传该题 id;新建时省略(对象落在草稿前缀下) */
  resourceId?: string
}

/**
 * ContentAttachmentsField 渲染附件清单与上传入口。
 */
export function ContentAttachmentsField({
  attachments,
  onChange,
  uploading,
  onUploadingChange,
  disabled = false,
  resourceId,
}: ContentAttachmentsFieldProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [progress, setProgress] = useState<number>()
  const [error, setError] = useState<string>()

  /** upload 上传一个文件并把返回的对象引用追加进清单。 */
  async function upload(file: File) {
    if (disabled || uploading) return
    setError(undefined)
    onUploadingChange(true)
    try {
      const saved = await api.content.uploadAttachment(file, resourceId, setProgress)
      onChange((current) => [...current, saved])
    } catch (uploadError) {
      setError(userFacingErrorMessage(uploadError, '附件没有上传成功,请稍后重试。'))
    } finally {
      onUploadingChange(false)
      setProgress(undefined)
    }
  }

  /**
   * download 换取短时授权后取件。
   * 授权是一次性的,每次点下载都要重新签发 —— 这是统一文件服务的口径,不缓存 token。
   */
  async function download(attachment: ContentAttachment) {
    setError(undefined)
    try {
      const grant = await api.content.issueAttachmentDownloadGrant({
        resource_id: resourceId ?? '',
        object_ref: attachment.object_ref,
      })
      const file = await api.storage.consumeGrant(grant.token)
      downloadAttachment(file)
    } catch (downloadError) {
      setError(userFacingErrorMessage(downloadError, '附件没能下载,请稍后重试。'))
    }
  }

  return (
    <div className="flex flex-col gap-2">
      {attachments.length > 0 ? (
        <ul className="flex flex-col gap-1">
          {attachments.map((attachment) => (
            <li
              key={attachment.object_ref}
              className="flex items-center gap-2 rounded-md bg-surface-hover px-2 py-1.5"
            >
              <Paperclip aria-hidden="true" className="size-4 shrink-0 text-ink-faint" />
              <span className="min-w-0 flex-1 truncate text-sm text-ink">
                {attachment.file_name}
              </span>
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
                disabled={disabled}
                onClick={() =>
                  onChange((current) =>
                    current.filter((item) => item.object_ref !== attachment.object_ref)
                  )
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
          disabled={disabled}
          onClick={() => inputRef.current?.click()}
        >
          添加附件
        </Button>
        {progress !== undefined ? (
          <span className="font-mono text-xs tabular-nums text-ink-sub">已上传 {progress}%</span>
        ) : null}
        <input
          ref={inputRef}
          type="file"
          hidden
          disabled={disabled || uploading}
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
