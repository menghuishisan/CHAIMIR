// 运行时登记表单：用字段级控件编辑受控声明，页面不接收或解析原始 JSON。

import { useCallback, useId, useMemo, useState } from 'react'
import {
  RuntimeAdapterLevel,
  RuntimeStatus,
  type SandboxAdapterSpec,
  type SandboxRuntime,
  type SandboxRuntimeRequest,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  SegmentedControl,
  Select,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { runtimeStatusLabel } from '../../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

const ADAPTER_LEVELS = [
  {
    value: String(RuntimeAdapterLevel.HOSTED),
    label: '一级 · 只托管环境',
    hint: '平台只负责启动环境',
  },
  {
    value: String(RuntimeAdapterLevel.STANDARD),
    label: '二级 · 声明标准链能力',
    hint: '平台统一调用部署、交易、查询和重置命令',
  },
  {
    value: String(RuntimeAdapterLevel.PLUGIN),
    label: '三级 · 自带插件实现',
    hint: '链能力由运行时插件实现',
  },
] as const

const SUBMITTABLE_STATUSES = [RuntimeStatus.ONBOARDING, RuntimeStatus.DISABLED] as const
const WORKSPACE_OP_KEYS = [
  'read_file',
  'write_file',
  'list_files',
  'pack_tar',
  'unpack_tar',
  'run_script',
  'terminal',
  'selftest',
] as const
type WorkspaceOpKey = (typeof WORKSPACE_OP_KEYS)[number]
const WORKSPACE_OP_LABELS: Record<WorkspaceOpKey, string> = {
  read_file: '读文件',
  write_file: '写文件',
  list_files: '列目录',
  pack_tar: '打包工作区',
  unpack_tar: '解包到工作区',
  run_script: '执行脚本',
  terminal: '打开终端',
  selftest: '自检',
}

interface PortState {
  name: string
  containerPort: string
  servicePort: string
  protocol: string
  raw?: Record<string, unknown>
}
interface SidecarState {
  name: string
  imageUrl: string
  prepullCommand: string[]
  raw?: Record<string, unknown>
}
interface VolumeState {
  name: string
  mountPath: string
  studentAccess: string
  persistence: string
  snapshotScope: string
  raw?: Record<string, unknown>
}
interface CapabilityState {
  command: string[]
  timeout: string
}
interface FormSpec {
  baseSpec: Record<string, unknown>
  workspaceDir: string
  containerName: string
  imageUrl: string
  ports: PortState[]
  sidecars: SidecarState[]
  volumes: VolumeState[]
  workspaceOps: Record<WorkspaceOpKey, string[]>
  capabilities: Record<'deploy' | 'tx' | 'query' | 'reset', CapabilityState>
}

const emptyWorkspaceOps = (): Record<WorkspaceOpKey, string[]> =>
  Object.fromEntries(WORKSPACE_OP_KEYS.map((key) => [key, ['']])) as Record<
    WorkspaceOpKey,
    string[]
  >
const emptyCapabilities = (): FormSpec['capabilities'] => ({
  deploy: { command: [''], timeout: '' },
  tx: { command: [''], timeout: '' },
  query: { command: [''], timeout: '' },
  reset: { command: [''], timeout: '' },
})

function componentRecord(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : ''
}
function numberText(value: unknown): string {
  return typeof value === 'number' && Number.isFinite(value) ? String(value) : ''
}
function commandText(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string')
    : []
}

function readFormSpec(runtime?: SandboxRuntime): FormSpec {
  const spec = componentRecord(runtime?.adapter_spec)
  const container = componentRecord(spec.runtime_container)
  const ports = Array.isArray(container.ports) ? container.ports : []
  const sidecars = Array.isArray(spec.infra_sidecars) ? spec.infra_sidecars : []
  const volumes = Array.isArray(spec.volume_domains) ? spec.volume_domains : []
  const ops = componentRecord(spec.workspace_ops)
  const capability = componentRecord(spec.capability_commands)
  const workspaceOps = emptyWorkspaceOps()
  for (const key of WORKSPACE_OP_KEYS) workspaceOps[key] = commandText(ops[key])
  const capabilities = emptyCapabilities()
  for (const key of ['deploy', 'tx', 'query', 'reset'] as const) {
    const item = componentRecord(capability[key])
    capabilities[key] = {
      command: commandText(item.command),
      timeout: numberText(item.timeout_seconds),
    }
  }
  return {
    baseSpec: spec,
    workspaceDir: stringValue(spec.workspace_dir) || '/workspace',
    containerName: stringValue(container.name),
    imageUrl: stringValue(container.image_url),
    ports: ports.map((item) => {
      const port = componentRecord(item)
      return {
        name: stringValue(port.name),
        containerPort: numberText(port.container_port),
        servicePort: numberText(port.service_port),
        protocol: stringValue(port.protocol) || 'TCP',
        raw: port,
      }
    }),
    sidecars: sidecars.map((item) => {
      const sidecar = componentRecord(item)
      return {
        name: stringValue(sidecar.name),
        imageUrl: stringValue(sidecar.image_url),
        prepullCommand: commandText(sidecar.prepull_command),
        raw: sidecar,
      }
    }),
    volumes: volumes.map((item) => {
      const volume = componentRecord(item)
      return {
        name: stringValue(volume.name),
        mountPath: stringValue(volume.mount_path),
        studentAccess: stringValue(volume.student_access),
        persistence: stringValue(volume.persistence),
        snapshotScope: stringValue(volume.snapshot_scope),
        raw: volume,
      }
    }),
    workspaceOps,
    capabilities,
  }
}

function buildAdapterSpec(form: FormSpec): SandboxAdapterSpec {
  return {
    ...form.baseSpec,
    workspace_dir: form.workspaceDir.trim(),
    runtime_container: {
      ...componentRecord(form.baseSpec.runtime_container),
      name: form.containerName.trim(),
      image_url: form.imageUrl.trim(),
      ports: form.ports
        .filter((port) => port.name.trim() !== '')
        .map((port) => ({
          ...port.raw,
          name: port.name.trim(),
          container_port: Number(port.containerPort),
          service_port: Number(port.servicePort),
          protocol: port.protocol.trim() || 'TCP',
        })),
    },
    infra_sidecars: form.sidecars
      .filter((sidecar) => sidecar.name.trim() !== '')
      .map((sidecar) => ({
        ...sidecar.raw,
        name: sidecar.name.trim(),
        image_url: sidecar.imageUrl.trim(),
        prepull_command: sidecar.prepullCommand,
      })),
    volume_domains: form.volumes
      .filter((volume) => volume.name.trim() !== '')
      .map((volume) => ({
        ...volume.raw,
        name: volume.name.trim(),
        mount_path: volume.mountPath.trim(),
        student_access: volume.studentAccess.trim(),
        persistence: volume.persistence.trim(),
        snapshot_scope: volume.snapshotScope.trim(),
      })),
    workspace_ops: Object.fromEntries(
      WORKSPACE_OP_KEYS.map((key) => [key, form.workspaceOps[key]])
    ),
    capability_commands: Object.fromEntries(
      (['deploy', 'tx', 'query', 'reset'] as const).map((key) => [
        key,
        {
          command: form.capabilities[key].command,
          timeout_seconds:
            form.capabilities[key].timeout.trim() === ''
              ? 0
              : Number(form.capabilities[key].timeout),
        },
      ])
    ),
  } as SandboxAdapterSpec
}

export interface RuntimeFormModalProps {
  runtime?: SandboxRuntime
  onClose: () => void
  onSaved: () => void
}

/** RuntimeFormModal 编辑运行时基本信息和后端校验所需的结构化声明字段。 */
export function RuntimeFormModal({ runtime, onClose, onSaved }: RuntimeFormModalProps) {
  const fieldId = useId()
  const editing = runtime !== undefined
  const [code, setCode] = useState(runtime?.code ?? '')
  const [name, setName] = useState(runtime?.name ?? '')
  const [eco, setEco] = useState(runtime?.eco ?? '')
  const [adapterLevel, setAdapterLevel] = useState(
    String(runtime?.adapter_level ?? RuntimeAdapterLevel.STANDARD)
  )
  const [capabilityImpl, setCapabilityImpl] = useState(runtime?.capability_impl ?? '')
  const [pluginRef, setPluginRef] = useState(runtime?.plugin_ref ?? '')
  const [status, setStatus] = useState(
    String(
      runtime?.status === RuntimeStatus.DISABLED ? RuntimeStatus.DISABLED : RuntimeStatus.ONBOARDING
    )
  )
  const [form, setForm] = useState<FormSpec>(() => readFormSpec(runtime))
  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)
  const parsed = useMemo(() => buildAdapterSpec(form), [form])

  const updateForm = useCallback(
    <K extends keyof FormSpec>(key: K, value: FormSpec[K]) =>
      setForm((current) => ({ ...current, [key]: value })),
    []
  )
  const updateOp = useCallback(
    (key: WorkspaceOpKey, value: string[]) =>
      setForm((current) => ({
        ...current,
        workspaceOps: { ...current.workspaceOps, [key]: value },
      })),
    []
  )
  const updateCapability = useCallback(
    (key: keyof FormSpec['capabilities'], field: keyof CapabilityState, value: string | string[]) =>
      setForm((current) => ({
        ...current,
        capabilities: {
          ...current.capabilities,
          [key]: { ...current.capabilities[key], [field]: value },
        },
      })),
    []
  )

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const next: Record<string, string | null> = {
        code: CODE_PATTERN.test(code.trim()) ? null : '短名只能使用小写字母、数字和连字符。',
        name: name.trim() === '' ? '请输入运行时名称。' : null,
        eco: eco.trim() === '' ? '请输入所属生态。' : null,
        workspaceDir: form.workspaceDir.startsWith('/')
          ? null
          : '工作区目录必须是环境内的绝对路径。',
        container: form.containerName.trim() === '' ? '请填写主环境名称。' : null,
        ports: form.ports.some(
          (port) =>
            port.name.trim() !== '' &&
            Number(port.containerPort) > 0 &&
            Number(port.servicePort) > 0
        )
          ? null
          : '请至少填写一个有效端口。',
        sidecars: form.sidecars.every(
          (sidecar) =>
            sidecar.name.trim() !== '' &&
            sidecar.imageUrl.trim() !== '' &&
            sidecar.prepullCommand.some((item) => item.trim() !== '')
        )
          ? null
          : '附加组件必须填写名称、镜像和预拉取命令。',
        workspaceOps: WORKSPACE_OP_KEYS.every((key) =>
          form.workspaceOps[key].some((item) => item.trim() !== '')
        )
          ? null
          : '请补齐八项工作区操作命令。',
        capabilities:
          Number(adapterLevel) < RuntimeAdapterLevel.STANDARD ||
          capabilityImpl.trim() !== '' ||
          pluginRef.trim() !== '' ||
          (['deploy', 'tx', 'query', 'reset'] as const).every((key) => {
            const capability = form.capabilities[key]
            return (
              capability.command.some((item) => item.trim() !== '') &&
              Number.isInteger(Number(capability.timeout)) &&
              Number(capability.timeout) > 0
            )
          })
            ? null
            : '请补齐四项链能力命令并填写有效超时。',
      }
      setErrors(next)
      if (Object.values(next).some((value) => value !== null)) {
        setFormError('有几项还不能提交，请按提示补全。')
        return
      }
      setFormError(undefined)
      setSubmitting(true)
      try {
        const payload: SandboxRuntimeRequest = {
          code: code.trim(),
          name: name.trim(),
          eco: eco.trim(),
          adapter_level: Number(adapterLevel) as RuntimeAdapterLevel,
          adapter_spec: parsed,
          capability_impl: capabilityImpl.trim(),
          plugin_ref: pluginRef.trim(),
          status: Number(status) as RuntimeStatus,
        }
        if (editing) await api.sandbox.updateRuntime(runtime.id, payload)
        else await api.sandbox.registerRuntime(payload)
        toast.success(editing ? '运行时配置已更新' : '运行时已登记')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功，请检查声明后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [
      adapterLevel,
      capabilityImpl,
      code,
      eco,
      editing,
      form,
      name,
      onSaved,
      parsed,
      pluginRef,
      runtime?.id,
      status,
    ]
  )

  const levelHint = ADAPTER_LEVELS.find((item) => item.value === adapterLevel)?.hint
  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? '修改运行时配置' : '登记链运行时'}</ModalTitle>
          <ModalDescription>
            运行时声明使用字段编辑器维护，登记后还需镜像预拉取和自检。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex max-h-screen flex-col gap-4 overflow-y-auto">
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="运行时短名"
                htmlFor={`${fieldId}-code`}
                required
                error={errors.code}
              >
                <Input
                  id={`${fieldId}-code`}
                  value={code}
                  disabled={editing}
                  invalid={Boolean(errors.code)}
                  onChange={(event) => setCode(event.target.value)}
                />
              </FormField>
              <FormField
                label="运行时名称"
                htmlFor={`${fieldId}-name`}
                required
                error={errors.name}
              >
                <Input
                  id={`${fieldId}-name`}
                  value={name}
                  invalid={Boolean(errors.name)}
                  onChange={(event) => setName(event.target.value)}
                />
              </FormField>
              <FormField label="所属生态" htmlFor={`${fieldId}-eco`} required error={errors.eco}>
                <Input
                  id={`${fieldId}-eco`}
                  value={eco}
                  invalid={Boolean(errors.eco)}
                  onChange={(event) => setEco(event.target.value)}
                />
              </FormField>
              <FormField label="适配层级" htmlFor={`${fieldId}-level`} required helper={levelHint}>
                <Select
                  id={`${fieldId}-level`}
                  options={ADAPTER_LEVELS.map((item) => ({ value: item.value, label: item.label }))}
                  value={adapterLevel}
                  onValueChange={setAdapterLevel}
                />
              </FormField>
              <FormField label="链能力实现名称" htmlFor={`${fieldId}-impl`}>
                <Input
                  id={`${fieldId}-impl`}
                  value={capabilityImpl}
                  onChange={(event) => setCapabilityImpl(event.target.value)}
                />
              </FormField>
              <FormField label="链能力插件名称" htmlFor={`${fieldId}-plugin`}>
                <Input
                  id={`${fieldId}-plugin`}
                  value={pluginRef}
                  onChange={(event) => setPluginRef(event.target.value)}
                />
              </FormField>
            </div>
            <FormField label="登记状态" required helper="可用由自检通过后自动写入。">
              <SegmentedControl
                aria-label="登记状态"
                options={SUBMITTABLE_STATUSES.map((item) => ({
                  value: String(item),
                  label: runtimeStatusLabel(item),
                }))}
                value={status}
                onValueChange={setStatus}
              />
            </FormField>
            <section className="well flex flex-col gap-3 p-3">
              <h3 className="text-sm font-medium text-ink">主环境</h3>
              <div className="grid gap-3 sm:grid-cols-3">
                <FormField label="工作区目录" error={errors.workspaceDir}>
                  <Input
                    value={form.workspaceDir}
                    invalid={Boolean(errors.workspaceDir)}
                    onChange={(event) => updateForm('workspaceDir', event.target.value)}
                  />
                </FormField>
                <FormField label="容器名称" error={errors.container}>
                  <Input
                    value={form.containerName}
                    invalid={Boolean(errors.container)}
                    onChange={(event) => updateForm('containerName', event.target.value)}
                  />
                </FormField>
                <FormField label="镜像地址">
                  <Input
                    value={form.imageUrl}
                    onChange={(event) => updateForm('imageUrl', event.target.value)}
                  />
                </FormField>
              </div>
              <PortEditor
                ports={form.ports}
                error={errors.ports}
                onChange={(ports) => updateForm('ports', ports)}
              />
            </section>
            <section className="well flex flex-col gap-3 p-3">
              <h3 className="text-sm font-medium text-ink">附加组件</h3>
              <SidecarEditor
                sidecars={form.sidecars}
                error={errors.sidecars}
                onChange={(sidecars) => updateForm('sidecars', sidecars)}
              />
            </section>
            <section className="well flex flex-col gap-3 p-3">
              <h3 className="text-sm font-medium text-ink">工作区操作</h3>
              <fieldset
                className="flex flex-col gap-3"
                aria-describedby={
                  errors.workspaceOps ? `${fieldId}-workspace-ops-error` : undefined
                }
                aria-invalid={errors.workspaceOps ? true : undefined}
              >
                <legend className="sr-only">工作区操作命令列表</legend>
                <div className="grid gap-3 sm:grid-cols-2">
                  {WORKSPACE_OP_KEYS.map((key) => (
                    <ArgvEditor
                      key={key}
                      label={WORKSPACE_OP_LABELS[key]}
                      value={form.workspaceOps[key]}
                      onChange={(value) => updateOp(key, value)}
                    />
                  ))}
                </div>
                {errors.workspaceOps ? (
                  <p
                    id={`${fieldId}-workspace-ops-error`}
                    role="alert"
                    className="text-xs text-danger"
                  >
                    {errors.workspaceOps}
                  </p>
                ) : null}
              </fieldset>
            </section>
            <section className="well flex flex-col gap-3 p-3">
              <h3 className="text-sm font-medium text-ink">标准链能力命令</h3>
              <div className="grid gap-3 sm:grid-cols-2">
                {(['deploy', 'tx', 'query', 'reset'] as const).map((key) => (
                  <fieldset
                    key={key}
                    className="grid gap-2"
                    aria-describedby={
                      errors.capabilities ? `${fieldId}-${key}-capability-error` : undefined
                    }
                    aria-invalid={errors.capabilities ? true : undefined}
                  >
                    <legend className="text-sm font-medium text-ink">
                      {key === 'deploy'
                        ? '部署能力'
                        : key === 'tx'
                          ? '交易能力'
                          : key === 'query'
                            ? '查询能力'
                            : '重置能力'}
                    </legend>
                    <ArgvEditor
                      label="命令参数"
                      value={form.capabilities[key].command}
                      onChange={(value) => updateCapability(key, 'command', value)}
                    />
                    <FormField label="超时秒数">
                      <Input
                        type="number"
                        min={0}
                        value={form.capabilities[key].timeout}
                        onChange={(event) => updateCapability(key, 'timeout', event.target.value)}
                      />
                    </FormField>
                    {errors.capabilities ? (
                      <p
                        id={`${fieldId}-${key}-capability-error`}
                        role="alert"
                        className="text-xs text-danger"
                      >
                        {errors.capabilities}
                      </p>
                    ) : null}
                  </fieldset>
                ))}
              </div>
            </section>
            <section className="well flex flex-col gap-3 p-3">
              <h3 className="text-sm font-medium text-ink">数据卷域</h3>
              <VolumeEditor
                volumes={form.volumes}
                onChange={(volumes) => updateForm('volumes', volumes)}
              />
            </section>
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={submitting}>
              {editing ? '保存声明' : '登记运行时'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

const CODE_PATTERN = /^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/

function PortEditor({
  ports,
  error,
  onChange,
}: {
  ports: PortState[]
  error?: string | null
  onChange: (ports: PortState[]) => void
}) {
  const errorId = 'runtime-ports-error'

  return (
    <fieldset
      className="flex flex-col gap-2"
      aria-describedby={error ? errorId : undefined}
      aria-invalid={error ? true : undefined}
    >
      <legend className="sr-only">对外端口</legend>
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted">对外端口</span>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() =>
            onChange([...ports, { name: '', containerPort: '', servicePort: '', protocol: 'TCP' }])
          }
        >
          添加端口
        </Button>
      </div>
      {ports.map((port, index) => (
        <div key={`port-${index}`} className="grid gap-2 sm:grid-cols-5">
          <Input
            aria-label="端口名称"
            value={port.name}
            placeholder="rpc"
            onChange={(event) =>
              onChange(
                ports.map((item, i) => (i === index ? { ...item, name: event.target.value } : item))
              )
            }
          />
          <Input
            aria-label="容器端口"
            type="number"
            value={port.containerPort}
            placeholder="8545"
            onChange={(event) =>
              onChange(
                ports.map((item, i) =>
                  i === index ? { ...item, containerPort: event.target.value } : item
                )
              )
            }
          />
          <Input
            aria-label="服务端口"
            type="number"
            value={port.servicePort}
            placeholder="8545"
            onChange={(event) =>
              onChange(
                ports.map((item, i) =>
                  i === index ? { ...item, servicePort: event.target.value } : item
                )
              )
            }
          />
          <Input
            aria-label="协议"
            value={port.protocol}
            onChange={(event) =>
              onChange(
                ports.map((item, i) =>
                  i === index ? { ...item, protocol: event.target.value } : item
                )
              )
            }
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onChange(ports.filter((_, i) => i !== index))}
          >
            移除
          </Button>
        </div>
      ))}
      {error ? (
        <p id={errorId} role="alert" className="text-xs text-danger">
          {error}
        </p>
      ) : null}
    </fieldset>
  )
}

function ArgvEditor({
  label,
  value,
  onChange,
  error,
}: {
  label: string
  value: string[]
  onChange: (value: string[]) => void
  error?: string | null
}) {
  const fieldId = useId()
  const errorId = `${fieldId}-error`

  return (
    <fieldset
      className="flex min-w-0 flex-col gap-2"
      aria-describedby={error ? errorId : undefined}
      aria-invalid={error ? true : undefined}
    >
      <legend className="text-xs text-muted">{label}</legend>
      {value.map((item, index) => (
        <div key={`${label}-${index}`} className="flex items-center gap-2">
          <Input
            aria-label={`${label}第 ${index + 1} 个参数`}
            className="min-w-0 flex-1 font-mono text-sm"
            value={item}
            placeholder={index === 0 ? '命令' : '参数'}
            onChange={(event) =>
              onChange(
                value.map((part, partIndex) => (partIndex === index ? event.target.value : part))
              )
            }
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={value.length <= 1}
            onClick={() => onChange(value.filter((_, partIndex) => partIndex !== index))}
          >
            移除
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={() => onChange([...value, ''])}>
        添加参数
      </Button>
      {error ? (
        <p id={errorId} role="alert" className="text-xs text-danger">
          {error}
        </p>
      ) : null}
    </fieldset>
  )
}

function SidecarEditor({
  sidecars,
  error,
  onChange,
}: {
  sidecars: SidecarState[]
  error?: string | null
  onChange: (sidecars: SidecarState[]) => void
}) {
  const errorId = 'runtime-sidecars-error'

  return (
    <fieldset
      className="flex flex-col gap-2"
      aria-describedby={error ? errorId : undefined}
      aria-invalid={error ? true : undefined}
    >
      <legend className="sr-only">附加组件</legend>
      <div className="flex justify-end">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onChange([...sidecars, { name: '', imageUrl: '', prepullCommand: [''] }])}
        >
          添加组件
        </Button>
      </div>
      {sidecars.map((sidecar, index) => (
        <div key={`sidecar-${index}`} className="grid gap-2 sm:grid-cols-4">
          <Input
            aria-label="组件名称"
            value={sidecar.name}
            placeholder="节点服务"
            onChange={(event) =>
              onChange(
                sidecars.map((item, i) =>
                  i === index ? { ...item, name: event.target.value } : item
                )
              )
            }
          />
          <ArgvEditor
            label="预拉取命令"
            value={sidecar.prepullCommand}
            onChange={(prepullCommand) =>
              onChange(
                sidecars.map((item, i) => (i === index ? { ...item, prepullCommand } : item))
              )
            }
          />
          <Input
            aria-label="组件镜像"
            value={sidecar.imageUrl}
            placeholder="镜像地址"
            onChange={(event) =>
              onChange(
                sidecars.map((item, i) =>
                  i === index ? { ...item, imageUrl: event.target.value } : item
                )
              )
            }
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onChange(sidecars.filter((_, i) => i !== index))}
          >
            移除
          </Button>
        </div>
      ))}
      {error ? (
        <p id={errorId} role="alert" className="text-xs text-danger">
          {error}
        </p>
      ) : null}
    </fieldset>
  )
}

function VolumeEditor({
  volumes,
  onChange,
}: {
  volumes: VolumeState[]
  onChange: (volumes: VolumeState[]) => void
}) {
  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="sr-only">数据卷域</legend>
      <div className="flex justify-end">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() =>
            onChange([
              ...volumes,
              { name: '', mountPath: '', studentAccess: '', persistence: '', snapshotScope: '' },
            ])
          }
        >
          添加卷域
        </Button>
      </div>
      {volumes.map((volume, index) => (
        <div key={`volume-${index}`} className="grid gap-2 sm:grid-cols-2">
          <Input
            aria-label="卷域名称"
            value={volume.name}
            placeholder="workspace"
            onChange={(event) =>
              onChange(
                volumes.map((item, i) =>
                  i === index ? { ...item, name: event.target.value } : item
                )
              )
            }
          />
          <Input
            aria-label="挂载路径"
            value={volume.mountPath}
            placeholder="/workspace"
            onChange={(event) =>
              onChange(
                volumes.map((item, i) =>
                  i === index ? { ...item, mountPath: event.target.value } : item
                )
              )
            }
          />
          <Input
            aria-label="学生访问权限"
            value={volume.studentAccess}
            placeholder="read-write"
            onChange={(event) =>
              onChange(
                volumes.map((item, i) =>
                  i === index ? { ...item, studentAccess: event.target.value } : item
                )
              )
            }
          />
          <Input
            aria-label="持久化策略"
            value={volume.persistence}
            placeholder="ephemeral"
            onChange={(event) =>
              onChange(
                volumes.map((item, i) =>
                  i === index ? { ...item, persistence: event.target.value } : item
                )
              )
            }
          />
          <Input
            aria-label="快照范围"
            value={volume.snapshotScope}
            placeholder="workspace"
            onChange={(event) =>
              onChange(
                volumes.map((item, i) =>
                  i === index ? { ...item, snapshotScope: event.target.value } : item
                )
              )
            }
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onChange(volumes.filter((_, i) => i !== index))}
          >
            移除卷域
          </Button>
        </div>
      ))}
    </fieldset>
  )
}
