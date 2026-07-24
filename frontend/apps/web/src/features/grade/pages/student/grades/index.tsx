// GradesPage 展示当前学生成绩汇总，并通过 grade 后端生成成绩单。

import React, { useMemo, useState } from 'react'
import type { CourseGrade, GradeSummary } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Select, Table, ResourceState } from '@chaimir/ui'
import { GraduationCap, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { StudentGradeActions } from './StudentGradeActions'

interface StudentGradeState {
  studentId: string
  semesters: { id: string; name: string }[]
  courses: { id: string; name: string }[]
  summary: GradeSummary
  gpaHistory: GradeSummary[]
}

const GradesPage: React.FC = () => {
  const [semesterId, setSemesterId] = useState('')
  const resource = useAsyncResource(async () => {
    const [me, semesters] = await Promise.all([
      api.identity.getMe(),
      api.grade.listSemesters(),
    ])
    const [summary, gpaHistory, courses] = await Promise.all([
      api.grade.studentGrades(me.account.id, semesterId || undefined),
      api.grade.studentGPA(me.account.id),
      api.teaching.getCourses({ role: 'student', page: 1, size: 100 }),
    ])
    return {
      studentId: me.account.id,
      semesters: semesters.map((semester) => ({ id: semester.id, name: semester.name })),
      courses: courses.list,
      summary,
      gpaHistory,
    }
  }, [semesterId])

  const semesterOptions = useMemo(() => [
    { value: '', label: '全部学期' },
    ...((resource.data as StudentGradeState | undefined)?.semesters || []).map((semester) => ({ value: semester.id, label: semester.name })),
  ], [resource.data])

  const columns = useMemo<TableColumn<CourseGrade>[]>(() => [
    { key: 'course', title: '课程', render: (row) => (resource.data as StudentGradeState | undefined)?.courses.find((course) => course.id === row.course_id)?.name || '课程', priority: 'primary' },
    { key: 'score', title: '最终成绩', render: (row) => row.final_total.toFixed(1) },
    { key: 'credits', title: '获得学分', render: (row) => row.credits.toFixed(1) },
  ], [resource.data])

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
        </div>
      </div>

      <div className={styles.toolbar}>
        <Select value={semesterId} options={semesterOptions} onChange={setSemesterId} />
      </div>

      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取成绩" />}
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
          <section className={styles.panel}>
            <h2>历史绩点</h2>
            <Table
              columns={[
                { key: 'semester', title: '学期', render: (row: GradeSummary) => (resource.data as StudentGradeState | undefined)?.semesters.find((semester) => semester.id === row.semester_id)?.name || '未指定', priority: 'primary' },
                { key: 'gpa', title: '学期 GPA', render: (row: GradeSummary) => row.gpa.toFixed(2) },
                { key: 'credits', title: '学分', render: (row: GradeSummary) => row.total_credits.toFixed(1) },
              ]}
              rows={resource.data?.gpaHistory || []}
              rowKey={(row) => row.semester_id || row.computed_at}
              emptyTitle="暂无历史绩点"
              emptyDescription="当前还没有已落库的学期绩点。"
              ariaLabel="历史绩点列表"
            />
          </section>
          <StudentGradeActions studentId={resource.data?.studentId || ''} semesterId={semesterId} />
        </>
      )}
    </div>
  )
}

export default GradesPage
