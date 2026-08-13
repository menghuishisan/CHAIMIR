// 品牌图片上传控件(校徽与课程封面共用)。
//
// 校徽和封面都存对象引用而不是地址:生产安全头把图片来源限定在本站与内联数据,
// 外链图片会被浏览器拦掉;而且不该把师生的访问记录暴露给外部站点。
// 于是这两个字段都不是「填地址」而是「选文件上传」。
//
// 本组件只管共同流程:选文件、上传中、进度、失败提示。上传之后怎么处理(校徽即时生效、
// 封面等表单提交)差别很大,故一律由调用方在 onUpload 里自己决定,组件不碰业务语义。
//
// 无障碍:真实的 input[type=file] 是唯一可聚焦控件(sr-only 而不是 hidden ——
// hidden 会被移出无障碍树,字段标签就指不到任何控件),可见按钮只是鼠标视觉入口,
// 故 aria-hidden 且不进 Tab 序列,避免同一动作出现两个焦点停靠点。

import { useCallback, useState } from 'react'
import type { ReactNode } from 'react'
import { ImageUp } from 'lucide-react'
import { Button, Callout } from '@chaimir/ui'
import { userFacingErrorMessage } from '../../utils/userFacingError'

export interface ImageUploadFieldProps {
  /**
   * 真实文件输入的 id,由调用方传入并同时给 FormField 的 htmlFor。
   * 必须由外部给:字段标签在 FormField 上,标签要指向本组件内部那个 input 才算关联成功。
   */
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
}

/** readImageDataUrl 把本地文件读成内联预览地址,供「上传后尚未保存」的字段先显示出来。 */
export function readImageDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '')
    reader.onerror = () => reject(reader.error ?? new Error('read failed'))
    reader.readAsDataURL(file)
  })
}

/**
 * ImageUploadField 渲染预览 + 选择文件 + 移除。
 */
export function ImageUploadField({
  inputId,
  preview,
  onUpload,
  onCleared,
  hasImage,
  failureMessage,
}: ImageUploadFieldProps) {
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState<number>()
  const [error, setError] = useState<string>()

  /** upload 执行一次上传,失败只留在本控件内就近提示,不影响表单其余字段。 */
  const upload = useCallback(
    async (file: File) => {
      setError(undefined)
      setUploading(true)
      try {
        await onUpload(file, setProgress)
      } catch (uploadError) {
        setError(userFacingErrorMessage(uploadError, failureMessage))
      } finally {
        setUploading(false)
        setProgress(undefined)
      }
    },
    [failureMessage, onUpload],
  )

  /** clear 执行移除;移除可能要请求后端(如校徽即时生效),失败与上传失败同处提示。 */
  const clear = useCallback(async () => {
    if (!onCleared) return
    setError(undefined)
    try {
      await onCleared()
    } catch (clearError) {
      setError(userFacingErrorMessage(clearError, failureMessage))
    }
  }, [failureMessage, onCleared])

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-start gap-3">
        {preview}
        <div className="flex min-w-0 flex-col gap-2">
          <input
            id={inputId}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            className="sr-only"
            onChange={(event) => {
              const file = event.target.files?.[0]
              event.target.value = ''
              if (file) void upload(file)
            }}
          />
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              leftIcon={ImageUp}
              loading={uploading}
              aria-hidden="true"
              tabIndex={-1}
              onClick={() => document.getElementById(inputId)?.click()}
            >
              {hasImage ? '换一张' : '选择图片'}
            </Button>
            {hasImage && onCleared ? (
              <Button type="button" variant="ghost" size="sm" onClick={() => void clear()}>
                不用图片
              </Button>
            ) : null}
            {/* 上传进度对读屏播报:可见按钮已 aria-hidden,进度是唯一的过程反馈 */}
            <span role="status" className="font-mono text-xs tabular-nums text-ink-sub">
              {progress !== undefined ? `已上传 ${progress}%` : ''}
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
