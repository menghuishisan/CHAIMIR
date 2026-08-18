// 运行时声明表单(链运行时页内)。
//
// 运行时的 adapter_spec 是声明式容器编排清单:主环境、附加组件、端口、探针、卷域、
// 服务与网络规则都是不定长数组,键不可枚举。把它拆成表单会得到一个几百字段仍不完备的
// 界面,故这里的分工是 ——
//   可枚举的顶层字段(编码/名称/生态/适配层级/能力实现/状态)做结构化表单;
//   清单本身用文档编辑器,但前端按后端 normalizeAndValidateAdapterSpec 的必要条件做结构校验
//   (JSON 合法、工作区目录绝对路径、八个工作区操作命令齐备、主环境有名字和端口),
//   并把解析结果摊成可读摘要 —— 让人在保存前看见系统读到了什么,而不是提交后才知道。
//
// 状态只提供「接入中 / 已停用」:后端 validateRuntimeRequest 拒绝把状态直接设为可用,
// 可用是自检通过后由系统写入的结果,不是能手填的字段。

import { useCallback, useId, useMemo, useState } from 'react'
import {
  RuntimeAdapterLevel,
  RuntimeStatus,
  type SandboxAdapterSpec,
  type SandboxRuntime,
  type SandboxRuntimeRequest,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  DescriptionList,
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
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { runtimeStatusLabel } from '../../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { asRecord } from '../../runtimeSpec'

/** 适配层级:1 只托管环境,2 提供标准链能力,3 自带插件实现。 */
const ADAPTER_LEVELS = [
  {
    value: String(RuntimeAdapterLevel.HOSTED),
    label: '一级 · 只托管环境',
    hint: '平台只负责启动环境,链操作由学生自己在终端完成',
  },
  {
    value: String(RuntimeAdapterLevel.STANDARD),
    label: '二级 · 声明标准链能力',
    hint: '用命令声明部署/交易/查询/重置,平台统一调用',
  },
  {
    value: String(RuntimeAdapterLevel.PLUGIN),
    label: '三级 · 自带插件实现',
    hint: '链能力由运行时插件实现,平台按插件名称调用',
  },
] as const

/** 可提交的状态:可用由自检结果决定,不接受手填(后端 validateRuntimeRequest)。 */
const SUBMITTABLE_STATUSES = [RuntimeStatus.ONBOARDING, RuntimeStatus.DISABLED] as const

/** 工作区操作的八个命令键,与后端 validateWorkspaceOps 逐项校验的字段一致。 */
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

/** 新建时给出的最小可用清单骨架:照这个结构填,比从空白开始少踩键名错误。 */
const SPEC_TEMPLATE = `{
  "workspace_dir": "/workspace",
  "runtime_container": {
    "name": "chain",
    "image_url": "",
    "ports": [
      { "name": "rpc", "container_port": 8545, "service_port": 8545, "protocol": "TCP" }
    ]
  },
  "workspace_ops": {
    "read_file": ["cat"],
    "write_file": ["tee"],
    "list_files": ["ls", "-l"],
    "pack_tar": ["tar", "-cf", "-"],
    "unpack_tar": ["tar", "-xf", "-"],
    "run_script": ["sh"],
    "terminal": ["sh"],
    "selftest": ["true"]
  }
}`

export interface RuntimeFormModalProps {
  /** 传入即为修改模式;缺省为新增。修改时编码不可变(后端按路径 ID 校验 code 一致) */
  runtime?: SandboxRuntime
  onClose: () => void
  onSaved: () => void
}

/**
 * RuntimeFormModal 承载链运行时的登记与修改。
 */
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
  const [specText, setSpecText] = useState(() =>
    runtime ? JSON.stringify(runtime.adapter_spec, null, 2) : SPEC_TEMPLATE
  )

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const parsed = useMemo(() => parseSpec(specText), [specText])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const next: Record<string, string | null> = {
        code: CODE_PATTERN.test(code.trim())
          ? null
          : '以小写字母或数字开头,只含小写字母、数字与连字符,长度 3 到 64 位',
        name: name.trim() === '' ? '请输入运行时名称' : null,
        eco: eco.trim() === '' ? '请输入所属生态,例如 ethereum' : null,
        spec: parsed.error ?? null,
      }
      setErrors(next)
      if (Object.values(next).some((value) => value !== null)) {
        setFormError('有几项还不能提交,按提示改一下。')
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
          adapter_spec: parsed.spec as SandboxAdapterSpec,
          capability_impl: capabilityImpl.trim(),
          plugin_ref: pluginRef.trim(),
          status: Number(status) as RuntimeStatus,
        }
        if (editing) await api.sandbox.updateRuntime(runtime.id, payload)
        else await api.sandbox.registerRuntime(payload)
        toast.success(editing ? '运行时配置已更新' : '运行时已登记,接下来登记镜像并预拉取')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请检查声明后重试。'))
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
            运行时描述一条链在平台里怎么跑起来。登记后还要登记镜像、预拉取、自检通过,才会变成可用。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="运行时短名"
                htmlFor={`${fieldId}-code`}
                required
                error={errors.code}
                helper={editing ? '登记后不能修改' : '平台内唯一短名,例如 ethereum-hardhat'}
              >
                <Input
                  id={`${fieldId}-code`}
                  value={code}
                  disabled={editing}
                  placeholder="ethereum-hardhat"
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
                  placeholder="以太坊测试链"
                  invalid={Boolean(errors.name)}
                  onChange={(event) => setName(event.target.value)}
                />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="所属生态"
                htmlFor={`${fieldId}-eco`}
                required
                error={errors.eco}
                helper="教师选运行时时按生态归类,例如 ethereum、fabric"
              >
                <Input
                  id={`${fieldId}-eco`}
                  value={eco}
                  placeholder="ethereum"
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
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="链能力实现名称"
                htmlFor={`${fieldId}-impl`}
                helper="二级以上需要指定能力来源:填写实现名称、插件名称,或在清单里声明四个能力命令"
              >
                <Input
                  id={`${fieldId}-impl`}
                  value={capabilityImpl}
                  placeholder="builtin-exec"
                  onChange={(event) => setCapabilityImpl(event.target.value)}
                />
              </FormField>
              <FormField
                label="链能力插件名称"
                htmlFor={`${fieldId}-plugin`}
                helper="三级运行时填写插件名称,平台会用它调用链能力"
              >
                <Input
                  id={`${fieldId}-plugin`}
                  value={pluginRef}
                  onChange={(event) => setPluginRef(event.target.value)}
                />
              </FormField>
            </div>

            <FormField
              label="登记状态"
              required
              helper="「可用」由自检通过后自动写入,这里只能选接入中或停用"
            >
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

            <FormField
              label="环境配置清单"
              htmlFor={`${fieldId}-spec`}
              required
              error={errors.spec}
              helper="组件、端口、卷域与工作区操作命令的完整声明。保存前会先在本地校验结构。"
            >
              <Textarea
                id={`${fieldId}-spec`}
                className="font-mono text-sm"
                value={specText}
                rows={16}
                spellCheck={false}
                invalid={Boolean(errors.spec)}
                onChange={(event) => setSpecText(event.target.value)}
              />
            </FormField>

            <SpecSummary summary={parsed.summary} error={parsed.error} />

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

/** 运行时编码规则,与后端 codePattern 一致。 */
const CODE_PATTERN = /^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/

/** SpecSummary 是清单解析后的可读摘要,让人在保存前确认系统读到了什么。 */
interface SpecSummaryData {
  workspaceDir: string
  containerName: string
  imageUrl: string
  portCount: number
  sidecarCount: number
  volumeCount: number
  hasCapabilityCommands: boolean
  missingOps: string[]
}

/**
 * SpecSummary 渲染清单摘要或解析失败提示。
 */
function SpecSummary({ summary, error }: { summary?: SpecSummaryData; error?: string }) {
  if (error || !summary) {
    return (
      <Callout tone="warning" title="清单还读不通">
        {error ?? '请先补全清单内容。'}
      </Callout>
    )
  }
  return (
    <div className="flex flex-col gap-3 well p-4">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-base text-ink">系统从清单里读到</span>
        {summary.hasCapabilityCommands ? (
          <Badge tone="jade">已声明四个标准链能力命令</Badge>
        ) : (
          <Badge tone="neutral">未声明链能力命令</Badge>
        )}
      </div>
      <DescriptionList
        dense
        columns={2}
        items={[
          { term: '工作区目录', description: summary.workspaceDir, mono: true },
          { term: '主环境', description: summary.containerName, mono: true },
          {
            term: '主环境镜像',
            description: summary.imageUrl || '按镜像版本登记时指定',
            mono: true,
          },
          { term: '对外端口', description: `${summary.portCount} 个` },
          { term: '附加组件', description: `${summary.sidecarCount} 个` },
          { term: '数据卷域', description: `${summary.volumeCount} 个` },
        ]}
      />
      {summary.missingOps.length > 0 ? (
        <Callout tone="warning">
          还缺这些工作区操作命令:{summary.missingOps.join('、')}。缺任意一项都无法读写学生的工作区。
        </Callout>
      ) : null}
    </div>
  )
}

/** ParsedSpec 是清单文本的解析结果:要么给出摘要,要么给出用户向的失败原因。 */
interface ParsedSpec {
  spec?: SandboxAdapterSpec
  summary?: SpecSummaryData
  error?: string
}

/**
 * parseSpec 解析并按后端必要条件校验声明清单。
 * 校验项对齐 normalizeAndValidateAdapterSpec 的硬性前提:JSON 合法、工作区目录为绝对路径、
 * 八个工作区操作命令齐备、主环境有名字且至少一个端口。其余细项(镜像签名、探针、网络规则)
 * 由后端最终判定 —— 前端只挡住能在本地确定的错误,不重复实现一套校验器。
 */
function parseSpec(text: string): ParsedSpec {
  const trimmed = text.trim()
  if (trimmed === '') return { error: '清单不能为空。' }

  let raw: unknown
  try {
    raw = JSON.parse(trimmed)
  } catch {
    return { error: '内容不是合法的声明格式,检查是否漏了逗号或引号。' }
  }
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
    return { error: '清单最外层要是一个对象。' }
  }
  const spec = raw as Record<string, unknown>

  const workspaceDir = typeof spec.workspace_dir === 'string' ? spec.workspace_dir.trim() : ''
  if (!workspaceDir.startsWith('/')) {
    return { error: '工作区目录要填环境内的绝对路径,例如 /workspace。' }
  }

  const container = asRecord(spec.runtime_container)
  const containerName = typeof container?.name === 'string' ? container.name.trim() : ''
  if (containerName === '') {
    return { error: '主环境要有名字(runtime_container.name)。' }
  }
  const ports = Array.isArray(container?.ports) ? container.ports : []
  if (ports.length === 0) {
    return { error: '主环境至少要声明一个对外端口,否则平台无法访问这条链。' }
  }

  const ops = asRecord(spec.workspace_ops)
  const missingOps = WORKSPACE_OP_KEYS.filter((key) => {
    const value = ops?.[key]
    return !Array.isArray(value) || value.length === 0
  }).map(workspaceOpLabel)

  const capability = asRecord(spec.capability_commands)
  const hasCapabilityCommands = ['deploy', 'tx', 'query', 'reset'].every((key) => {
    const command = asRecord(capability?.[key])?.command
    return Array.isArray(command) && command.length > 0
  })

  return {
    // 必要字段已在上面逐项确认(工作区目录、主环境与端口、八个操作命令),
    // 其余细项由后端最终判定,故此处按声明类型交给 SDK,不再复制一份类型定义。
    spec: spec as SandboxAdapterSpec,
    summary: {
      workspaceDir,
      containerName,
      imageUrl: typeof container?.image_url === 'string' ? container.image_url : '',
      portCount: ports.length,
      sidecarCount: Array.isArray(spec.infra_sidecars) ? spec.infra_sidecars.length : 0,
      volumeCount: Array.isArray(spec.volume_domains) ? spec.volume_domains.length : 0,
      hasCapabilityCommands,
      missingOps,
    },
    error: missingOps.length > 0 ? `缺少工作区操作命令:${missingOps.join('、')}` : undefined,
  }
}

/** 工作区操作的用户向名称:摘要与错误提示里不出现内部键名。 */
const WORKSPACE_OP_LABELS: Record<(typeof WORKSPACE_OP_KEYS)[number], string> = {
  read_file: '读文件',
  write_file: '写文件',
  list_files: '列目录',
  pack_tar: '打包工作区',
  unpack_tar: '解包到工作区',
  run_script: '执行脚本',
  terminal: '打开终端',
  selftest: '自检',
}

/** workspaceOpLabel 返回工作区操作名称。 */
function workspaceOpLabel(key: (typeof WORKSPACE_OP_KEYS)[number]): string {
  return WORKSPACE_OP_LABELS[key]
}
