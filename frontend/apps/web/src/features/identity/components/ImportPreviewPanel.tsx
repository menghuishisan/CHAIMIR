// ImportPreviewPanel 统一渲染身份域批量导入的服务端预览结果。
// 账号和组织导入的业务接口不同,但逐行错误、统计和确认前提示完全相同,页面只保留各自流程。

import type { ImportPreviewResponse } from '@chaimir/api-client'
import { Badge, Callout, Table, type TableColumn } from '@chaimir/ui'

interface ImportPreviewPanelProps {
  preview?: ImportPreviewResponse
  emptyDescription: string
}

/** ImportPreviewPanel 展示导入预览统计与逐行错误,不负责上传或提交。 */
export function ImportPreviewPanel({ preview, emptyDescription }: ImportPreviewPanelProps) {
  if (!preview) {
    return <Callout tone="info">{emptyDescription}</Callout>
  }

  const errorRows = preview.rows.filter((row) => row.error)
  const errorColumns: TableColumn<{ line: number; error?: string }>[] = [
    { key: 'line', header: '行号', align: 'right', mono: true },
    {
      key: 'error',
      header: '问题',
      render: (row) => <span className="text-sm text-danger">{row.error}</span>,
    },
  ]

  return (
    <div className="flex flex-col gap-3 well p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Badge tone="neutral">共 {preview.total} 行</Badge>
        <Badge tone="success">可导入 {preview.valid} 行</Badge>
        {preview.invalid > 0 ? <Badge tone="danger">有问题 {preview.invalid} 行</Badge> : null}
      </div>

      {preview.invalid > 0 ? (
        <>
          <Callout tone="warning" title="有问题的行不会被导入">
            修正这些行后重新上传,或直接提交只导入没有问题的行。
          </Callout>
          <Table columns={errorColumns} data={errorRows} rowKey={(row) => String(row.line)} />
        </>
      ) : (
        <Callout tone="success" title="校验通过">
          全部 {preview.valid} 行都可以导入。
        </Callout>
      )}
    </div>
  )
}
