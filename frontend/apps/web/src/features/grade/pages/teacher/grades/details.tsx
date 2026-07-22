// TeacherGradesDetailsPage 展示成绩审核明细状态，避免前端伪造未开放的调分能力。

import React, { useMemo } from 'react'
import type { GradeReview } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { Calculator, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { formatDateTime, gradeReviewStatusLabel } from '../../../../../utils/index'


const TeacherGradesDetailsPage: React.FC = () => {
  const resource = useAsyncResource(() => api.grade.listOwnReviews({ page: 1, size: 20 }), [])

  const columns = useMemo<TableColumn<GradeReview>[]>(() => [
    { key: 'course', title: '课程编号', dataIndex: 'course_id', priority: 'primary' },
    { key: 'semester', title: '学期', render: (row) => row.semester_id || '未指定' },
    { key: 'comment', title: '提交说明', render: (row) => row.comment || '无' },
    { key: 'status', title: '审核状态', render: (row) => <span className={styles.status}>{gradeReviewStatusLabel(row.status)}</span> },
    { key: 'locked', title: '发布锁定', render: (row) => (row.is_locked ? '已锁定' : '未锁定') },
    { key: 'reviewed', title: '审核时间', render: (row) => (row.reviewed_at ? formatDateTime(row.reviewed_at) : '待审核') },
  ], [])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Calculator size={28} />成绩报送明细</h1>
          <p className={styles.subtitle}>查看成绩报送、审核和锁定进度。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取成绩明细" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无成绩明细" emptyDescription="当前没有可查看的成绩报送明细。" ariaLabel="成绩报送明细列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherGradesDetailsPage
