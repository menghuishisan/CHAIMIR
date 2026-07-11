// PlatformSimulationsPage 审核仿真包预览报告，并调用 M4 审核接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { SimPackageReview, SimReviewResult } from '@chaimir/api-client'
import { SIM_PACKAGE_STATUS, SIM_REVIEW_RESULT } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table } from '@chaimir/ui'
import { Archive, CheckCircle, RefreshCw, RotateCcw, Shield, XCircle } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../sim.module.css'
import { simReviewResultOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const PlatformSimulationsPage: React.FC = () => {
  const [result, setResult] = useState('')
  const [rejectComment, setRejectComment] = useState('请修正仿真包后重新提交。')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => api.sim.getReviews({
    result: (result || undefined) as SimReviewResult | undefined,
    page: 1,
    size: 20,
  }), [result])

  /**
   * reviewAction 执行仿真包审核通过或退回。
   */
  const reviewAction = useCallback(async (action: () => Promise<unknown>, successMessage: string) => {
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(successMessage)
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '审核操作失败，请稍后重试。'))
    }
  }, [resource])

  const columns = useMemo<TableColumn<SimPackageReview>[]>(() => [
    { key: 'package', title: '仿真包', render: (row) => row.package?.name || row.package_id, priority: 'primary' },
    { key: 'submitter', title: '提交人', dataIndex: 'submitter_id' },
    { key: 'result', title: '审核结果', render: (row) => <span className={styles.status}>{row.result}</span> },
    { key: 'scan', title: '静态扫描', render: (row) => row.preview_report.static_scan?.status || '未返回' },
    { key: 'preview', title: 'Worker 预览', render: (row) => row.preview_report.worker_preview?.status || '未返回' },
    {
      key: 'actions',
      title: '操作',
      render: (row) => {
        const reviewPending = row.result === SIM_REVIEW_RESULT.PENDING && row.package?.status === SIM_PACKAGE_STATUS.REVIEWING
        const canArchive = row.package?.status === SIM_PACKAGE_STATUS.PUBLISHED
        const canRepublish = row.package?.status === SIM_PACKAGE_STATUS.ARCHIVED
        return (
          <div className={styles.actions}>
            {reviewPending && <Button variant="outline" size="sm" icon={<CheckCircle size={14} />} onClick={() => reviewAction(() => api.sim.approveReview(row.id), '仿真包已通过审核。')}>通过</Button>}
            {reviewPending && <Button variant="ghost" size="sm" icon={<XCircle size={14} />} onClick={() => reviewAction(() => api.sim.rejectReview(row.id, rejectComment), '仿真包已退回。')}>退回</Button>}
            {canArchive && <Button variant="ghost" size="sm" icon={<Archive size={14} />} onClick={() => reviewAction(() => api.sim.archivePackage(row.package_id), '仿真包已下架。')}>下架</Button>}
            {canRepublish && <Button variant="ghost" size="sm" icon={<RotateCcw size={14} />} onClick={() => reviewAction(() => api.sim.republishPackage(row.package_id), '仿真包已重新上架。')}>重新上架</Button>}
            {!reviewPending && !canArchive && !canRepublish && <span className={styles.status}>暂无可用操作</span>}
          </div>
        )
      },
    },
  ], [rejectComment, reviewAction])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Shield size={28} />大型仿真靶机审核与发布</h1>
          <p className={styles.subtitle}>基于后端预览报告审核仿真包。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}
      <div className={styles.toolbar}>
        <Select value={result} options={simReviewResultOptions} onChange={setResult} />
        <Input value={rejectComment} onChange={(event) => setRejectComment(event.target.value)} />
      </div>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取审核列表" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无审核" emptyDescription="当前没有仿真包审核记录。" ariaLabel="仿真包审核列表" />
        </div>
      )}
    </div>
  )
}

export default PlatformSimulationsPage
