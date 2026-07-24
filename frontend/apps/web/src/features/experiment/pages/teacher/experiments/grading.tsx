// 教师实验报告页：读取后端提交记录，不伪造报告内容或评分入口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ReportDTO } from '@chaimir/api-client'
import { ExperimentReportStatus } from '@chaimir/api-client'
import { Button, Callout, Input, Table, Textarea, ResourceState, FormField } from '@chaimir/ui'
import { Check, FileCheck, RefreshCw } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { usePendingAction } from '../../../../../hooks'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../experiment.module.css'
import { formatDateTime } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherExperimentGradingPage: React.FC = () => {
  const { id } = useParams()
  const resource = useAsyncResource(
    async () => {
      if (!id) {
        throw new Error('缺少实验编号，无法读取报告。')
      }
      return api.experiment.listReports(id, { page: 1, size: 30 })
    },
    [id]
  )
  const rows = useMemo(() => resource.data?.list ?? [], [resource.data?.list])
  const [selectedReport, setSelectedReport] = useState<ReportDTO | null>(null)
  const [score, setScore] = useState('')
  const [comment, setComment] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const { pendingAction, runPendingAction } = usePendingAction()

  const selectReport = useCallback((report: ReportDTO) => {
    setSelectedReport(report)
    setScore(report.manual_score > 0 ? String(report.manual_score) : '')
    setComment(report.comment ?? '')
    setMessage(null)
    setError(null)
  }, [])

  const saveGrade = useCallback(async () => {
    if (!selectedReport) return
    const manualScore = Number(score)
    if (!Number.isFinite(manualScore) || manualScore < 0 || manualScore > 100) {
      setError('请输入 0 到 100 之间的分数。')
      return
    }
    setError(null)
    setMessage(null)
    try {
      await api.experiment.gradeReport(selectedReport.id, { manual_score: manualScore, comment: comment.trim() })
      setMessage('报告评分已保存。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '报告评分保存失败，请稍后重试。'))
    }
  }, [comment, resource, score, selectedReport])

  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在读取实验报告" description="系统正在同步学生提交记录。" />
  }

  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 批改中心 / 批阅实验报告</div>

      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <FileCheck className={styles.titleIcon} size={28} />
            实验报告提交
          </h1>
          <p className={styles.subtitle}>查看提交状态并保存人工评分。报告正文由服务端授权后提供。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <Callout variant="danger" title="评分未保存">{error}</Callout>}
      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      <section className={`${styles.panel} ${styles.section}`}>
        <h2 className={styles.sectionTitle}>提交记录</h2>
        <Table<ReportDTO>
          rows={rows}
          rowKey="id"
          ariaLabel="实验报告提交记录"
          emptyTitle="暂无报告"
          emptyDescription="学生提交实验报告后会显示在这里。"
          columns={[
            { key: 'report', title: '报告', render: () => '实验报告', priority: 'primary' },
            { key: 'submitted', title: '提交时间', render: (row) => formatDateTime(row.submitted_at), priority: 'secondary' },
            { key: 'score', title: '人工评分', render: (row) => row.status === ExperimentReportStatus.GRADED ? String(row.manual_score) : '未评分' },
            { key: 'status', title: '状态', render: (row) => row.status === ExperimentReportStatus.GRADED ? '已批改' : '待批改' },
            { key: 'actions', title: '操作', render: (row) => <Button variant="outline" size="sm" icon={<Check size={14} />} onClick={() => selectReport(row)}>批改</Button> },
          ]}
        />
        <p className={styles.muted}>当前接口只返回提交状态、评分和对象引用，不在页面展示内部对象路径。</p>
      </section>
      {selectedReport && (
        <section className={`${styles.panel} ${styles.section}`} aria-labelledby="report-grading-title">
          <h2 id="report-grading-title" className={styles.sectionTitle}>批改报告</h2>
          <p className={styles.muted}>提交时间：{formatDateTime(selectedReport.submitted_at)}。报告正文需服务端提供授权内容后才能查看。</p>
          <div className={styles.formGrid}>
            <FormField className={styles.field} label="人工评分"><Input type="number" min={0} max={100} value={score} onChange={(event) => setScore(event.target.value)} /></FormField>
            <FormField className={styles.field} label="评语"><Textarea value={comment} onChange={(event) => setComment(event.target.value)} /></FormField>
          </div>
          <div className={styles.actions}>
            <Button icon={<Check size={16} />} loading={pendingAction === 'grade'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('grade', saveGrade)}>保存评分</Button>
            <Button variant="ghost" disabled={Boolean(pendingAction)} onClick={() => setSelectedReport(null)}>取消</Button>
          </div>
        </section>
      )}
    </div>
  )
}

export default TeacherExperimentGradingPage
