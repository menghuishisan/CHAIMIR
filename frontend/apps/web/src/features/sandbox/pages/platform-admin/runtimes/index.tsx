// RuntimesPage 展示平台链运行时列表，数据来自 sandbox 后端模块。

import React, { useMemo, useState } from 'react'
import type { SandboxAdapterSpec, SandboxRuntime, SandboxRuntimeRequest, WorkloadComponent } from '@chaimir/api-client'
import { RuntimeStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table, ResourceState } from '@chaimir/ui'
import { Eye, Pencil, Plus, RefreshCw, Server } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource, usePendingAction } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { runtimeSelftestStatusLabel, runtimeStatusLabel } from '../../../../../utils/index'
import { parseDelimitedList } from '../../../../../utils'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

/**
 * RuntimesPage 读取链运行时声明和自检状态。
 */
const RuntimesPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(() => api.sandbox.listRuntimes(), [])
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState<Omit<SandboxRuntimeRequest, 'adapter_spec'>>({ code: '', name: '', eco: '', adapter_level: 1, capability_impl: '', plugin_ref: '', status: RuntimeStatus.ONBOARDING })
  const [editingSpec, setEditingSpec] = useState<SandboxAdapterSpec | null>(null)
  const [workspaceDir, setWorkspaceDir] = useState('/workspace')
  const [containerName, setContainerName] = useState('runtime')
  const [containerCommand, setContainerCommand] = useState('')
  const [portName, setPortName] = useState('rpc')
  const [port, setPort] = useState('8545')
  const [cpuRequest, setCPURequest] = useState('250m')
  const [memoryRequest, setMemoryRequest] = useState('512Mi')
  const [cpuLimit, setCPULimit] = useState('2')
  const [memoryLimit, setMemoryLimit] = useState('2Gi')
  const [defaultTools, setDefaultTools] = useState('terminal')
  const [capabilityExecutable, setCapabilityExecutable] = useState('/usr/local/bin/chaimir-chain')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()
  const rows = resource.data || []

  /** saveRuntime 创建或更新完整运行时适配声明。 */
  const saveRuntime = async () => {
    setError('')
    try {
      if (!form.name.trim() || !/^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/.test(form.code.trim()) || !form.eco.trim()) {
        setError('请填写名称、生态和符合格式的运行时编号。')
        return
      }
      if (!workspaceDir.startsWith('/') || !containerName.trim() || !portName.trim() || Number(port) < 1 || Number(port) > 65535) {
        setError('请检查工作区目录、容器名称和服务端口。')
        return
      }
      if (form.adapter_level >= 2 && !form.capability_impl.trim() && !form.plugin_ref.trim() && !capabilityExecutable.trim() && !editingSpec?.capability_commands) {
        setError('L2/L3 运行时需要选择能力实现、插件或填写标准命令入口。')
        return
      }
      const payload: SandboxRuntimeRequest = { ...form, adapter_spec: buildAdapterSpec({ editingSpec, workspaceDir, containerName, containerCommand, portName, port: Number(port), cpuRequest, memoryRequest, cpuLimit, memoryLimit, defaultTools, capabilityExecutable, adapterLevel: form.adapter_level, hasExternalCapability: Boolean(form.capability_impl.trim() || form.plugin_ref.trim()) }) }
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
    const spec = runtime.adapter_spec
    const component = spec.runtime_container
    const firstPort = component.ports?.[0]
    setForm({
      code: runtime.code,
      name: runtime.name,
      eco: runtime.eco,
      adapter_level: runtime.adapter_level,
      capability_impl: runtime.capability_impl,
      plugin_ref: runtime.plugin_ref,
      status: undefined,
    })
    setEditingSpec(spec)
    setWorkspaceDir(spec.workspace_dir)
    setContainerName(component.name)
    setContainerCommand((component.command || []).join(', '))
    setPortName(firstPort?.name || 'rpc')
    setPort(String(firstPort?.container_port || 8545))
    setCPURequest(component.resources?.requests.cpu || '250m')
    setMemoryRequest(component.resources?.requests.memory || '512Mi')
    setCPULimit(component.resources?.limits.cpu || '2')
    setMemoryLimit(component.resources?.limits.memory || '2Gi')
    setDefaultTools((spec.default_tool_codes || []).join(', '))
    setCapabilityExecutable('')
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
          <Select
            value={String(form.status ?? 0)}
            onChange={(value) => setForm((current) => ({ ...current, status: value === '0' ? undefined : Number(value) as RuntimeStatus }))}
            options={[
              ...(editingId ? [{ value: '0', label: '保持当前状态' }] : []),
              { value: '2', label: '接入中' },
              { value: '3', label: '停用' },
            ]}
          />
          <Input value={workspaceDir} onChange={(event) => setWorkspaceDir(event.target.value)} placeholder="工作区目录" fullWidth />
          <Input value={containerName} onChange={(event) => setContainerName(event.target.value)} placeholder="主容器名称" fullWidth />
          <Input value={containerCommand} onChange={(event) => setContainerCommand(event.target.value)} placeholder="启动命令参数，使用逗号分隔" fullWidth />
          <Input value={portName} onChange={(event) => setPortName(event.target.value)} placeholder="服务端口名称" fullWidth />
          <Input type="number" value={port} onChange={(event) => setPort(event.target.value)} placeholder="服务端口" fullWidth />
          <Input value={cpuRequest} onChange={(event) => setCPURequest(event.target.value)} placeholder="CPU 请求量" fullWidth />
          <Input value={memoryRequest} onChange={(event) => setMemoryRequest(event.target.value)} placeholder="内存请求量" fullWidth />
          <Input value={cpuLimit} onChange={(event) => setCPULimit(event.target.value)} placeholder="CPU 上限" fullWidth />
          <Input value={memoryLimit} onChange={(event) => setMemoryLimit(event.target.value)} placeholder="内存上限" fullWidth />
          <Input value={defaultTools} onChange={(event) => setDefaultTools(event.target.value)} placeholder="默认工具编号，使用逗号分隔" fullWidth />
          {form.adapter_level >= 2 && !form.capability_impl && !form.plugin_ref && <Input value={capabilityExecutable} onChange={(event) => setCapabilityExecutable(event.target.value)} placeholder={editingSpec ? '标准链能力命令入口，留空保留现有声明' : '标准链能力命令入口'} fullWidth />}
          <Button icon={<Plus size={15} />} loading={pendingAction === 'runtime'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('runtime', saveRuntime)}>{editingId ? '保存运行时' : '登记运行时'}</Button>
        </div>
      </section>

      {resource.status === 'error' && (
        <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取链运行时" />}
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

interface AdapterSpecForm {
  editingSpec: SandboxAdapterSpec | null
  workspaceDir: string
  containerName: string
  containerCommand: string
  portName: string
  port: number
  cpuRequest: string
  memoryRequest: string
  cpuLimit: string
  memoryLimit: string
  defaultTools: string
  capabilityExecutable: string
  adapterLevel: number
  hasExternalCapability: boolean
}

/** buildAdapterSpec 生成后端唯一声明式运行时契约，并保留未在基础表单修改的高级拓扑。 */
function buildAdapterSpec(values: AdapterSpecForm): SandboxAdapterSpec {
  const baseComponent = values.editingSpec?.runtime_container
  const firstPort = {
    name: values.portName.trim(),
    container_port: values.port,
    service_port: values.port,
    protocol: 'TCP' as const,
  }
  const component: WorkloadComponent = {
    ...baseComponent,
    name: values.containerName.trim(),
    command: parseDelimitedList(values.containerCommand),
    ports: [firstPort, ...(baseComponent?.ports?.slice(1) || [])],
    resources: {
      requests: { cpu: values.cpuRequest.trim(), memory: values.memoryRequest.trim() },
      limits: { cpu: values.cpuLimit.trim(), memory: values.memoryLimit.trim() },
    },
    readiness_probe: { type: 'tcp', port: firstPort.name },
  }
  const workspaceHelper = '/usr/local/bin/chaimir-workspace'
  const standardWorkspaceOps: SandboxAdapterSpec['workspace_ops'] = {
    read_file: [workspaceHelper, 'read', '{{workspace}}', '{{path}}'],
    write_file: [workspaceHelper, 'write', '{{workspace}}', '{{path}}'],
    list_files: [workspaceHelper, 'list', '{{workspace}}', '{{path}}'],
    pack_tar: [workspaceHelper, 'pack', '{{workspace}}', '{{path}}'],
    unpack_tar: [workspaceHelper, 'unpack', '{{workspace}}', '{{path}}'],
    run_script: [workspaceHelper, 'run', '{{workspace}}', '{{workspace}}', '{{script}}'],
    terminal: [workspaceHelper, 'terminal', '{{workspace}}'],
    selftest: [workspaceHelper, 'selftest'],
  }
  const executable = values.capabilityExecutable.trim()
  const capabilityCommands = values.adapterLevel >= 2 && !values.hasExternalCapability
    ? executable
      ? Object.fromEntries(['deploy', 'tx', 'query', 'reset'].map((action) => [action, { command: [executable, action], timeout_seconds: action === 'deploy' || action === 'tx' ? 60 : 30 }])) as SandboxAdapterSpec['capability_commands']
      : values.editingSpec?.capability_commands
    : undefined
  return {
    ...values.editingSpec,
    workspace_dir: values.workspaceDir.trim(),
    runtime_container: component,
    workspace_ops: values.editingSpec?.workspace_ops || standardWorkspaceOps,
    default_tool_codes: parseDelimitedList(values.defaultTools),
    capability_commands: capabilityCommands,
  }
}
