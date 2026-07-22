// TeacherCourseMembersPage 展示课程成员，并调用后端成员管理接口移除学生。

import React, { useCallback, useMemo, useState } from 'react'
import type { CourseMember } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Table } from '@chaimir/ui'
import { Plus, RefreshCw, Trash2, Users } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { formatDateTime, joinModeLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


const TeacherCourseMembersPage: React.FC = () => {
  const { id } = useParams()
  const resource = useAsyncResource(() => api.teaching.listMembers(String(id), { page: 1, size: 50 }), [id])
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [studentIds, setStudentIds] = useState('')

  /**
   * removeMember 移除课程成员。
   */
  const removeMember = useCallback(async (member: CourseMember) => {
    if (!id) return
    setError(null)
    setMessage(null)
    try {
      await api.teaching.removeMember(id, String(member.student_id))
      setMessage('成员已移除。')
      resource.reload()
    } catch (removeError) {
      setError(userFacingErrorMessage(removeError, '成员移除失败，请稍后重试。'))
    }
  }, [id, resource])

  /** addMembers 批量添加输入的学生编号并刷新名册。 */
  const addMembers = async () => {
    if (!id) return
    const ids = Array.from(new Set(studentIds.split(',').map((value) => value.trim()).filter((value) => /^[1-9]\d*$/.test(value))))
    if (!ids.length) {
      setError('请填写有效的学生编号。')
      return
    }
    setError(null)
    try {
      await api.teaching.addMembers(id, { student_ids: ids })
      setStudentIds('')
      setMessage('成员已添加。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '成员添加失败，请检查学生编号。'))
    }
  }

  const columns = useMemo<TableColumn<CourseMember>[]>(() => [
    { key: 'student', title: '学生编号', render: (row) => String(row.student_id), priority: 'primary' },
    { key: 'joinMode', title: '加入方式', render: (row) => joinModeLabel(row.join_mode) },
    { key: 'joinedAt', title: '加入时间', render: (row) => <span className={styles.muted}>{formatDateTime(row.joined_at)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <Button variant="outline" size="sm" icon={<Trash2 size={14} />} onClick={() => removeMember(row)}>
          移除
        </Button>
      ),
    },
  ], [removeMember])

  if (!id) return <EmptyState title="缺少课程编号" description="当前链接没有课程编号。" />

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Users size={28} />
            学生名册
          </h1>
          <p className={styles.subtitle}>查看课程成员，并维护学生选课关系。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}

      <section className={styles.panel}>
        <h2>添加学生</h2>
        <div className={styles.actions}>
          <Input value={studentIds} onChange={(event) => setStudentIds(event.target.value)} placeholder="多个学生编号用逗号分隔" />
          <Button icon={<Plus size={15} />} onClick={() => void addMembers()}>添加成员</Button>
        </div>
      </section>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取课程成员" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey={(row) => String(row.id)} emptyTitle="暂无成员" emptyDescription="当前课程还没有成员。" ariaLabel="课程成员列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherCourseMembersPage
