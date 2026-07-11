// SandboxToolsPage 展示平台沙箱工具定义，数据来自 sandbox 后端模块。

import React, { useMemo, useState } from 'react'
import type { SandboxToolDefinition, SandboxToolRequest } from '@chaimir/api-client'
import { SandboxToolKind, ToolStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table, Textarea } from '@chaimir/ui'
import { Package, Plus, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { sandboxToolKindLabel, toolStatusLabel } from '../../../../../utils/index'
import { parseJsonObject } from '../../../../../utils'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

/**
 * SandboxToolsPage 读取全局沙箱工具定义。
 */
const SandboxToolsPage: React.FC = () => {
  const resource = useAsyncResource(async () => {
    const [tools, quota] = await Promise.all([api.sandbox.listTools(), api.sandbox.getQuota()])
    return { tools, quota }
  }, [])
  const [form, setForm] = useState<SandboxToolRequest>({ code: '', name: '', kind: SandboxToolKind.BUILTIN, eco_tags: [], resource_spec: {}, status: ToolStatus.AVAILABLE })
  const [ecoTags, setEcoTags] = useState('')
  const [resourceSpec, setResourceSpec] = useState('{}')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const rows = resource.data?.tools || []

  /** registerTool 登记平台统一沙箱工具声明。 */
  const registerTool = async () => {
    setError('')
    try {
      await api.sandbox.registerTool({ ...form, eco_tags: ecoTags.split(',').map((item) => item.trim()).filter(Boolean), resource_spec: parseJsonObject(resourceSpec) })
      setMessage('沙箱工具已登记。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '沙箱工具登记失败，请检查配置后重试。'))
    }
  }

  const columns = useMemo<TableColumn<SandboxToolDefinition>[]>(() => [
    { key: 'name', title: '工具名称', dataIndex: 'name', priority: 'primary' },
    { key: 'code', title: '工具编码', dataIndex: 'code', priority: 'secondary' },
    { key: 'kind', title: '工具类型', render: (row) => sandboxToolKindLabel(row.kind) },
    {
      key: 'ecos',
      title: '适用生态',
      render: (row) => row.eco_tags.join('、') || '通用',
    },
    {
      key: 'status',
      title: '全局状态',
      render: (row) => <span className={styles.status}>{toolStatusLabel(row.status)}</span>,
    },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Package className={styles.icon} size={28} />
            全局沙箱工具链
          </h1>
          <p className={styles.subtitle}>查看平台登记的 Web 工具和受控命令工具。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
      {resource.data?.quota && (
        <Callout variant="info" title="当前沙箱配额">
          活跃 {resource.data.quota.active_sandbox_count || 0}/{resource.data.quota.max_concurrent_sandbox}，CPU 上限 {resource.data.quota.max_cpu}，内存上限 {resource.data.quota.max_memory_mb} MB。
        </Callout>
      )}
      <section className={styles.tableWrap}>
        <div className={styles.formGrid}>
          <Input value={form.name} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} placeholder="工具名称" fullWidth />
          <Input value={form.code} onChange={(event) => setForm((current) => ({ ...current, code: event.target.value }))} placeholder="工具编号" fullWidth />
          <Select value={String(form.kind)} onChange={(value) => setForm((current) => ({ ...current, kind: Number(value) as SandboxToolKind }))} options={[{ value: '1', label: '平台内置' }, { value: '2', label: '终端' }, { value: '3', label: 'Web 工具' }, { value: '4', label: '受控命令' }]} />
          <Select value={String(form.status)} onChange={(value) => setForm((current) => ({ ...current, status: Number(value) as ToolStatus }))} options={[{ value: '1', label: '可用' }, { value: '2', label: '停用' }]} />
          <Input value={ecoTags} onChange={(event) => setEcoTags(event.target.value)} placeholder="适用生态，多个用逗号分隔" fullWidth />
          <Textarea rows={4} value={resourceSpec} onChange={(event) => setResourceSpec(event.target.value)} placeholder="资源约束" fullWidth />
          <Button icon={<Plus size={15} />} onClick={() => void registerTool()}>登记工具</Button>
        </div>
      </section>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取沙箱工具" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无工具"
            emptyDescription="当前平台还没有登记沙箱工具。"
            ariaLabel="平台沙箱工具列表"
          />
        </div>
      )}
    </div>
  )
}

export default SandboxToolsPage
