// GradesPage 展示当前学生成绩汇总，并通过 grade 后端生成成绩单。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, CourseGrade, GradeSummary } from '@chaimir/api-client'
import { TranscriptScope } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Select, Table } from '@chaimir/ui'
import { FileText, GraduationCap, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'

interface StudentGradeState {
  studentId: string
  semesters: { id: string; name: string }[]
  summary: GradeSummary
}

const GradesPage: React.FC = () => {
  const [semesterId, setSemesterId] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(async () => {
    const [me, semesters] = await Promise.all([
      api.identity.getMe(),
      api.grade.listSemesters(),
    ])
    const summary = await api.grade.studentGrades(me.account.id, semesterId || undefined)
    return {
      studentId: me.account.id,
      semesters: semesters.map((semester) => ({ id: semester.id, name: semester.name })),
      summary,
    }
  }, [semesterId])

  const semesterOptions = useMemo(() => [
    { value: '', label: '全部学期' },
    ...((resource.data as StudentGradeState | undefined)?.semesters || []).map((semester) => ({ value: semester.id, label: semester.name })),
  ], [resource.data])

  /**
   * generateTranscript 按当前筛选范围向后端申请成绩单生成。
   */
  const generateTranscript = useCallback(async () => {
    if (!resource.data) {
      return
    }
    setError(null)
    setMessage(null)
    try {
      await api.grade.generateTranscript({
        student_id: resource.data.studentId,
        scope: semesterId ? TranscriptScope.SEMESTER : TranscriptScope.FULL,
        semester_id: semesterId || undefined,
      })
      setMessage('成绩单已生成，可在成绩档案中查看。')
    } catch (actionError) {
      setError((actionError as ApiError).message || '成绩单生成失败，请稍后重试。')
    }
  }, [resource.data, semesterId])

  const columns = useMemo<TableColumn<CourseGrade>[]>(() => [
    { key: 'course', title: '课程编号', dataIndex: 'course_id', priority: 'primary' },
    { key: 'score', title: '最终成绩', render: (row) => row.final_total.toFixed(1) },
    { key: 'credits', title: '获得学分', render: (row) => row.credits.toFixed(1) },
  ], [])

  const summary = resource.data?.summary
  const rows = summary?.course_grades || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><GraduationCap size={28} />成绩中心</h1>
          <p className={styles.subtitle}>查看已发布的课程成绩、学分和绩点汇总。</p>
        </div>
        <div className={styles.actions}>
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
          <Button icon={<FileText size={16} />} onClick={generateTranscript} disabled={!summary}>生成成绩单</Button>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="生成成功">{message}</Callout>}

      <div className={styles.toolbar}>
        <Select value={semesterId} options={semesterOptions} onChange={setSemesterId} />
      </div>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取成绩" />}
      {(resource.status === 'success' || resource.status === 'empty') && summary && (
        <>
          <section className={styles.summary} aria-label="成绩概览">
            <div className={styles.metric}><span>学期 GPA</span><strong>{summary.gpa.toFixed(2)}</strong></div>
            <div className={styles.metric}><span>累计 GPA</span><strong>{summary.cumulative_gpa.toFixed(2)}</strong></div>
            <div className={styles.metric}><span>已获学分</span><strong>{summary.total_credits.toFixed(1)}</strong></div>
          </section>
          <div className={styles.tableWrap}>
            <Table columns={columns} rows={rows} rowKey="course_id" emptyTitle="暂无成绩" emptyDescription="当前筛选范围内还没有已发布成绩。" ariaLabel="学生成绩列表" />
          </div>
        </>
      )}
    </div>
  )
}

export default GradesPage
