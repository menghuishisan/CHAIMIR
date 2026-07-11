// TeacherExamsPage 展示内容中心试卷列表，并提供重新组卷入口。

import React, { useCallback, useMemo, useState } from 'react'
import type { Paper } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Table } from '@chaimir/ui'
import { File, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../content.module.css'
import { formatDateTime, paperModeLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'



const TeacherExamsPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(() => api.content.listPapers({ page: 1, size: 20 }), [])
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /**
   * handleRegenerate 调用后端重新组卷接口。
   */
  const handleRegenerate = useCallback(async (paper: Paper) => {
    setError(null)
    setMessage(null)
    try {
      await api.content.regeneratePaper(String(paper.id))
      setMessage('试卷已重新组卷。')
      resource.reload()
    } catch (regenerateError) {
      setError(userFacingErrorMessage(regenerateError, '重新组卷失败，请稍后重试。'))
    }
  }, [resource])

  const columns = useMemo<TableColumn<Paper>[]>(() => [
    { key: 'name', title: '试卷名称', dataIndex: 'name', priority: 'primary' },
    { key: 'mode', title: '组卷方式', render: (row) => <span className={styles.status}>{paperModeLabel(row.gen_mode)}</span> },
    {
      key: 'criteria',
      title: '抽题规则',
      render: (row) => row.gen_criteria.count ? `${row.gen_criteria.count} 题` : '手动选择',
    },
    {
      key: 'updatedAt',
      title: '更新时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.updated_at)}</span>,
    },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          <Button variant="outline" size="sm" onClick={() => handleRegenerate(row)}>
            重新组卷
          </Button>
          <Button variant="ghost" size="sm" onClick={() => navigate(`/teacher/exams/edit?id=${row.id}`)}>
            编辑规则
          </Button>
        </div>
      ),
    },
  ], [handleRegenerate, navigate])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <File size={28} />
            试卷库管理
          </h1>
          <p className={styles.subtitle}>试卷和组卷规则来自内容中心后端。</p>
        </div>
        <div className={styles.toolbar}>
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
          <Button onClick={() => navigate('/teacher/exams/edit')}>创建新试卷</Button>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取试卷列表" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey={(row) => String(row.id)} emptyTitle="暂无试卷" emptyDescription="当前还没有试卷记录。" ariaLabel="试卷列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherExamsPage
