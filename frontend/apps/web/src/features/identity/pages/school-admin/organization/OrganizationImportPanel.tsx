// OrganizationImportPanel 处理组织结构模板、服务端预览和提交批次。

import React, { useMemo, useRef, useState } from 'react'
import type { ImportRowResult } from '@chaimir/api-client'
import { IMPORT_TEMPLATE_FORMAT } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Table } from '@chaimir/ui'
import { Download, Upload } from 'lucide-react'
import { api } from '../../../../../app/api'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import styles from '../../identity-admin.module.css'

/** OrganizationImportPanel 以服务端 preview_id 为导入权威状态。 */
export function OrganizationImportPanel(): React.ReactElement {
  const [file, setFile] = useState<File | null>(null)
  const [previewId, setPreviewId] = useState('')
  const [rows, setRows] = useState<ImportRowResult[]>([])
  const [summary, setSummary] = useState<{ total: number; valid: number; invalid: number }>()
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const selectionVersionRef = useRef(0)

  /** selectFile 切换源文件时立即废弃旧预览，防止提交与当前文件不一致的批次。 */
  const selectFile = (nextFile: File | null) => {
    selectionVersionRef.current += 1
    setFile(nextFile)
    setPreviewId('')
    setRows([])
    setSummary(undefined)
    setError('')
    setBusy('')
  }

  /** downloadTemplate 下载后端当前版本的组织导入模板。 */
  const downloadTemplate = async () => {
    setBusy('template')
    setError('')
    try {
      const blob = await api.identity.downloadOrgImportTemplate({ format: IMPORT_TEMPLATE_FORMAT.XLSX })
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = 'organization-template.xlsx'
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '组织模板下载失败，请稍后重试。'))
    } finally {
      setBusy('')
    }
  }

  /** previewImport 上传文件并保存后端生成的预览编号。 */
  const previewImport = async () => {
    if (!file) return
    const selectionVersion = selectionVersionRef.current
    setBusy('preview')
    setError('')
    try {
      const preview = await api.identity.previewOrgImport(file)
      if (selectionVersion !== selectionVersionRef.current) return
      setPreviewId(preview.preview_id)
      setRows(preview.rows)
      setSummary({ total: preview.total, valid: preview.valid, invalid: preview.invalid })
    } catch (actionError) {
      if (selectionVersion !== selectionVersionRef.current) return
      setError(userFacingErrorMessage(actionError, '组织导入预览失败，请检查文件内容。'))
    } finally {
      if (selectionVersion === selectionVersionRef.current) setBusy('')
    }
  }

  /** commitImport 提交已通过服务端校验的组织预览批次。 */
  const commitImport = async () => {
    if (!previewId) return
    setBusy('commit')
    setError('')
    try {
      const result = await api.identity.commitOrgImport({ preview_id: previewId })
      setPreviewId('')
      setFile(null)
      setSummary({ total: result.batch.total, valid: result.batch.success, invalid: result.batch.failed })
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '组织导入提交失败，请稍后重试。'))
    } finally {
      setBusy('')
    }
  }

  const columns = useMemo<TableColumn<ImportRowResult>[]>(() => [
    { key: 'line', title: '行号', dataIndex: 'line', priority: 'primary' },
    { key: 'result', title: '校验结果', render: (row) => row.error || '可导入' },
  ], [])

  return (
    <section className={styles.panel}>
      <h2>批量导入组织</h2>
      {error && <Callout variant="danger" title="导入未完成">{error}</Callout>}
      {summary && <Callout variant={summary.invalid ? 'warning' : 'success'} title="导入状态">共 {summary.total} 行，可导入 {summary.valid} 行，需修正 {summary.invalid} 行。</Callout>}
      <input type="file" accept=".xlsx,.csv" onChange={(event) => selectFile(event.target.files?.[0] || null)} />
      <div className={styles.actions}>
        <Button variant="outline" icon={<Download size={15} />} loading={busy === 'template'} onClick={() => void downloadTemplate()}>下载模板</Button>
        <Button variant="outline" icon={<Upload size={15} />} disabled={!file} loading={busy === 'preview'} onClick={() => void previewImport()}>生成预览</Button>
        <Button disabled={!previewId || Boolean(summary?.invalid)} loading={busy === 'commit'} onClick={() => void commitImport()}>提交导入</Button>
      </div>
      <Table columns={columns} rows={rows} rowKey={(row) => String(row.line)} emptyTitle="暂无预览" emptyDescription="选择文件并生成预览后显示校验结果。" ariaLabel="组织导入预览" />
    </section>
  )
}
