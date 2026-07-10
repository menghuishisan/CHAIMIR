// RuntimeDetailPage 展示平台运行时、自检详情和镜像预拉取状态。

import React, { useCallback, useMemo, useState } from 'react'
import { ImagePrepullStatus, RuntimeImageStatus } from '@chaimir/api-client'
import type { SandboxRuntimeImage } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, DescriptionList, Table } from '@chaimir/ui'
import { ArrowLeft, HardDrive, Play, RefreshCw } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import {
  formatDateTime,
  formatMetricsSummary,
  imagePrepullStatusLabel,
  runtimeSelftestStatusLabel,
  runtimeStatusLabel,
} from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import listStyles from '../../../../admin/pages/list.module.css'
import styles from './detail.module.css'

/**
 * RuntimeDetailPage 读取运行时和镜像列表，并提供自检与镜像预拉取动作。
 */
const RuntimeDetailPage: React.FC = () => {
  const { id } = useParams()
  const navigate = useNavigate()
  const [runningSelftest, setRunningSelftest] = useState(false)
  const [prepullingImageId, setPrepullingImageId] = useState<number | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(async () => {
    if (!id) {
      throw new Error('缺少运行时编号，无法读取详情。')
    }
    const [runtimes, images] = await Promise.all([
      api.sandbox.listRuntimes(),
      api.sandbox.listRuntimeImages(id),
    ])
    const runtime = runtimes.find((item) => String(item.id) === id)
    if (!runtime) {
      throw new Error('未找到该运行时，请返回列表刷新后重试。')
    }
    return { runtime, images }
  }, [id])

  /**
   * handleSelftest 触发后端运行时自检，并刷新摘要状态。
   */
  const handleSelftest = useCallback(async () => {
    if (!id) return
    setRunningSelftest(true)
    setMessage(null)
    setError(null)
    try {
      await api.sandbox.runRuntimeSelftest(id)
      setMessage('运行时自检已完成，状态已刷新。')
      resource.reload()
    } catch (selftestError) {
      setError(userFacingErrorMessage(selftestError, '运行时自检失败，请稍后重试。'))
    } finally {
      setRunningSelftest(false)
    }
  }, [id, resource])

  /**
   * handlePrepull 对指定镜像触发后端预拉取，并刷新镜像状态。
   */
  const handlePrepull = useCallback(async (imageId: number) => {
    if (!id) return
    setPrepullingImageId(imageId)
    setMessage(null)
    setError(null)
    try {
      await api.sandbox.prepullRuntimeImage(id, String(imageId))
      setMessage('镜像预拉取已执行，状态已刷新。')
      resource.reload()
    } catch (prepullError) {
      setError(userFacingErrorMessage(prepullError, '镜像预拉取失败，请稍后重试。'))
    } finally {
      setPrepullingImageId(null)
    }
  }, [id, resource])

  const columns = useMemo<TableColumn<SandboxRuntimeImage>[]>(() => [
    { key: 'image', title: '镜像地址', dataIndex: 'image_url', priority: 'primary' },
    { key: 'version', title: '版本', dataIndex: 'version', priority: 'secondary' },
    { key: 'default', title: '默认镜像', render: (row) => (row.is_default ? '是' : '否') },
    { key: 'status', title: '状态', render: (row) => row.status === RuntimeImageStatus.AVAILABLE ? '可用' : '已停用' },
    { key: 'prepull', title: '预拉取', render: (row) => imagePrepullStatusLabel(row.prepull_status) },
    { key: 'time', title: '完成时间', render: (row) => formatDateTime(row.prepulled_at) },
    {
      key: 'action',
      title: '操作',
      render: (row) => (
        <Button
          size="sm"
          variant="outline"
          icon={<Play size={14} />}
          loading={prepullingImageId === row.id}
          disabled={row.status !== RuntimeImageStatus.AVAILABLE || row.prepull_status === ImagePrepullStatus.RUNNING}
          onClick={() => void handlePrepull(row.id)}
        >
          预拉取
        </Button>
      ),
    },
  ], [handlePrepull, prepullingImageId])

  const runtime = resource.data?.runtime
  const images = resource.data?.images || []

  return (
    <div className={listStyles.page}>
      <div className={listStyles.header}>
        <div>
          <h1 className={listStyles.title}>
            <HardDrive className={listStyles.icon} size={28} />
            运行时详情
          </h1>
          <p className={listStyles.subtitle}>查看接入状态、镜像版本和节点预拉取结果。</p>
        </div>
        <div className={listStyles.toolbar}>
          <Button variant="ghost" icon={<ArrowLeft size={16} />} onClick={() => navigate('/platform-admin/runtimes')}>返回列表</Button>
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
          <Button icon={<Play size={16} />} loading={runningSelftest} onClick={() => void handleSelftest()}>运行自检</Button>
        </div>
      </div>

      {message && <p className={styles.message} role="status">{message}</p>}
      {error && <p className={styles.error} role="alert">{error}</p>}
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取运行时详情" />}
      {runtime && (
        <>
          <section className={styles.summary}>
            <h2 className={styles.sectionTitle}>运行时摘要</h2>
            <DescriptionList
              columns={3}
              items={[
                { key: 'name', label: '名称', value: runtime.name, tone: 'strong' },
                { key: 'code', label: '编码', value: runtime.code },
                { key: 'eco', label: '生态', value: runtime.eco },
                { key: 'adapter', label: '适配等级', value: `L${runtime.adapter_level}` },
                { key: 'status', label: '运行状态', value: runtimeStatusLabel(runtime.status) },
                { key: 'selftest', label: '自检状态', value: runtimeSelftestStatusLabel(runtime.selftest_status) },
                { key: 'detail', label: '自检详情', value: formatMetricsSummary(runtime.selftest_detail || {}) },
              ]}
            />
          </section>

          <section className={listStyles.tableWrap}>
            <Table
              columns={columns}
              rows={images}
              rowKey="id"
              emptyTitle="暂无镜像版本"
              emptyDescription="该运行时还没有登记可用镜像。"
              ariaLabel="运行时镜像版本列表"
            />
          </section>
        </>
      )}
    </div>
  )
}

export default RuntimeDetailPage
