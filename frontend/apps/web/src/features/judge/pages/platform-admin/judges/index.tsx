// JudgesPage 展示平台判题器配置列表，数据来自 judge 后端模块。

import React, { useState } from 'react'
import type { Judger, JudgerRequest } from '@chaimir/api-client'
import { JudgerStatus, JudgerType } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Switch, Table, Textarea } from '@chaimir/ui'
import { Cpu, Play, RefreshCw, Save } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../judge.module.css'
import { formatSeconds, judgerStatusLabel, judgerTypeLabel } from '../../../../../utils/index'
import { parseJsonObject } from '../../../../../utils'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


/**
 * JudgesPage 读取判题器声明和执行器状态。
 */
const JudgesPage: React.FC = () => {
  const resource = useAsyncResource(() => api.judge.listJudgers(), [])
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState<JudgerRequest>({ code: '', name: '', type: JudgerType.TESTCASE, executor_ref: '', runtime_required: true, default_timeout_sec: 30, resource_spec: {}, status: JudgerStatus.AVAILABLE })
  const [resourceSpec, setResourceSpec] = useState('{}')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const rows = resource.data || []

  /** saveJudger 创建或更新完整判题器声明。 */
  const saveJudger = async () => {
    setError('')
    try {
      const payload = { ...form, resource_spec: parseJsonObject(resourceSpec) }
      if (editingId) await api.judge.updateJudger(editingId, payload)
      else await api.judge.createJudger(payload)
      setMessage(editingId ? '判题器已更新。' : '判题器已创建。')
      setEditingId('')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '判题器保存失败，请检查输入后重试。'))
    }
  }

  /** editJudger 把现有判题器声明载入表单。 */
  const editJudger = (judger: Judger) => {
    setEditingId(judger.id)
    setForm(judger)
    setResourceSpec(JSON.stringify(judger.resource_spec, null, 2))
  }

  /** selftestJudger 触发接入即测并刷新服务端状态。 */
  const selftestJudger = async (judger: Judger) => {
    setError('')
    try {
      await api.judge.runJudgerSelftest(judger.id)
      setMessage(`${judger.name}自测已完成。`)
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '判题器自测失败，请检查执行器配置。'))
    }
  }

  const columns: TableColumn<Judger>[] = [
    { key: 'name', title: '判题器名称', dataIndex: 'name', priority: 'primary' },
    { key: 'code', title: '编码', dataIndex: 'code', priority: 'secondary' },
    { key: 'type', title: '类型', render: (row) => judgerTypeLabel(row.type) },
    {
      key: 'runtime',
      title: '需要运行时',
      render: (row) => (row.runtime_required ? '需要' : '不需要'),
    },
    {
      key: 'timeout',
      title: '默认超时',
      render: (row) => formatSeconds(row.default_timeout_sec),
    },
    {
      key: 'status',
      title: '状态',
      render: (row) => <span className={styles.status}>{judgerStatusLabel(row.status)}</span>,
    },
    { key: 'actions', title: '操作', render: (row) => <div className={styles.actions}><Button variant="outline" size="sm" onClick={() => editJudger(row)}>编辑</Button><Button variant="ghost" size="sm" icon={<Play size={14} />} onClick={() => void selftestJudger(row)}>运行自测</Button></div> },
  ]

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Cpu size={28} />
            判题引擎集群
          </h1>
          <p className={styles.subtitle}>查看平台判题器配置、运行时依赖和默认执行约束。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
      <section className={styles.panel}>
        <h2>{editingId ? '编辑判题器' : '登记判题器'}</h2>
        <div className={styles.formGrid}>
          <label className={styles.field}>名称<Input fullWidth value={form.name} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} /></label>
          <label className={styles.field}>编号<Input fullWidth value={form.code} onChange={(event) => setForm((current) => ({ ...current, code: event.target.value }))} /></label>
          <label className={styles.field}>执行器地址<Input fullWidth value={form.executor_ref} onChange={(event) => setForm((current) => ({ ...current, executor_ref: event.target.value }))} /></label>
          <label className={styles.field}>判题类型<Select fullWidth value={String(form.type)} onChange={(value) => setForm((current) => ({ ...current, type: Number(value) as JudgerType }))} options={Object.values(JudgerType).filter((value): value is number => typeof value === 'number').map((value) => ({ value: String(value), label: judgerTypeLabel(value) }))} /></label>
          <label className={styles.field}>默认时限（秒）<Input fullWidth type="number" value={form.default_timeout_sec} onChange={(event) => setForm((current) => ({ ...current, default_timeout_sec: Number(event.target.value) }))} /></label>
          <label className={styles.field}>状态<Select fullWidth value={String(form.status)} onChange={(value) => setForm((current) => ({ ...current, status: Number(value) as JudgerStatus }))} options={[{ value: '1', label: '可用' }, { value: '2', label: '停用' }]} /></label>
        </div>
        <Switch checked={form.runtime_required} label="需要运行环境" onChange={(event) => setForm((current) => ({ ...current, runtime_required: event.target.checked }))} />
        <label className={styles.field}>资源约束<Textarea rows={4} fullWidth value={resourceSpec} onChange={(event) => setResourceSpec(event.target.value)} /></label>
        <Button icon={<Save size={15} />} onClick={() => void saveJudger()}>{editingId ? '保存更新' : '创建判题器'}</Button>
      </section>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取判题器" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无判题器"
            emptyDescription="当前平台还没有登记判题器。"
            ariaLabel="平台判题器列表"
          />
        </div>
      )}
    </div>
  )
}

export default JudgesPage
