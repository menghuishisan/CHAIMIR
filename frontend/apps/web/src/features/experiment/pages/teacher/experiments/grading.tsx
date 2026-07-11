// 教师实验报告批改页：读取后端报告列表并提交人工评分。

import React, { useMemo, useState } from 'react'
import type { ReportDTO } from '@chaimir/api-client'
import { ExperimentReportStatus } from '@chaimir/api-client'
import { Button, Input, Table, Textarea } from '@chaimir/ui'
import { FileCheck, Save } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../experiment.module.css'
import { formatDateTime } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherExperimentGradingPage: React.FC = () => {
  const { id } = useParams()
  const [selectedId, setSelectedId] = useState('')
  const [score, setScore] = useState(0)
  const [comment, setComment] = useState('')
  const [message, setMessage] = useState('')
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
  const selected = useMemo(() => rows.find((row) => row.id === selectedId) ?? rows[0], [rows, selectedId])

  /**
   * grade 提交当前选中报告的人工评分，并刷新服务端列表。
   */
  const grade = async () => {
    const target = selectedId || selected?.id
    if (!target) return
    setMessage('')
    try {
      await api.experiment.gradeReport(target, { manual_score: score, comment })
      setMessage('评分已保存。')
      resource.reload()
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法保存评分。'))
    }
  }

  if (resource.status === 'loading') {
    return <LoadingState title="正在读取实验报告" description="系统正在同步学生提交记录。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 批改中心 / 批阅实验报告</div>

      <div className={styles.header}>
        <h1 className={styles.title}>
          <FileCheck className={styles.titleIcon} size={28} />
          批阅实验报告
        </h1>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <div className={styles.reportGrid}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>提交记录</h2>
          <Table<ReportDTO>
            rows={rows}
            rowKey="id"
            ariaLabel="实验报告"
            emptyTitle="暂无报告"
            emptyDescription="学生提交实验报告后会显示在这里。"
            columns={[
              { key: 'student', title: '学生', dataIndex: 'student_id', priority: 'primary' },
              { key: 'content', title: '内容引用', dataIndex: 'content_ref', priority: 'secondary' },
              { key: 'score', title: '人工评分', render: (row) => row.manual_score || '未评分' },
              { key: 'status', title: '状态', render: (row) => row.status === ExperimentReportStatus.GRADED ? '已批改' : '待批改' },
              {
                key: 'action',
                title: '操作',
                render: (row) => (
                  <Button size="sm" variant="outline" onClick={() => {
                    setSelectedId(row.id)
                    setScore(row.manual_score || 0)
                    setComment(row.comment ?? '')
                  }}>
                    选择
                  </Button>
                ),
              },
            ]}
          />
        </section>

        <aside className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>评分面板</h2>
          {selected ? (
            <>
              <div className={`${styles.panel} ${styles.reportViewer}`}>
                <p className={styles.muted}>报告内容引用</p>
                <strong>{selected.content_ref}</strong>
                <p className={styles.muted}>实例 {selected.instance_id}</p>
                <p className={styles.muted}>提交时间 {formatDateTime(selected.submitted_at)}</p>
              </div>
              <div className={styles.field}>
                <label className={styles.label} htmlFor="manual-score">人工评分</label>
                <Input id="manual-score" type="number" value={score} onChange={(event) => setScore(Number(event.target.value))} fullWidth />
              </div>
              <div className={styles.field}>
                <label className={styles.label} htmlFor="grade-comment">教师评语</label>
                <Textarea id="grade-comment" rows={6} value={comment} onChange={(event) => setComment(event.target.value)} fullWidth />
              </div>
              <Button icon={<Save size={16} />} onClick={grade}>保存评分</Button>
            </>
          ) : (
            <p className={styles.muted}>请选择一份报告进行批改。</p>
          )}
        </aside>
      </div>
    </div>
  )
}

export default TeacherExperimentGradingPage
