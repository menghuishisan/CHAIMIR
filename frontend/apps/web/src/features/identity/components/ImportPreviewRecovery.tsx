// ImportPreviewRecovery供账号与组织导入共用服务端预览发现、恢复和明确放弃。
import { useEffect, useRef, useState } from 'react'
import { FileSpreadsheet } from 'lucide-react'
import type { ImportPreviewResponse, ImportPreviewSummary } from '@chaimir/api-client'
import { Button, Callout, DataPanel, Pagination, Table, type TableColumn } from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { usePagedResource } from '../../../hooks'
import { formatDateTime } from '../../../utils/formatters'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

interface ImportPreviewRecoveryProps {
  target: 'teacher' | 'student' | 'org'
  disabled: boolean
  onRestore: (preview: ImportPreviewResponse) => void
}

/** ImportPreviewRecovery关闭后仍以服务端预览为权威,不依赖本机文件或缓存。 */
export function ImportPreviewRecovery({ target, disabled, onRestore }: ImportPreviewRecoveryProps) {
  const previews = usePagedResource(
    (params) => api.identity.listImportPreviews({ ...params, type: target }),
    [target]
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

  /** runAction避免离开或切换导入目标后迟到的读取恢复到另一表单。 */
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
      if (active.current)
        setError(
          userFacingErrorMessage(
            cause,
            discard ? '预览未能放弃,请稍后重试。' : '预览未能恢复,请稍后重试。'
          )
        )
    } finally {
      if (active.current) setWorking(false)
    }
  }
  const columns: TableColumn<ImportPreviewSummary>[] = [
    { key: 'file_name', header: '已保存的文件' },
    { key: 'total', header: '总行数', align: 'right' },
    { key: 'invalid', header: '问题行数', align: 'right' },
    { key: 'expire_at', header: '有效期至', render: (item) => formatDateTime(item.expire_at) },
    {
      key: 'actions',
      header: '操作',
      render: (item) => (
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            size="sm"
            disabled={working || disabled}
            onClick={() => void runAction(item.preview_id, false)}
          >
            继续预览
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={working || disabled}
            onClick={() => void runAction(item.preview_id, true)}
          >
            放弃预览
          </Button>
        </div>
      ),
    },
  ]
  return (
    <DataPanel
      label="未完成的导入预览"
      footer={
        <Pagination
          page={previews.page}
          pageSize={previews.pageSize}
          total={previews.total}
          onPageChange={previews.setPage}
        />
      }
    >
      {error ? <Callout tone="danger">{error}</Callout> : null}
      <ResourceState
        resource={previews}
        emptyIcon={FileSpreadsheet}
        emptyTitle="没有未完成的导入"
        emptyDescription="选择文件后上传校验,或稍后回来继续已保存的预览。"
      >
        {(data) => (
          <Table
            columns={columns}
            data={data.list}
            rowKey={(item) => item.preview_id}
            elevated={false}
          />
        )}
      </ResourceState>
    </DataPanel>
  )
}
