// AssignmentResultPage 展示作业提交后的真实评分结果。

import React, { useMemo } from 'react'
import type { Submission } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table, ResourceState } from '@chaimir/ui'
import { FileCheck, RefreshCw } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { formatDateTime, submissionStatusLabel } from '../../../../../utils/index'

const AssignmentResultPage: React.FC = () => {
  const [searchParams] = useSearchParams()
  const submissionId = searchParams.get('submissionId') || ''
  const resource = useAsyncResource(
    () => submissionId ? api.teaching.getSubmission(submissionId) : Promise.resolve(null),
    [submissionId],
    (value) => value === null,
  )

  const columns = useMemo<TableColumn<Submission>[]>(() => [
    { key: 'attempt', title: '提交次数', render: (row) => `第 ${row.attempt_no} 次`, priority: 'primary' },
    { key: 'score', title: '最终得分', render: (row) => (row.final_score === undefined ? '待评分' : row.final_score.toFixed(1)) },
    { key: 'status', title: '状态', render: (row) => submissionStatusLabel(row.status) },
    { key: 'late', title: '迟交', render: (row) => (row.is_late ? '是' : '否') },
    { key: 'submitted', title: '提交时间', render: (row) => formatDateTime(row.submitted_at) },
    { key: 'comment', title: '教师评语', render: (row) => row.comment || '暂无' },
  ], [])

  const rows = resource.data ? [resource.data] : []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><FileCheck size={28} />作业评分结果</h1>
          <p className={styles.subtitle}>查看本作业的提交记录、评分和教师评语。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取评分结果" />}
      {resource.status === 'empty' && <ResourceState status="empty" title="缺少提交记录" description="请从作业提交完成页进入评分结果。" />}
      {resource.status === 'success' && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无提交结果" emptyDescription="当前作业还没有可查看的提交记录。" ariaLabel="学生作业评分结果" />
        </div>
      )}
    </div>
  )
}

export default AssignmentResultPage
