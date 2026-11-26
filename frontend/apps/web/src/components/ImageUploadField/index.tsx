import { useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { ImageUp } from 'lucide-react'
import { Button, Callout } from '@chaimir/ui'
import { userFacingErrorMessage } from '../../utils/userFacingError'

export interface ImageUploadFieldProps {
  /** 与外部 FormField 的 htmlFor 对应的文件输入 id。 */
  inputId: string
  /** 预览区:由调用方渲染 TenantCrest 或 CoverImage */
  preview: ReactNode
  /** 上传并按各自语义处理结果;抛错即由本组件显示失败提示 */
  onUpload: (file: File, onProgress: (progress: number) => void) => Promise<void>
  /** 移除图片;不传则不显示移除按钮。允许是异步动作,失败与上传失败在同一处就近提示 */
  onCleared?: () => void | Promise<void>
  /** 是否已有图片,决定按钮文案与是否可移除 */
  hasImage: boolean
  /** 这个字段更新失败时的兜底提示;上传与移除共用,故文案要对两种动作都成立 */
  failureMessage: string
  disabled?: boolean
  /** 处理开始与结束时通知调用方，用于协调包含图片的表单提交。 */
  onBusyChange?: (busy: boolean) => void
}

/** readImageDataUrl 读取本地预览，失败或取消时拒绝本次图片更新。 */
export function readImageDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === 'string') resolve(reader.result)
      else reject(new Error('Image preview is unavailable'))
    }
    reader.onerror = () => reject(reader.error ?? new Error('read failed'))
    reader.onabort = () => reject(new Error('Image preview was canceled'))
    reader.readAsDataURL(file)
  })
}

/** ImageUploadField 串行处理品牌图片的上传与移除，结果由调用方保存。 */
export function ImageUploadField({
  inputId,
  preview,
  onUpload,
  onCleared,
  hasImage,
  failureMessage,
  disabled = false,
  onBusyChange,
}: ImageUploadFieldProps) {
  const [pending, setPending] = useState<'upload' | 'clear'>()
  // 同一轮渲染的回调也必须互斥，状态负责显示，ref 负责立即拦截重入。
  const active = useRef(false)
  const [progress, setProgress] = useState<number>()
  const [error, setError] = useState<string>()

  const run = async (action: 'upload' | 'clear', operation: () => void | Promise<void>) => {
    if (disabled || active.current) return
    active.current = true
    setPending(action)
    setError(undefined)
    try {
      onBusyChange?.(true)
      await operation()
    } catch (cause) {
      setError(userFacingErrorMessage(cause, failureMessage))
    } finally {
      active.current = false
      setPending(undefined)
      setProgress(undefined)
      onBusyChange?.(false)
    }
  }
  const busy = pending !== undefined

  return (
    <div className="flex flex-col gap-2" aria-busy={busy}>
      <div className="flex items-start gap-3">
        {preview}
        <div className="flex min-w-0 flex-col gap-2">
          <input
            id={inputId}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            className="sr-only"
            disabled={disabled || busy}
            onChange={(event) => {
              const file = event.target.files?.[0]
              event.target.value = ''
              if (file) void run('upload', () => onUpload(file, setProgress))
            }}
          />
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              leftIcon={ImageUp}
              loading={pending === 'upload'}
              disabled={disabled || pending === 'clear'}
              aria-hidden="true"
              tabIndex={-1}
              onClick={() => document.getElementById(inputId)?.click()}
            >
              {hasImage ? '换一张' : '选择图片'}
            </Button>
            {hasImage && onCleared ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                loading={pending === 'clear'}
                disabled={disabled || pending === 'upload'}
                onClick={() => void run('clear', onCleared)}
              >
                不用图片
              </Button>
            ) : null}
            {/* 上传进度对读屏播报:可见按钮已 aria-hidden,进度是唯一的过程反馈 */}
            <span role="status" className="font-mono text-xs tabular-nums text-ink-sub">
              {progress !== undefined
                ? `已上传 ${progress}%`
                : pending === 'upload'
                  ? '正在上传图片'
                  : pending === 'clear'
                    ? '正在移除图片'
                    : ''}
            </span>
          </div>
          <p className="text-xs text-ink-sub">支持 JPG、PNG、WEBP。</p>
        </div>
      </div>
      {error ? (
        <Callout tone="danger">
          <span role="alert">{error}</span>
        </Callout>
      ) : null}
    </div>
  )
}
