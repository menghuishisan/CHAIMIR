// UserImportPage 处理账号批量导入，预览与提交状态以后端 preview_id 为准。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, ImportRowResult } from '@chaimir/api-client'
import { IMPORT_TEMPLATE_FORMAT } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Select, Table } from '@chaimir/ui'
import { Download, Upload } from 'lucide-react'
import { api } from '../../../../../app/api'
import styles from '../../identity-admin.module.css'
import { accountImportTargetOptions } from '../../../../../utils/index'

const UserImportPage: React.FC = () => {
  const [targetType, setTargetType] = useState<'student' | 'teacher'>('student')
  const [file, setFile] = useState<File | null>(null)
  const [previewId, setPreviewId] = useState('')
  const [rows, setRows] = useState<ImportRowResult[]>([])
  const [summary, setSummary] = useState<{ total: number; valid: number; invalid: number } | null>(null)
  const [activationCodes, setActivationCodes] = useState<string[]>([])
  const [submitting, setSubmitting] = useState<'preview' | 'commit' | 'template' | null>(null)
  const [error, setError] = useState<string | null>(null)

  /**
   * handleTemplate 通过后端模板接口下载导入模板。
   */
  const handleTemplate = useCallback(async () => {
    setSubmitting('template')
    setError(null)
    try {
      const blob = await api.identity.downloadAccountImportTemplate({ type: targetType, format: IMPORT_TEMPLATE_FORMAT.XLSX })
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `${targetType}-accounts-template.xlsx`
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (templateError) {
      setError((templateError as ApiError).message || '模板下载失败，请稍后重试。')
    } finally {
      setSubmitting(null)
    }
  }, [targetType])

  /**
   * handlePreview 上传文件并获取服务端持久化的预览编号。
   */
  const handlePreview = useCallback(async () => {
    if (!file) {
      setError('请选择需要导入的文件。')
      return
    }
    setSubmitting('preview')
    setError(null)
    setActivationCodes([])
    try {
      const preview = await api.identity.previewAccountImport(targetType, file)
      setPreviewId(preview.preview_id)
      setSummary({ total: preview.total, valid: preview.valid, invalid: preview.invalid })
      setRows(preview.rows)
    } catch (previewError) {
      setError((previewError as ApiError).message || '导入预览失败，请检查文件内容。')
    } finally {
      setSubmitting(null)
    }
  }, [file, targetType])

  /**
   * handleCommit 提交后端预览批次，导入结果以后端返回为准。
   */
  const handleCommit = useCallback(async () => {
    if (!previewId) {
      setError('请先完成导入预览。')
      return
    }
    setSubmitting('commit')
    setError(null)
    try {
      const result = await api.identity.commitAccountImport({ preview_id: previewId })
      setActivationCodes((result.activation_codes || []).map((item) => `${item.no} ${item.name} ${item.activation_code}`))
      setSummary({ total: result.batch.total, valid: result.batch.success, invalid: result.batch.failed })
    } catch (commitError) {
      setError((commitError as ApiError).message || '提交导入失败，请稍后重试。')
    } finally {
      setSubmitting(null)
    }
  }, [previewId])

  const columns = useMemo<TableColumn<ImportRowResult>[]>(() => [
    { key: 'line', title: '行号', dataIndex: 'line', priority: 'primary' },
    { key: 'error', title: '校验结果', render: (row) => row.error || '可导入', priority: 'primary' },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Upload size={28} />
            账号导入向导
          </h1>
          <p className={styles.subtitle}>模板下载、导入预览和提交批次都通过后端接口完成。</p>
        </div>
        <Button variant="outline" loading={submitting === 'template'} icon={<Download size={16} />} onClick={handleTemplate}>
          下载模板
        </Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {summary && (
        <Callout variant={summary.invalid > 0 ? 'warning' : 'success'} title="预览结果">
          共 {summary.total} 行，可导入 {summary.valid} 行，需修正 {summary.invalid} 行。
        </Callout>
      )}

      <section className={styles.panel}>
        <h2>上传文件</h2>
        <div className={styles.formGrid}>
          <label className={styles.field}>
            导入对象
            <Select fullWidth value={targetType} options={accountImportTargetOptions} onChange={(value) => setTargetType(value as 'student' | 'teacher')} />
          </label>
          <label className={styles.field}>
            导入文件
            <input type="file" accept=".xlsx,.csv" onChange={(event) => setFile(event.target.files?.[0] || null)} />
          </label>
        </div>
        <div className={styles.dropzone}>{file ? file.name : '请选择后端模板格式的 Excel 或 CSV 文件'}</div>
        <div className={styles.actions}>
          <Button loading={submitting === 'preview'} onClick={handlePreview}>
            生成预览
          </Button>
          <Button loading={submitting === 'commit'} disabled={!previewId || Boolean(summary?.invalid)} onClick={handleCommit}>
            提交可导入数据
          </Button>
        </div>
      </section>

      <section className={styles.panel}>
        <h2>校验明细</h2>
        <Table
          columns={columns}
          rows={rows}
          rowKey={(row) => String(row.line)}
          emptyTitle="暂无预览"
          emptyDescription="上传文件并生成预览后，会展示后端校验结果。"
          ariaLabel="账号导入校验明细"
        />
      </section>

      {activationCodes.length > 0 && (
        <section className={styles.panel}>
          <h2>激活码</h2>
          {activationCodes.map((item) => (
            <span className={styles.status} key={item}>{item}</span>
          ))}
        </section>
      )}
    </div>
  )
}

export default UserImportPage
