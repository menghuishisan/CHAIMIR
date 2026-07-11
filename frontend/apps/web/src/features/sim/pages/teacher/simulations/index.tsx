// TeacherSimulationsPage 展示教师仿真包，并提交新的仿真包审核。

import React, { useCallback, useMemo, useState } from 'react'
import type { SimCompute, SimPackageMeta } from '@chaimir/api-client'
import { SIM_COMPUTE } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table, Textarea } from '@chaimir/ui'
import { Eye, Network, Pencil, RefreshCw, Upload } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../sim.module.css'
import { formatDateTime, parseJsonObject, simComputeOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherSimulationsPage: React.FC = () => {
  const resource = useAsyncResource(() => api.sim.getPackages({ page: 1, size: 20 }), [])
  const [file, setFile] = useState<File | null>(null)
  const [code, setCode] = useState('')
  const [version, setVersion] = useState('v1')
  const [name, setName] = useState('')
  const [category, setCategory] = useState('')
  const [compute, setCompute] = useState<SimCompute>(SIM_COMPUTE.FRONTEND)
  const [backendAdapter, setBackendAdapter] = useState('')
  const [backendConfig, setBackendConfig] = useState('{}')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [editingPackage, setEditingPackage] = useState<SimPackageMeta | null>(null)

  /**
   * submitPackage 把仿真 bundle 交给后端校验和审核流程。
   */
  const submitPackage = useCallback(async () => {
    if (!file) {
      setError('请选择仿真包文件后再提交。')
      return
    }
    if (compute === SIM_COMPUTE.BACKEND && !backendAdapter.trim()) {
      setError('后端运行方式需要填写平台已注册的计算适配器编号。')
      return
    }
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      const payload = {
        bundle: file,
        code,
        version,
        name,
        category,
        compute,
        scale_limit: {},
        backend_adapter: compute === SIM_COMPUTE.BACKEND ? backendAdapter.trim() : undefined,
        backend_config: compute === SIM_COMPUTE.BACKEND ? parseJsonObject(backendConfig) : {},
      }
      if (editingPackage) await api.sim.updatePackage(editingPackage.id, payload)
      else await api.sim.submitPackage(payload)
      setMessage(editingPackage ? '仿真包更新已提交审核。' : '仿真包已提交审核。')
      setEditingPackage(null)
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '仿真包提交失败，请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [backendAdapter, backendConfig, category, code, compute, editingPackage, file, name, resource, version])

  /** editPackage 把已提交包载入表单，更新时仍要求重新选择完整 bundle。 */
  const editPackage = useCallback((item: SimPackageMeta) => {
    setEditingPackage(item)
    setCode(item.code)
    setVersion(item.version)
    setName(item.name)
    setCategory(item.category)
    setCompute(item.compute)
    setBackendAdapter(item.backend_adapter || '')
    setBackendConfig('{}')
    setFile(null)
    setMessage('请重新选择完整仿真包文件后提交更新。')
  }, [])

  /** previewPackage 读取后端校验报告并展示关键结论。 */
  const previewPackage = useCallback(async (item: SimPackageMeta) => {
    setError(null)
    try {
      const review = await api.sim.previewPackage(item.id)
      setMessage(`预览结果：元数据 ${review.preview_report.metadata_validation?.status || '待检查'}，确定性 ${review.preview_report.determinism_check?.status || '待检查'}。`)
    } catch (previewError) {
      setError(userFacingErrorMessage(previewError, '预览报告读取失败，请稍后重试。'))
    }
  }, [])

  const columns = useMemo<TableColumn<SimPackageMeta>[]>(() => [
    { key: 'name', title: '仿真包', dataIndex: 'name', priority: 'primary' },
    { key: 'code', title: '编号', dataIndex: 'code' },
    { key: 'version', title: '版本', dataIndex: 'version' },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{row.status}</span> },
    { key: 'updated', title: '更新时间', render: (row) => formatDateTime(row.updated_at) },
    { key: 'actions', title: '操作', render: (row) => <div className={styles.actions}><Button variant="outline" size="sm" icon={<Pencil size={14} />} onClick={() => editPackage(row)}>更新</Button><Button variant="ghost" size="sm" icon={<Eye size={14} />} onClick={() => void previewPackage(row)}>预览报告</Button></div> },
  ], [editPackage, previewPackage])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Network size={28} />仿真包提交流程</h1>
          <p className={styles.subtitle}>上传仿真包进入平台审核与发布流程。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="提交成功">{message}</Callout>}
      <section className={styles.panel}>
        <div className={styles.formGrid}>
          <label className={styles.field}>包编号<Input fullWidth value={code} onChange={(event) => setCode(event.target.value)} /></label>
          <label className={styles.field}>版本<Input fullWidth value={version} onChange={(event) => setVersion(event.target.value)} /></label>
          <label className={styles.field}>名称<Input fullWidth value={name} onChange={(event) => setName(event.target.value)} /></label>
          <label className={styles.field}>分类<Input fullWidth value={category} onChange={(event) => setCategory(event.target.value)} /></label>
          <label className={styles.field}>运行方式<Select fullWidth value={compute} options={simComputeOptions} onChange={(value) => setCompute(value as SimCompute)} /></label>
          {compute === SIM_COMPUTE.BACKEND && <label className={styles.field}>计算适配器编号<Input fullWidth value={backendAdapter} onChange={(event) => setBackendAdapter(event.target.value)} /></label>}
          <label className={styles.field}>Bundle 文件<input type="file" onChange={(event) => setFile(event.target.files?.[0] || null)} /></label>
        </div>
        {compute === SIM_COMPUTE.BACKEND && <label className={styles.field}>后端计算配置<Textarea fullWidth rows={5} value={backendConfig} onChange={(event) => setBackendConfig(event.target.value)} /></label>}
        <Button icon={<Upload size={16} />} loading={submitting} onClick={submitPackage}>{editingPackage ? '提交更新' : '提交审核'}</Button>
      </section>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取仿真包" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无仿真包" emptyDescription="当前还没有仿真包记录。" ariaLabel="教师仿真包列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherSimulationsPage
