// 教师实验报告页：读取后端提交记录，不伪造报告内容或评分入口。

import React, { useCallback, useMemo, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import type { ReportDTO } from '@chaimir/api-client'
import { ExperimentReportStatus } from '@chaimir/api-client'
import { Button, Callout, Input, Table, Textarea, ResourceState, FormField } from '@chaimir/ui'
import { Check, Download, FileCheck, RefreshCw } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { usePendingAction } from '../../../../../hooks'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../experiment.module.css'
import { formatDateTime } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { saveBlob } from '../../../../../utils/download'

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
  const [reportContent, setReportContent] = useState('')
  const [reportDownload, setReportDownload] = useState<{ blob: Blob; fileName: string } | null>(null)
  const [reportLoading, setReportLoading] = useState(false)
  const { pendingAction, runPendingAction } = usePendingAction()

  const selectReport = useCallback(async (report: ReportDTO) => {
    setSelectedReport(report)
    setScore(report.manual_score > 0 ? String(report.manual_score) : '')
    setComment(report.comment ?? '')
    setMessage(null)
    setError(null)
    setReportContent('')
    setReportDownload(null)
    setReportLoading(true)
    try {
      const downloaded = await api.experiment.downloadReport(report.id)
      setReportDownload(downloaded)
      setReportContent(await downloaded.blob.text())
    } catch (loadError) {
      setError(userFacingErrorMessage(loadError, '实验报告读取失败，请稍后重试。'))
    } finally {
      setReportLoading(false)
    }
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
          <p className={styles.subtitle}>查看学生提交的 Markdown 报告并保存人工评分。</p>
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
            { key: 'student', title: '学生', render: (row) => row.student_name || row.student_no || '学生信息未提供', priority: 'primary' },
            { key: 'report', title: '报告', dataIndex: 'file_name' },
            { key: 'submitted', title: '提交时间', render: (row) => formatDateTime(row.submitted_at), priority: 'secondary' },
            { key: 'score', title: '人工评分', render: (row) => row.status === ExperimentReportStatus.GRADED ? String(row.manual_score) : '未评分' },
            { key: 'status', title: '状态', render: (row) => row.status === ExperimentReportStatus.GRADED ? '已批改' : '待批改' },
            { key: 'actions', title: '操作', render: (row) => <Button variant="outline" size="sm" icon={<Check size={14} />} onClick={() => void selectReport(row)}>批改</Button> },
          ]}
        />
      </section>
      {selectedReport && (
        <section className={`${styles.panel} ${styles.section}`} aria-labelledby="report-grading-title">
          <h2 id="report-grading-title" className={styles.sectionTitle}>批改报告</h2>
          <p className={styles.muted}>{selectedReport.student_name || selectedReport.student_no} · {selectedReport.file_name} · 提交于 {formatDateTime(selectedReport.submitted_at)}</p>
          <div className={styles.layout}>
            <div className={styles.reportPreview} aria-label="实验报告正文">
              {reportLoading ? <ResourceState status="loading" title="正在读取实验报告" /> : <ReactMarkdown skipHtml disallowedElements={['img']}>{reportContent}</ReactMarkdown>}
            </div>
            <div className={styles.section}>
              <FormField className={styles.field} label="人工评分"><Input type="number" min={0} max={100} value={score} onChange={(event) => setScore(event.target.value)} /></FormField>
              <FormField className={styles.field} label="评语"><Textarea value={comment} onChange={(event) => setComment(event.target.value)} /></FormField>
            </div>
          </div>
          <div className={styles.actions}>
            <Button icon={<Check size={16} />} loading={pendingAction === 'grade'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('grade', saveGrade)}>保存评分</Button>
            <Button variant="outline" icon={<Download size={16} />} disabled={!reportDownload || Boolean(pendingAction)} onClick={() => reportDownload && saveBlob(reportDownload.blob, reportDownload.fileName)}>下载原文件</Button>
            <Button variant="ghost" disabled={Boolean(pendingAction)} onClick={() => setSelectedReport(null)}>取消</Button>
          </div>
        </section>
      )}
    </div>
  )
}

export default TeacherExperimentGradingPage
