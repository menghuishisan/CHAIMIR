// AlertsPage 展示学业预警并触发后端预警扫描。

import React, { useCallback, useMemo, useState } from 'react'
import type { GradeWarning } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Table, ResourceState } from '@chaimir/ui'
import { AlertTriangle, RefreshCw, Send } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource, usePendingAction } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { formatStudentReference, gradeWarningDetailLabel, gradeWarningStatusLabel, gradeWarningTypeLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const AlertsPage: React.FC = () => {
  const [studentId, setStudentId] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const { pendingAction, runPendingAction } = usePendingAction()
  const resource = useAsyncResource(() => api.grade.listWarnings({
    student_id: studentId || undefined,
    page: 1,
    size: 20,
  }), [studentId])

  /**
   * handleScan 调用后端扫描学业预警。
   */
  const handleScan = useCallback(async () => {
    setError(null)
    setMessage(null)
    try {
      const result = await api.grade.scanWarnings({ student_id: studentId || undefined })
      setMessage(`已扫描 ${result.scanned} 条记录，新增 ${result.created} 条预警。`)
      resource.reload()
    } catch (scanError) {
      setError(userFacingErrorMessage(scanError, '预警扫描失败，请稍后重试。'))
    }
  }, [resource, studentId])

  const columns = useMemo<TableColumn<GradeWarning>[]>(() => [
    { key: 'student', title: '学生', render: (row) => formatStudentReference(row.student_id), priority: 'primary' },
    { key: 'semester', title: '学期', render: (row) => row.semester_id ? '已指定学期' : '未指定学期' },
    { key: 'type', title: '预警类型', render: (row) => <span className={styles.status}>{gradeWarningTypeLabel(row.type)}</span> },
    { key: 'detail', title: '详情', render: (row) => gradeWarningDetailLabel(row.detail) },
    { key: 'status', title: '状态', render: (row) => gradeWarningStatusLabel(row.status) },
  ], [])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><AlertTriangle size={28} />学业预警干预</h1>
          <p className={styles.subtitle}>查看学业预警，并按当前规则重新检查学生成绩。</p>
        </div>
        <Button icon={<Send size={16} />} loading={pendingAction === 'scan'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('scan', handleScan)}>触发预警扫描</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="扫描完成">{message}</Callout>}
      <div className={styles.toolbar}>
        <Input placeholder="按学生编号筛选" value={studentId} onChange={(event) => setStudentId(event.target.value)} />
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取学业预警" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无预警" emptyDescription="当前没有学业预警记录。" ariaLabel="学业预警列表" />
        </div>
      )}
    </div>
  )
}

export default AlertsPage
