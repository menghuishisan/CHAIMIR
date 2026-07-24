// CourseGradebookPanel 接入课程进度、成绩权重、计算、调整和导出闭环。

import React, { useEffect, useState } from 'react'
import type { GradeWeightInput, TeachingCourseGrade } from '@chaimir/api-client'
import { GradeSource } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Table, ResourceState, FormField } from '@chaimir/ui'
import { Calculator, Download, Plus, Save, Trash2 } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import styles from '../../teaching.module.css'

/** CourseGradebookPanel 管理指定课程的服务端成绩册。 */
export function CourseGradebookPanel({ courseId }: { courseId: string }): React.ReactElement {
  const resource = useAsyncResource(async () => {
    if (!courseId) return null
    const [stats, weights, grades] = await Promise.all([
      api.teaching.getProgressStats(courseId),
      api.teaching.listGradeWeights(courseId),
      api.teaching.listGrades(courseId, { page: 1, size: 100 }),
    ])
    const [assignments, experiments, members] = await Promise.all([
      api.teaching.listAssignments(courseId),
      api.experiment.getExperiments({ course_id: courseId, page: 1, size: 100 }),
      api.teaching.listMembers(courseId, { page: 1, size: 100 }),
    ])
    return { stats, weights, grades: grades.list, assignments, experiments: experiments.list, members: members.list }
  }, [courseId], () => false)
  const [weights, setWeights] = useState<GradeWeightInput[]>([])
  const [sourceType, setSourceType] = useState(String(GradeSource.ASSIGNMENT))
  const [sourceRef, setSourceRef] = useState('')
  const [weight, setWeight] = useState('0')
  const [studentId, setStudentId] = useState('')
  const [overrideTotal, setOverrideTotal] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (resource.data) setWeights(resource.data.weights.map((item) => ({ source_type: item.source_type, source_ref: item.source_ref, weight: item.weight })))
  }, [resource.data])

  /** runAction 执行成绩册动作并刷新服务端权威数据。 */
  const runAction = async (action: () => Promise<unknown>, success: string) => {
    setError('')
    setMessage('')
    try {
      await action()
      setMessage(success)
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '成绩册操作失败，请检查输入后重试。'))
    }
  }

  /** addWeight 在本地编辑列表中加入一个成绩来源。 */
  const addWeight = () => {
    if (!sourceRef.trim() || !Number(weight)) return
    setWeights((current) => [...current, { source_type: Number(sourceType) as GradeSource, source_ref: sourceRef.trim(), weight: Number(weight) }])
    setSourceRef('')
    setWeight('0')
  }

  if (!courseId) return <Callout variant="info" title="填写课程编号">填写课程编号后可维护课程成绩册。</Callout>
  if (resource.status === 'loading') return <ResourceState status="loading" title="正在获取课程成绩册" />
  if (resource.status === 'error') return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />

  const grades = resource.data?.grades || []
  const sourceOptions = sourceType === String(GradeSource.ASSIGNMENT)
    ? (resource.data?.assignments || []).map((item) => ({ value: item.id, label: item.title }))
    : (resource.data?.experiments || []).map((item) => ({ value: item.id, label: item.name }))
  const sourceNames = new Map([
    ...(resource.data?.assignments || []).map((item) => [`${GradeSource.ASSIGNMENT}:${item.id}`, item.title] as const),
    ...(resource.data?.experiments || []).map((item) => [`${GradeSource.EXPERIMENT}:${item.id}`, item.name] as const),
  ])
  const memberOptions = (resource.data?.members || []).map((member) => ({ value: member.student_id, label: member.student_no ? `${member.student_name}（${member.student_no}）` : member.student_name }))
  return (
    <section className={styles.panel}>
      <h2>课程成绩册</h2>
      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
      {resource.data?.stats && (
        <div className={styles.actions}>
          <span className={styles.status}>成员 {resource.data.stats.member_count}</span>
          <span className={styles.status}>课时 {resource.data.stats.lesson_count}</span>
          <span className={styles.status}>已完成 {resource.data.stats.completed_count}</span>
        </div>
      )}
      <div className={styles.formGrid}>
        <FormField className={styles.field} label="成绩来源" htmlFor="grade-source-type"><Select id="grade-source-type" fullWidth value={sourceType} onChange={(value) => { setSourceType(value); setSourceRef('') }} options={[{ value: '1', label: '作业' }, { value: '2', label: '实验' }]} /></FormField>
        <FormField className={styles.field} label="来源项目" htmlFor="grade-source-ref"><Select id="grade-source-ref" fullWidth value={sourceRef} onChange={setSourceRef} options={sourceOptions} placeholder="选择当前课程项目" /></FormField>
        <FormField className={styles.field} label="权重" htmlFor="grade-source-weight"><Input id="grade-source-weight" fullWidth type="number" min="1" max="100" value={weight} onChange={(event) => setWeight(event.target.value)} /></FormField>
        <Button variant="outline" icon={<Plus size={14} />} onClick={addWeight}>添加权重</Button>
      </div>
      {weights.map((item, index) => (
        <div className={styles.actions} key={`${item.source_type}-${item.source_ref}-${index}`}>
          <span>{sourceNames.get(`${item.source_type}:${item.source_ref}`)} · {item.weight.toFixed(0)}%</span>
          <Button variant="ghost" size="sm" icon={<Trash2 size={13} />} aria-label="移除权重" onClick={() => setWeights((current) => current.filter((_, currentIndex) => currentIndex !== index))} />
        </div>
      ))}
      <div className={styles.actions}>
        <Button variant="outline" icon={<Save size={14} />} onClick={() => void runAction(() => api.teaching.setGradeWeights(courseId, { items: weights }), '成绩权重已保存。')}>保存权重</Button>
        <Button icon={<Calculator size={14} />} onClick={() => void runAction(() => api.teaching.computeGrades(courseId), '课程成绩已重新计算。')}>计算成绩</Button>
        <Button variant="outline" icon={<Download size={14} />} onClick={() => void runAction(() => api.teaching.exportGrades(courseId), '成绩导出任务已创建。')}>导出成绩</Button>
      </div>
      <Table<TeachingCourseGrade> rows={grades} rowKey={(row) => String(row.student_id)} ariaLabel="课程成绩列表" emptyTitle="暂无成绩" emptyDescription="配置权重并计算后显示课程成绩。" columns={[
        { key: 'student', title: '学生', render: (row) => row.student_name, priority: 'primary' },
        { key: 'studentNo', title: '学号', render: (row) => row.student_no || '未设置' },
        { key: 'auto', title: '自动总分', dataIndex: 'auto_total' },
        { key: 'final', title: '最终总分', dataIndex: 'final_total' },
        { key: 'adjusted', title: '人工调整', render: (row) => row.is_overridden ? '已调整' : '未调整' },
      ]} />
      <div className={styles.formGrid}>
        <FormField className={styles.field} label="学生" htmlFor="grade-student"><Select id="grade-student" fullWidth value={studentId} onChange={setStudentId} options={memberOptions} placeholder="选择课程成员" /></FormField>
        <FormField className={styles.field} label="调整后总分" htmlFor="grade-override-total"><Input id="grade-override-total" fullWidth type="number" min="0" max="100" value={overrideTotal} onChange={(event) => setOverrideTotal(event.target.value)} /></FormField>
        <Button disabled={!studentId || !overrideTotal} onClick={() => void runAction(() => api.teaching.overrideGrade(courseId, studentId, { total: Number(overrideTotal) }), '学生总评已调整。')}>保存调整</Button>
      </div>
    </section>
  )
}
