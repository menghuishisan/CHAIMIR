// RuntimesPage 展示平台链运行时列表，数据来自 sandbox 后端模块。

import React, { useMemo, useState } from 'react'
import type { SandboxRuntime, SandboxRuntimeRequest } from '@chaimir/api-client'
import { RuntimeStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table, Textarea } from '@chaimir/ui'
import { Eye, Pencil, Plus, RefreshCw, Server } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { runtimeSelftestStatusLabel, runtimeStatusLabel } from '../../../../../utils/index'
import { parseJsonObject } from '../../../../../utils'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

/**
 * RuntimesPage 读取链运行时声明和自检状态。
 */
const RuntimesPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(() => api.sandbox.listRuntimes(), [])
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState<SandboxRuntimeRequest>({ code: '', name: '', eco: '', adapter_level: 1, adapter_spec: {}, capability_impl: '', plugin_ref: '', status: RuntimeStatus.ONBOARDING })
  const [adapterSpec, setAdapterSpec] = useState('{}')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const rows = resource.data || []

  /** saveRuntime 创建或更新完整运行时适配声明。 */
  const saveRuntime = async () => {
    setError('')
    try {
      const payload = { ...form, adapter_spec: parseJsonObject(adapterSpec) }
      if (editingId) await api.sandbox.updateRuntime(editingId, payload)
      else await api.sandbox.registerRuntime(payload)
      setMessage(editingId ? '运行时已更新。' : '运行时已登记，请继续配置镜像并运行自检。')
      setEditingId('')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '运行时保存失败，请检查适配声明。'))
    }
  }

  /** editRuntime 把现有运行时声明载入编辑表单。 */
  const editRuntime = (runtime: SandboxRuntime) => {
    setEditingId(String(runtime.id))
    setForm(runtime)
    setAdapterSpec(JSON.stringify(runtime.adapter_spec, null, 2))
  }

  const columns = useMemo<TableColumn<SandboxRuntime>[]>(() => [
    { key: 'name', title: '运行时名称', dataIndex: 'name', priority: 'primary' },
    { key: 'code', title: '编码', dataIndex: 'code', priority: 'secondary' },
    { key: 'eco', title: '生态', dataIndex: 'eco' },
    {
      key: 'level',
      title: '适配等级',
      render: (row) => `L${row.adapter_level}`,
    },
    {
      key: 'status',
      title: '运行状态',
      render: (row) => <span className={styles.status}>{runtimeStatusLabel(row.status)}</span>,
    },
    {
      key: 'selftest',
      title: '自检状态',
      render: (row) => <span className={styles.muted}>{runtimeSelftestStatusLabel(row.selftest_status)}</span>,
    },
    {
      key: 'action',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}><Button size="sm" variant="outline" icon={<Eye size={14} />} onClick={() => navigate(`/platform-admin/runtimes/${row.id}`)}>查看详情</Button><Button size="sm" variant="ghost" icon={<Pencil size={14} />} onClick={() => editRuntime(row)}>编辑</Button></div>
      ),
    },
  ], [navigate])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Server className={styles.icon} size={28} />
            链运行时与镜像集
          </h1>
          <p className={styles.subtitle}>查看平台可用链运行时、适配等级和自检状态。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
      <section className={styles.tableWrap}>
        <div className={styles.formGrid}>
          <Input value={form.name} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} placeholder="运行时名称" fullWidth />
          <Input value={form.code} onChange={(event) => setForm((current) => ({ ...current, code: event.target.value }))} placeholder="运行时编号" fullWidth />
          <Input value={form.eco} onChange={(event) => setForm((current) => ({ ...current, eco: event.target.value }))} placeholder="链生态" fullWidth />
          <Input type="number" value={form.adapter_level} onChange={(event) => setForm((current) => ({ ...current, adapter_level: Number(event.target.value) }))} placeholder="适配等级" fullWidth />
          <Input value={form.capability_impl} onChange={(event) => setForm((current) => ({ ...current, capability_impl: event.target.value }))} placeholder="能力实现" fullWidth />
          <Input value={form.plugin_ref} onChange={(event) => setForm((current) => ({ ...current, plugin_ref: event.target.value }))} placeholder="插件地址" fullWidth />
          <Select value={String(form.status)} onChange={(value) => setForm((current) => ({ ...current, status: Number(value) as RuntimeStatus }))} options={[{ value: '1', label: '可用' }, { value: '2', label: '接入中' }, { value: '3', label: '停用' }]} />
          <Textarea rows={4} value={adapterSpec} onChange={(event) => setAdapterSpec(event.target.value)} placeholder="适配器声明" fullWidth />
          <Button icon={<Plus size={15} />} onClick={() => void saveRuntime()}>{editingId ? '保存运行时' : '登记运行时'}</Button>
        </div>
      </section>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取链运行时" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无运行时"
            emptyDescription="当前平台还没有登记链运行时。"
            ariaLabel="平台链运行时列表"
          />
        </div>
      )}
    </div>
  )
}

export default RuntimesPage
