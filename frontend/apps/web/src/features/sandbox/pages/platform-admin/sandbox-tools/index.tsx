// SandboxToolsPage 展示平台沙箱工具定义，数据来自 sandbox 后端模块。

import React, { useMemo, useState } from 'react'
import type { SandboxToolDefinition, SandboxToolRequest, SandboxToolResourceSpec, WorkloadComponent } from '@chaimir/api-client'
import { SandboxToolKind, ToolStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Switch, Table, ResourceState } from '@chaimir/ui'
import { Package, Plus, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource, usePendingAction } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { sandboxToolKindLabel, toolStatusLabel } from '../../../../../utils/index'
import { parseDelimitedList } from '../../../../../utils'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

/**
 * SandboxToolsPage 读取全局沙箱工具定义。
 */
const SandboxToolsPage: React.FC = () => {
  const resource = useAsyncResource(async () => {
    const tools = await api.sandbox.listTools()
    return { tools }
  }, [])
  const [form, setForm] = useState<Omit<SandboxToolRequest, 'eco_tags' | 'resource_spec'>>({ code: '', name: '', kind: SandboxToolKind.BUILTIN, status: ToolStatus.AVAILABLE })
  const [ecoTags, setEcoTags] = useState('')
  const [builtinEndpoint, setBuiltinEndpoint] = useState('/api/v1/sandbox/sandboxes/{sandbox_id}/files')
  const [imageUrl, setImageUrl] = useState('')
  const [containerCommand, setContainerCommand] = useState('')
  const [prepullCommand, setPrepullCommand] = useState('')
  const [containerPort, setContainerPort] = useState('8080')
  const [healthPath, setHealthPath] = useState('/')
  const [mountWorkspace, setMountWorkspace] = useState(true)
  const [cpuRequest, setCPURequest] = useState('100m')
  const [memoryRequest, setMemoryRequest] = useState('128Mi')
  const [cpuLimit, setCPULimit] = useState('1')
  const [memoryLimit, setMemoryLimit] = useState('1Gi')
  const [allowedCommands, setAllowedCommands] = useState('')
  const [defaultTimeout, setDefaultTimeout] = useState('30')
  const [maxTimeout, setMaxTimeout] = useState('120')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()
  const rows = resource.data?.tools || []

  /** registerTool 登记平台统一沙箱工具声明。 */
  const registerTool = async () => {
    setError('')
    try {
      if (!form.name.trim() || !/^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/.test(form.code.trim())) {
        setError('请填写工具名称和符合格式的工具编号。')
        return
      }
      const resourceSpec = buildToolResourceSpec({ kind: form.kind, builtinEndpoint, imageUrl, containerCommand, prepullCommand, containerPort: Number(containerPort), healthPath, mountWorkspace, cpuRequest, memoryRequest, cpuLimit, memoryLimit, allowedCommands, defaultTimeout: Number(defaultTimeout), maxTimeout: Number(maxTimeout) })
      await api.sandbox.registerTool({ ...form, eco_tags: parseDelimitedList(ecoTags), resource_spec: resourceSpec })
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
      <section className={styles.tableWrap}>
        <div className={styles.formGrid}>
          <Input value={form.name} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} placeholder="工具名称" fullWidth />
          <Input value={form.code} onChange={(event) => setForm((current) => ({ ...current, code: event.target.value }))} placeholder="工具编号" fullWidth />
          <Select value={String(form.kind)} onChange={(value) => setForm((current) => ({ ...current, kind: Number(value) as SandboxToolKind }))} options={[{ value: '1', label: '平台内置' }, { value: '2', label: '终端' }, { value: '3', label: 'Web 工具' }, { value: '4', label: '受控命令' }]} />
          <Select value={String(form.status)} onChange={(value) => setForm((current) => ({ ...current, status: Number(value) as ToolStatus }))} options={[{ value: '1', label: '可用' }, { value: '2', label: '停用' }]} />
          <Input value={ecoTags} onChange={(event) => setEcoTags(event.target.value)} placeholder="适用生态，多个用逗号分隔" fullWidth />
          {form.kind === SandboxToolKind.BUILTIN && <Input value={builtinEndpoint} onChange={(event) => setBuiltinEndpoint(event.target.value)} placeholder="平台功能入口" fullWidth />}
          {(form.kind === SandboxToolKind.WEB_EMBED || form.kind === SandboxToolKind.COMMAND) && <>
            <Input value={imageUrl} onChange={(event) => setImageUrl(event.target.value)} placeholder="已通过准入的镜像地址与摘要" fullWidth />
            <Input value={containerCommand} onChange={(event) => setContainerCommand(event.target.value)} placeholder={form.kind === SandboxToolKind.COMMAND ? '容器常驻命令，使用逗号分隔' : '容器启动命令，使用逗号分隔'} fullWidth />
            <Input value={prepullCommand} onChange={(event) => setPrepullCommand(event.target.value)} placeholder="镜像预拉取检查命令，使用逗号分隔" fullWidth />
            <Input value={cpuRequest} onChange={(event) => setCPURequest(event.target.value)} placeholder="CPU 请求量" fullWidth />
            <Input value={memoryRequest} onChange={(event) => setMemoryRequest(event.target.value)} placeholder="内存请求量" fullWidth />
            <Input value={cpuLimit} onChange={(event) => setCPULimit(event.target.value)} placeholder="CPU 上限" fullWidth />
            <Input value={memoryLimit} onChange={(event) => setMemoryLimit(event.target.value)} placeholder="内存上限" fullWidth />
            <Switch checked={mountWorkspace} label="挂载学生工作区" onChange={(event) => setMountWorkspace(event.target.checked)} />
          </>}
          {form.kind === SandboxToolKind.WEB_EMBED && <>
            <Input type="number" value={containerPort} onChange={(event) => setContainerPort(event.target.value)} placeholder="Web 服务端口" fullWidth />
            <Input value={healthPath} onChange={(event) => setHealthPath(event.target.value)} placeholder="健康检查路径" fullWidth />
          </>}
          {form.kind === SandboxToolKind.COMMAND && <>
            <Input value={allowedCommands} onChange={(event) => setAllowedCommands(event.target.value)} placeholder="允许执行的命令名，使用逗号分隔" fullWidth />
            <Input type="number" value={defaultTimeout} onChange={(event) => setDefaultTimeout(event.target.value)} placeholder="默认超时秒数" fullWidth />
            <Input type="number" value={maxTimeout} onChange={(event) => setMaxTimeout(event.target.value)} placeholder="最大超时秒数" fullWidth />
          </>}
          <Button icon={<Plus size={15} />} loading={pendingAction === 'tool'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('tool', registerTool)}>登记工具</Button>
        </div>
      </section>

      {resource.status === 'error' && (
        <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取沙箱工具" />}
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

interface ToolSpecForm {
  kind: SandboxToolKind
  builtinEndpoint: string
  imageUrl: string
  containerCommand: string
  prepullCommand: string
  containerPort: number
  healthPath: string
  mountWorkspace: boolean
  cpuRequest: string
  memoryRequest: string
  cpuLimit: string
  memoryLimit: string
  allowedCommands: string
  defaultTimeout: number
  maxTimeout: number
}

/** buildToolResourceSpec 按工具类型生成唯一合法资源声明，不接受自由 JSON。 */
function buildToolResourceSpec(values: ToolSpecForm): SandboxToolResourceSpec {
  if (values.kind === SandboxToolKind.TERMINAL) return {}
  if (values.kind === SandboxToolKind.BUILTIN) {
    if (!values.builtinEndpoint.startsWith('/api/v1/sandbox/sandboxes/{sandbox_id}')) {
      throw new Error('平台功能入口必须位于沙箱受控路径下。')
    }
    return { builtin_endpoint: values.builtinEndpoint.trim() }
  }
  if (!values.imageUrl.includes('@sha256:')) throw new Error('镜像地址必须包含不可变摘要。')
  const command = parseDelimitedList(values.containerCommand)
  const prepull = parseDelimitedList(values.prepullCommand)
  if (!command.length || !prepull.length) throw new Error('请填写容器命令和镜像预拉取检查命令。')
  const component: WorkloadComponent = {
    name: values.kind === SandboxToolKind.COMMAND ? 'command' : 'web',
    image_url: values.imageUrl.trim(),
    command,
    resources: {
      requests: { cpu: values.cpuRequest.trim(), memory: values.memoryRequest.trim() },
      limits: { cpu: values.cpuLimit.trim(), memory: values.memoryLimit.trim() },
    },
    read_only_root_filesystem: true,
    mount_workspace: values.mountWorkspace,
  }
  if (values.kind === SandboxToolKind.COMMAND) {
    const allowed = parseDelimitedList(values.allowedCommands)
    if (!allowed.length || values.defaultTimeout <= 0 || values.maxTimeout < values.defaultTimeout) {
      throw new Error('请填写允许执行的命令，并确保最大超时不小于默认超时。')
    }
    return {
      components: [component],
      command_policy: { allowed_commands: allowed, default_timeout_seconds: values.defaultTimeout, max_timeout_seconds: values.maxTimeout },
      prepull_command: prepull,
    }
  }
  if (values.containerPort < 1 || values.containerPort > 65535 || !values.healthPath.startsWith('/')) {
    throw new Error('请检查 Web 服务端口和健康检查路径。')
  }
  component.ports = [{ name: 'http', container_port: values.containerPort, service_port: values.containerPort, protocol: 'TCP' }]
  component.readiness_probe = { type: 'http', path: values.healthPath.trim(), port: 'http' }
  return {
    components: [component],
    services: [{ name: 'tool-web', component: 'web', ports: [{ name: 'http', port: values.containerPort, target_port: 'http', protocol: 'TCP' }] }],
    routes: [{ path_prefix: '/', service: 'tool-web', port: 'http' }],
    prepull_command: prepull,
  }
}
