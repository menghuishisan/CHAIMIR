// JudgesPage 展示平台判题器配置列表，数据来自 judge 后端模块。

import React, { useState } from 'react'
import type { Judger, JudgerRequest, JudgerResourceSpec, WorkloadComponent } from '@chaimir/api-client'
import { JudgerStatus, JudgerType } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Switch, Table, ResourceState, FormField } from '@chaimir/ui'
import { Cpu, Play, RefreshCw, Save } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource, usePendingAction } from '../../../../../hooks'
import styles from '../../judge.module.css'
import { formatSeconds, judgerStatusLabel, judgerTypeLabel } from '../../../../../utils/index'
import { parseDelimitedList } from '../../../../../utils'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


/**
 * JudgesPage 读取判题器声明和执行器状态。
 */
const JudgesPage: React.FC = () => {
  const resource = useAsyncResource(() => api.judge.listJudgers(), [])
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState<Omit<JudgerRequest, 'resource_spec'>>({ code: '', name: '', type: JudgerType.TESTCASE, executor_ref: '', runtime_required: true, default_timeout_sec: 30, status: JudgerStatus.AVAILABLE })
  const [editingSpec, setEditingSpec] = useState<JudgerResourceSpec | null>(null)
  const [runtimeCode, setRuntimeCode] = useState('')
  const [runtimeVersion, setRuntimeVersion] = useState('')
  const [genesisRef, setGenesisRef] = useState('')
  const [toolCodes, setToolCodes] = useState('')
  const [judgeCommand, setJudgeCommand] = useState('')
  const [sidecarName, setSidecarName] = useState('judger')
  const [sidecarCommand, setSidecarCommand] = useState('sleep, 2147483647')
  const [suiteArchiveName, setSuiteArchiveName] = useState('tests.tar.gz')
  const [maxRetries, setMaxRetries] = useState('1')
  const [cpuRequest, setCPURequest] = useState('500m')
  const [memoryRequest, setMemoryRequest] = useState('1Gi')
  const [cpuLimit, setCPULimit] = useState('2')
  const [memoryLimit, setMemoryLimit] = useState('4Gi')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()
  const rows = resource.data || []

  /** saveJudger 创建或更新完整判题器声明。 */
  const saveJudger = async () => {
    setError('')
    try {
      if (!form.name.trim() || !/^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/.test(form.code.trim()) || !form.executor_ref.trim() || form.default_timeout_sec <= 0) {
        setError('请填写名称、执行器地址、默认时限和符合格式的判题器编号。')
        return
      }
      const payload: JudgerRequest = { ...form, resource_spec: buildJudgerResourceSpec({ base: editingSpec, type: form.type, runtimeRequired: form.runtime_required, executorRef: form.executor_ref, runtimeCode, runtimeVersion, genesisRef, toolCodes, judgeCommand, sidecarName, sidecarCommand, suiteArchiveName, timeout: form.default_timeout_sec, maxRetries: Number(maxRetries), cpuRequest, memoryRequest, cpuLimit, memoryLimit }) }
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
    const { resource_spec: spec } = judger
    setForm({ code: judger.code, name: judger.name, type: judger.type, executor_ref: judger.executor_ref, runtime_required: judger.runtime_required, default_timeout_sec: judger.default_timeout_sec, status: judger.status })
    setEditingSpec(spec)
    setRuntimeCode(spec.runtime_code || '')
    setRuntimeVersion(spec.runtime_image_version || '')
    setGenesisRef(spec.genesis_ref || '')
    setToolCodes((spec.tool_codes || []).join(', '))
    setJudgeCommand((spec.command || []).join(', '))
    const sidecar = spec.execution_sidecars?.[0]
    setSidecarName(sidecar?.name || 'judger')
    setSidecarCommand((sidecar?.command || ['sleep', '2147483647']).join(', '))
    setSuiteArchiveName(spec.suite_archive_name || 'tests.tar.gz')
    setMaxRetries(String(spec.max_retries ?? 1))
    setCPURequest(sidecar?.resources?.requests.cpu || '500m')
    setMemoryRequest(sidecar?.resources?.requests.memory || '1Gi')
    setCPULimit(sidecar?.resources?.limits.cpu || '2')
    setMemoryLimit(sidecar?.resources?.limits.memory || '4Gi')
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
    { key: 'actions', title: '操作', render: (row) => <div className={styles.actions}><Button variant="outline" size="sm" disabled={Boolean(pendingAction)} onClick={() => editJudger(row)}>编辑</Button><Button variant="ghost" size="sm" icon={<Play size={14} />} loading={pendingAction === `selftest-${row.id}`} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction(`selftest-${row.id}`, () => selftestJudger(row))}>运行自测</Button></div> },
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
          <FormField className={styles.field} label="名称"><Input fullWidth value={form.name} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} /></FormField>
          <FormField className={styles.field} label="编号"><Input fullWidth value={form.code} onChange={(event) => setForm((current) => ({ ...current, code: event.target.value }))} /></FormField>
          <FormField className={styles.field} label="执行器地址"><Input fullWidth value={form.executor_ref} onChange={(event) => setForm((current) => ({ ...current, executor_ref: event.target.value }))} /></FormField>
          <FormField className={styles.field} label="判题类型"><Select fullWidth value={String(form.type)} onChange={(value) => setForm((current) => ({ ...current, type: Number(value) as JudgerType }))} options={Object.values(JudgerType).filter((value): value is number => typeof value === 'number').map((value) => ({ value: String(value), label: judgerTypeLabel(value) }))} /></FormField>
          <FormField className={styles.field} label="默认时限（秒）"><Input fullWidth type="number" value={form.default_timeout_sec} onChange={(event) => setForm((current) => ({ ...current, default_timeout_sec: Number(event.target.value) }))} /></FormField>
          <FormField className={styles.field} label="状态"><Select fullWidth value={String(form.status)} onChange={(value) => setForm((current) => ({ ...current, status: Number(value) as JudgerStatus }))} options={[{ value: '1', label: '可用' }, { value: '2', label: '停用' }]} /></FormField>
        </div>
        <Switch checked={form.runtime_required} label="需要运行环境" onChange={(event) => setForm((current) => ({ ...current, runtime_required: event.target.checked }))} />
        {(form.runtime_required || [JudgerType.TESTCASE, JudgerType.ONCHAIN_ASSERT, JudgerType.STATIC_SCAN].includes(form.type)) && <div className={styles.formGrid}>
          <FormField className={styles.field} label="运行时编号"><Input fullWidth value={runtimeCode} onChange={(event) => setRuntimeCode(event.target.value)} /></FormField>
          <FormField className={styles.field} label="运行时镜像版本"><Input fullWidth value={runtimeVersion} onChange={(event) => setRuntimeVersion(event.target.value)} /></FormField>
          <FormField className={styles.field} label="创世配置引用"><Input fullWidth value={genesisRef} onChange={(event) => setGenesisRef(event.target.value)} /></FormField>
          <FormField className={styles.field} label="配套工具编号"><Input fullWidth value={toolCodes} onChange={(event) => setToolCodes(event.target.value)} /></FormField>
        </div>}
        {[JudgerType.TESTCASE, JudgerType.STATIC_SCAN].includes(form.type) && <div className={styles.formGrid}>
          <FormField className={styles.field} label="判题命令"><Input fullWidth value={judgeCommand} onChange={(event) => setJudgeCommand(event.target.value)} /></FormField>
          <FormField className={styles.field} label="测试包文件名"><Input fullWidth value={suiteArchiveName} onChange={(event) => setSuiteArchiveName(event.target.value)} /></FormField>
          <FormField className={styles.field} label="执行容器名称"><Input fullWidth value={sidecarName} onChange={(event) => setSidecarName(event.target.value)} /></FormField>
          <FormField className={styles.field} label="容器常驻命令"><Input fullWidth value={sidecarCommand} onChange={(event) => setSidecarCommand(event.target.value)} /></FormField>
          <FormField className={styles.field} label="CPU 请求量"><Input fullWidth value={cpuRequest} onChange={(event) => setCPURequest(event.target.value)} /></FormField>
          <FormField className={styles.field} label="内存请求量"><Input fullWidth value={memoryRequest} onChange={(event) => setMemoryRequest(event.target.value)} /></FormField>
          <FormField className={styles.field} label="CPU 上限"><Input fullWidth value={cpuLimit} onChange={(event) => setCPULimit(event.target.value)} /></FormField>
          <FormField className={styles.field} label="内存上限"><Input fullWidth value={memoryLimit} onChange={(event) => setMemoryLimit(event.target.value)} /></FormField>
        </div>}
        <FormField className={styles.field} label="最大重试次数"><Input fullWidth type="number" value={maxRetries} onChange={(event) => setMaxRetries(event.target.value)} /></FormField>
        <Button icon={<Save size={15} />} loading={pendingAction === 'save'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('save', saveJudger)}>{editingId ? '保存更新' : '创建判题器'}</Button>
      </section>

      {resource.status === 'error' && (
        <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取判题器" />}
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

interface JudgerSpecForm {
  base: JudgerResourceSpec | null
  type: JudgerType
  runtimeRequired: boolean
  executorRef: string
  runtimeCode: string
  runtimeVersion: string
  genesisRef: string
  toolCodes: string
  judgeCommand: string
  sidecarName: string
  sidecarCommand: string
  suiteArchiveName: string
  timeout: number
  maxRetries: number
  cpuRequest: string
  memoryRequest: string
  cpuLimit: string
  memoryLimit: string
}

/** buildJudgerResourceSpec 按判题类型生成后端唯一资源声明，并保留未编辑的高级字段。 */
function buildJudgerResourceSpec(values: JudgerSpecForm): JudgerResourceSpec {
  if (values.maxRetries < 0) throw new Error('最大重试次数不能小于零。')
  const requiresRuntime = values.runtimeRequired || [JudgerType.TESTCASE, JudgerType.ONCHAIN_ASSERT, JudgerType.STATIC_SCAN].includes(values.type)
  if (requiresRuntime && (!values.runtimeCode.trim() || !values.runtimeVersion.trim() || !values.genesisRef.trim())) {
    throw new Error('该判题类型需要完整的运行时编号、镜像版本和创世配置引用。')
  }
  const requiresExecutor = [JudgerType.TESTCASE, JudgerType.STATIC_SCAN].includes(values.type)
  let executionSidecars = values.base?.execution_sidecars
  let command = values.base?.command || []
  let execTarget = values.base?.exec_target
  if (requiresExecutor) {
    command = parseDelimitedList(values.judgeCommand)
    const keepalive = parseDelimitedList(values.sidecarCommand)
    if (!command.length || !keepalive.length || !values.sidecarName.trim() || !values.executorRef.includes('@sha256:')) {
      throw new Error('请填写判题命令、执行容器和带摘要的执行器镜像地址。')
    }
    const baseSidecar = values.base?.execution_sidecars?.[0]
    const sidecar: WorkloadComponent = {
      ...baseSidecar,
      name: values.sidecarName.trim(),
      image_url: values.executorRef.trim(),
      command: keepalive,
      workdir: '/workspace',
      resources: {
        requests: { cpu: values.cpuRequest.trim(), memory: values.memoryRequest.trim() },
        limits: { cpu: values.cpuLimit.trim(), memory: values.memoryLimit.trim() },
      },
      mount_workspace: true,
      read_only_root_filesystem: true,
      labels: { ...baseSidecar?.labels, 'chaimir.io/student-access': 'false', 'chaimir.io/sensitivity': 'judge-private' },
    }
    executionSidecars = [sidecar, ...(values.base?.execution_sidecars?.slice(1) || [])]
    execTarget = `sandbox/${sidecar.name}`
  }
  return {
    ...values.base,
    runtime_code: requiresRuntime ? values.runtimeCode.trim() : undefined,
    runtime_image_version: requiresRuntime ? values.runtimeVersion.trim() : undefined,
    genesis_ref: requiresRuntime ? values.genesisRef.trim() : undefined,
    tool_codes: parseDelimitedList(values.toolCodes),
    command: values.type === JudgerType.MANUAL ? undefined : command,
    exec_target: requiresExecutor ? execTarget : undefined,
    execution_sidecars: requiresExecutor ? executionSidecars : undefined,
    timeout_sec: values.timeout,
    max_retries: values.maxRetries,
    suite_archive_name: requiresExecutor ? values.suiteArchiveName.trim() : undefined,
  }
}
