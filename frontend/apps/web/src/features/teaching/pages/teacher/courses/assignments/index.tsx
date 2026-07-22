// TeacherCourseAssignmentsPage 以课程为入口管理作业，避免前端伪造不存在的作业列表接口。

import React, { useMemo } from 'react'
import type { Course } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { FileText, Plus, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../../hooks'
import styles from '../../../teaching.module.css'
import { courseStatusLabel } from '../../../../../../utils/index'

const TeacherCourseAssignmentsPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(() => api.teaching.getCourses({ role: 'teacher', page: 1, size: 20 }), [])

  const columns = useMemo<TableColumn<Course>[]>(() => [
    { key: 'name', title: '课程名称', dataIndex: 'name', priority: 'primary' },
    { key: 'semester', title: '学期', dataIndex: 'semester' },
    { key: 'credits', title: '学分', render: (row) => row.credits.toFixed(1) },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{courseStatusLabel(row.status)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          <Button size="sm" icon={<Plus size={14} />} onClick={() => navigate(`/teacher/courses/assignments/edit?courseId=${row.id}`)}>新建作业</Button>
          <Button variant="outline" size="sm" onClick={() => navigate(`/teacher/courses/${row.id}/outline`)}>查看大纲</Button>
        </div>
      ),
    },
  ], [navigate])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><FileText size={28} />作业发布与管理</h1>
          <p className={styles.subtitle}>选择课程后创建作业，并维护作业内容和提交要求。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取课程" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无课程" emptyDescription="当前没有可管理作业的课程。" ariaLabel="教师课程作业入口列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherCourseAssignmentsPage
