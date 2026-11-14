// ImportPreviewRecovery 账号与组织导入抽屉里的「未完成的导入」:服务端保存的预览可以继续或明确放弃。
// 它放在抽屉里,而浮层里不出现分页(规范 §6.5.4 A):未完成的预览只属于当前操作人、通常只有一两份,
// 所以按页取完后一次列出;没有未完成的预览时不占位。以服务端预览为权威,不依赖本机文件或缓存。

import { useEffect, useRef, useState } from 'react'
import {
  PAGINATION_MAX_SIZE,
  type ImportPreviewResponse,
  type ImportPreviewSummary,
} from '@chaimir/api-client'
import { Button, Callout, Skeleton } from '@chaimir/ui'
import { api } from '../../../app/api'
import { useAsyncResource } from '../../../hooks'
import { collectAllPages } from '../../../utils/allPages'
import { formatDateTime } from '../../../utils/formatters'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

interface ImportPreviewRecoveryProps {
  target: 'teacher' | 'student' | 'org'
  disabled: boolean
  onRestore: (preview: ImportPreviewResponse) => void
}

/** ImportPreviewRecovery 列出当前操作人未完成的导入预览,可继续或放弃 */
export function ImportPreviewRecovery({ target, disabled, onRestore }: ImportPreviewRecoveryProps) {
  const previews = useAsyncResource<ImportPreviewSummary[]>(
    () =>
      collectAllPages(
        (params) => api.identity.listImportPreviews({ ...params, type: target }),
        PAGINATION_MAX_SIZE
      ),
    [target],
    (list) => list.length === 0
  )
  const [working, setWorking] = useState(false)
  const [error, setError] = useState<string>()
  const active = useRef(true)
  useEffect(() => {
    active.current = true
    return () => {
      active.current = false
    }
  }, [])

  /** runAction 继续或放弃一份预览;离开或切换导入对象后迟到的结果不再写回 */
  const runAction = async (previewId: string, discard: boolean) => {
    if (working || disabled) return
    setWorking(true)
    setError(undefined)
    try {
      if (discard) {
        await api.identity.discardImportPreview(previewId)
        if (active.current) previews.reload()
      } else {
        const preview = await api.identity.getImportPreview(previewId)
        if (active.current) onRestore(preview)
      }
    } catch (cause) {
      if (active.current) {
        setError(
          userFacingErrorMessage(
            cause,
            discard ? '预览没有放弃成功,请稍后重试。' : '预览没有恢复成功,请稍后重试。'
          )
        )
      }
    } finally {
      if (active.current) setWorking(false)
    }
  }

  if (previews.status === 'empty') return null
  if (previews.status === 'loading') return <Skeleton variant="line" lines={2} />
  if (previews.status === 'error') {
    return (
      <p className="flex flex-wrap items-center gap-2 text-sm text-ink-sub">
        未完成的导入暂时读不到。
        <Button variant="ghost" size="sm" onClick={previews.reload}>
          重新读取
        </Button>
      </p>
    )
  }

  return (
    <section aria-label="未完成的导入" className="flex flex-col gap-2">
      <h3 className="text-sm font-semibold text-ink">未完成的导入</h3>
      {error ? <Callout tone="danger">{error}</Callout> : null}
      <ul className="divide-y divide-line rounded-md bg-surface-hover px-3">
        {(previews.data ?? []).map((item) => (
          <li key={item.preview_id} className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3">
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium text-ink">{item.file_name}</span>
              <span className="block text-xs text-ink-sub">
                共 {item.total} 行{item.invalid > 0 ? `,${item.invalid} 行有问题` : ',没有问题行'} ·
                保留到 {formatDateTime(item.expire_at)}
              </span>
            </span>
            <span className="flex shrink-0 items-center gap-2">
              <Button
                type="button"
                size="sm"
                disabled={working || disabled}
                onClick={() => void runAction(item.preview_id, false)}
              >
                继续
              </Button>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                disabled={working || disabled}
                onClick={() => void runAction(item.preview_id, true)}
              >
                放弃
              </Button>
            </span>
          </li>
        ))}
      </ul>
    </section>
  )
}
