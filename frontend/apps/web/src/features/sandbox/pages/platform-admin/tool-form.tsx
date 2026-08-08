// 沙箱工具登记表单(沙箱工具页内)。
//
// 四类工具的声明形态互斥,后端 validateToolResourceSpecShape 对每类都有必填与禁填:
//   平台内置:只要入口地址模板,不能带容器、命令策略或网络规则;
//   终端:什么都不要,带任何声明都会被拒;
//   网页工具:容器、服务、代理路由三者都必须有,不能带命令策略;
//   命令工具:恰好一个容器 + 命令白名单,不能有服务、路由或网络规则。
// 故表单按类型分叉,只渲染该类型允许的字段 —— 把四类字段并排摊出来,填错的概率高于填对。
//
// 容器声明本身是不定长的编排清单(镜像、端口、探针、挂载),键不可枚举,
// 用文档编辑器并在本地做结构校验;命令白名单与超时是可枚举的,做成结构化字段。

import { useCallback, useId, useMemo, useState } from 'react'
import {
  SandboxToolKind,
  ToolStatus,
  type SandboxToolRequest,
  type SandboxToolResourceSpec,
  type WorkloadComponent,
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
import {
  SANDBOX_TOOL_KINDS,
  TOOL_STATUSES,
  sandboxToolKindLabel,
  toolStatusLabel,
} from '../../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 工具编码规则,与后端 codePattern 一致。 */
const CODE_PATTERN = /^[a-z][a-z0-9-]{0,30}[a-z0-9]$/

/** 内置工具入口模板的固定前缀,与后端 validBuiltinEndpointTemplate 一致。 */
const BUILTIN_ENDPOINT_PREFIX = '/api/v1/sandbox/sandboxes/{sandbox_id}'

/** 命令工具的单容器清单骨架:恰好一个容器,不声明端口。 */
const COMMAND_COMPONENT_TEMPLATE = `[
  {
    "name": "compiler",
    "image_url": "",
    "command": ["sleep", "infinity"],
    "mount_workspace": true
  }
]`

/** 网页工具的清单骨架:容器 + 服务 + 代理路由三段都必须有。 */
const WEB_EMBED_TEMPLATE = `{
  "components": [
    {
      "name": "editor",
      "image_url": "",
      "ports": [
        { "name": "http", "container_port": 8080, "service_port": 8080, "protocol": "TCP" }
      ],
      "mount_workspace": true
    }
  ],
  "services": [
    {
      "name": "editor",
      "component": "editor",
      "ports": [{ "name": "http", "port": 8080, "target_port": "http", "protocol": "TCP" }]
    }
  ],
  "routes": [{ "path_prefix": "/", "service": "editor", "port": "http" }]
}`

export interface ToolFormModalProps {
  onClose: () => void
  onSaved: () => void
}

/**
 * ToolFormModal 承载沙箱工具的登记。
 * 后端是同一条 POST 做创建与覆盖(按编码 upsert),故这里只有一个登记入口,
 * 修改已有工具就是用同一个编码再登记一次 —— 表单里明确说明这一点。
 */
export function ToolFormModal({ onClose, onSaved }: ToolFormModalProps) {
  const fieldId = useId()

  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [kind, setKind] = useState(String(SandboxToolKind.COMMAND))
  const [ecoTags, setEcoTags] = useState('')
  const [status, setStatus] = useState(String(ToolStatus.AVAILABLE))

  const [builtinEndpoint, setBuiltinEndpoint] = useState(BUILTIN_ENDPOINT_PREFIX)
  const [allowedCommands, setAllowedCommands] = useState('')
  const [defaultTimeout, setDefaultTimeout] = useState('30')
  const [maxTimeout, setMaxTimeout] = useState('120')
  const [componentsText, setComponentsText] = useState(COMMAND_COMPONENT_TEMPLATE)
  const [webSpecText, setWebSpecText] = useState(WEB_EMBED_TEMPLATE)

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const kindValue = Number(kind) as SandboxToolKind

  const commandList = useMemo(
    () =>
      allowedCommands
        .split(/[\s,，]+/)
        .map((item) => item.trim())
        .filter((item) => item !== ''),
    [allowedCommands],
  )

  const componentsParsed = useMemo(() => parseComponents(componentsText), [componentsText])
  const webParsed = useMemo(() => parseWebEmbedSpec(webSpecText), [webSpecText])

  /** buildResourceSpec 按类型只组装该类型允许的字段,其余一律不带。 */
  const buildResourceSpec = useCallback((): SandboxToolResourceSpec => {
    switch (kindValue) {
      case SandboxToolKind.BUILTIN:
        return { builtin_endpoint: builtinEndpoint.trim() }
      case SandboxToolKind.COMMAND:
        return {
          components: componentsParsed.components,
          command_policy: {
            allowed_commands: commandList,
            default_timeout_seconds: Number(defaultTimeout),
            max_timeout_seconds: Number(maxTimeout),
          },
        }
      case SandboxToolKind.WEB_EMBED:
        return webParsed.spec ?? {}
      default:
        // 终端工具不带任何声明:后端对它禁填容器、入口模板、命令策略与网络规则
        return {}
    }
  }, [
    builtinEndpoint,
    commandList,
    componentsParsed.components,
    defaultTimeout,
    kindValue,
    maxTimeout,
    webParsed.spec,
  ])

  /** validate 按类型逐项校验,校验口径与后端 validateToolResourceSpecShape 对齐。 */
  const validate = useCallback((): boolean => {
    const next: Record<string, string | null> = {
      code: CODE_PATTERN.test(code.trim())
        ? null
        : '用小写字母开头,只含小写字母、数字与连字符,长度 2 到 32 位',
      name: name.trim() === '' ? '请输入工具名称' : null,
      builtinEndpoint: null,
      allowedCommands: null,
      timeout: null,
      components: null,
      webSpec: null,
    }

    if (kindValue === SandboxToolKind.BUILTIN) {
      const endpoint = builtinEndpoint.trim()
      next.builtinEndpoint =
        endpoint.startsWith(BUILTIN_ENDPOINT_PREFIX) && !endpoint.includes('://')
          ? null
          : `入口模板必须以 ${BUILTIN_ENDPOINT_PREFIX} 开头,并且不能是完整网址`
    }

    if (kindValue === SandboxToolKind.COMMAND) {
      const defaultValue = Number(defaultTimeout)
      const maxValue = Number(maxTimeout)
      next.allowedCommands =
        commandList.length === 0
          ? '至少写一个允许执行的命令'
          : commandList.some((item) => item.includes('/') || item.includes('\\'))
            ? '命令只写名字,不要带路径'
            : new Set(commandList).size !== commandList.length
              ? '有重复的命令,去掉重复项'
              : null
      next.timeout =
        !Number.isInteger(defaultValue) ||
        !Number.isInteger(maxValue) ||
        defaultValue <= 0 ||
        maxValue <= 0
          ? '两个超时都要是大于 0 的整数秒'
          : defaultValue > maxValue
            ? '默认超时不能大于最长超时'
            : null
      next.components = componentsParsed.error ?? null
    }

    if (kindValue === SandboxToolKind.WEB_EMBED) {
      next.webSpec = webParsed.error ?? null
    }

    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [
    builtinEndpoint,
    code,
    commandList,
    componentsParsed.error,
    defaultTimeout,
    kindValue,
    maxTimeout,
    name,
    webParsed.error,
  ])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) {
        setFormError('有几项还不能提交,按提示改一下。')
        return
      }
      setFormError(undefined)
      setSubmitting(true)
      try {
        const payload: SandboxToolRequest = {
          code: code.trim(),
          name: name.trim(),
          kind: kindValue,
          eco_tags: ecoTags
            .split(/[\s,，]+/)
            .map((item) => item.trim())
            .filter((item) => item !== ''),
          resource_spec: buildResourceSpec(),
          status: Number(status) as ToolStatus,
        }
        await api.sandbox.registerTool(payload)
        toast.success('工具已登记')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '登记没有成功,请检查声明后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [buildResourceSpec, code, ecoTags, kindValue, name, onSaved, status, validate],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>登记沙箱工具</ModalTitle>
          <ModalDescription>
            用已存在的短名再登记一次就是覆盖原定义。改动对之后创建的环境生效。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="工具短名"
                htmlFor={`${fieldId}-code`}
                required
                error={errors.code}
                helper="平台内唯一短名,例如 code-server"
              >
                <Input
                  id={`${fieldId}-code`}
                  value={code}
                  placeholder="code-server"
                  invalid={Boolean(errors.code)}
                  onChange={(event) => setCode(event.target.value)}
                />
              </FormField>
              <FormField label="工具名称" htmlFor={`${fieldId}-name`} required error={errors.name}>
                <Input
                  id={`${fieldId}-name`}
                  value={name}
                  placeholder="在线代码编辑器"
                  invalid={Boolean(errors.name)}
                  onChange={(event) => setName(event.target.value)}
                />
              </FormField>
            </div>

            <FormField label="工具类型" htmlFor={`${fieldId}-kind`} required helper={KIND_HINTS[kindValue]}>
              <Select
                id={`${fieldId}-kind`}
                options={SANDBOX_TOOL_KINDS.map((item) => ({
                  value: String(item),
                  label: sandboxToolKindLabel(item),
                }))}
                value={kind}
                onValueChange={setKind}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="适用生态"
                htmlFor={`${fieldId}-eco`}
                helper="留空表示不限生态。多个用逗号或空格分隔"
              >
                <Input
                  id={`${fieldId}-eco`}
                  value={ecoTags}
                  placeholder="ethereum, fabric"
                  onChange={(event) => setEcoTags(event.target.value)}
                />
              </FormField>
              <FormField label="登记状态" required helper="停用的工具不会出现在学生工作台">
                <SegmentedControl
                  aria-label="登记状态"
                  options={TOOL_STATUSES.map((item) => ({
                    value: String(item),
                    label: toolStatusLabel(item),
                  }))}
                  value={status}
                  onValueChange={setStatus}
                />
              </FormField>
            </div>

            {kindValue === SandboxToolKind.BUILTIN ? (
              <FormField
                label="入口地址模板"
                htmlFor={`${fieldId}-endpoint`}
                required
                error={errors.builtinEndpoint}
                helper="平台会自动替换学生实验环境编号并生成真实入口"
              >
                <Input
                  id={`${fieldId}-endpoint`}
                  className="font-mono text-sm"
                  value={builtinEndpoint}
                  invalid={Boolean(errors.builtinEndpoint)}
                  onChange={(event) => setBuiltinEndpoint(event.target.value)}
                />
              </FormField>
            ) : null}

            {kindValue === SandboxToolKind.TERMINAL ? (
              <Callout tone="info" title="终端工具不需要额外声明">
                终端直连运行时容器,用的是运行时声明里的终端命令。填了容器或命令策略反而会被拒绝。
              </Callout>
            ) : null}

            {kindValue === SandboxToolKind.COMMAND ? (
              <>
                <FormField
                  label="允许执行的命令"
                  htmlFor={`${fieldId}-commands`}
                  required
                  error={errors.allowedCommands}
                  helper="只写命令名不带路径,多个用逗号或空格分隔。白名单之外的命令一律拒绝执行"
                >
                  <Input
                    id={`${fieldId}-commands`}
                    className="font-mono text-sm"
                    value={allowedCommands}
                    placeholder="solc, forge, cast"
                    invalid={Boolean(errors.allowedCommands)}
                    onChange={(event) => setAllowedCommands(event.target.value)}
                  />
                </FormField>

                {commandList.length > 0 ? (
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="text-sm text-ink-sub">将放行:</span>
                    {commandList.map((item) => (
                      <Badge key={item} tone="jade">
                        {item}
                      </Badge>
                    ))}
                  </div>
                ) : null}

                <div className="grid gap-4 sm:grid-cols-2">
                  <FormField
                    label="默认超时(秒)"
                    htmlFor={`${fieldId}-default-timeout`}
                    required
                    error={errors.timeout}
                    helper="调用方没有指定超时时用这个值"
                  >
                    <Input
                      id={`${fieldId}-default-timeout`}
                      type="number"
                      min={1}
                      value={defaultTimeout}
                      invalid={Boolean(errors.timeout)}
                      onChange={(event) => setDefaultTimeout(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="最长超时(秒)"
                    htmlFor={`${fieldId}-max-timeout`}
                    required
                    helper="调用方指定的超时不能超过这个值"
                  >
                    <Input
                      id={`${fieldId}-max-timeout`}
                      type="number"
                      min={1}
                      value={maxTimeout}
                      invalid={Boolean(errors.timeout)}
                      onChange={(event) => setMaxTimeout(event.target.value)}
                    />
                  </FormField>
                </div>

                <FormField
                  label="执行环境配置"
                  htmlFor={`${fieldId}-components`}
                  required
                  error={errors.components}
                  helper="恰好一个容器。命令工具不对外暴露端口,也不配代理路由"
                >
                  <Textarea
                    id={`${fieldId}-components`}
                    className="font-mono text-sm"
                    value={componentsText}
                    rows={12}
                    spellCheck={false}
                    invalid={Boolean(errors.components)}
                    onChange={(event) => setComponentsText(event.target.value)}
                  />
                </FormField>

                {componentsParsed.components.length === 1 ? (
                  <DescriptionList
                    dense
                    columns={2}
                    items={[
                      {
                        term: '容器名',
                        description: readComponentField(componentsParsed.components[0], 'name'),
                        mono: true,
                      },
                      {
                        term: '容器镜像',
                        description:
                          readComponentField(componentsParsed.components[0], 'image_url') || '未填写',
                        mono: true,
                      },
                    ]}
                  />
                ) : null}
              </>
            ) : null}

            {kindValue === SandboxToolKind.WEB_EMBED ? (
              <>
                <FormField
                  label="网页工具配置"
                  htmlFor={`${fieldId}-web-spec`}
                  required
                  error={errors.webSpec}
                  helper="容器、服务、代理路由三段都要有:平台按路由把工具页面代理进工作台"
                >
                  <Textarea
                    id={`${fieldId}-web-spec`}
                    className="font-mono text-sm"
                    value={webSpecText}
                    rows={16}
                    spellCheck={false}
                    invalid={Boolean(errors.webSpec)}
                    onChange={(event) => setWebSpecText(event.target.value)}
                  />
                </FormField>

                {webParsed.summary ? (
                  <DescriptionList
                    dense
                    columns={3}
                    items={[
                      { term: '容器', description: `${webParsed.summary.components} 个` },
                      { term: '服务', description: `${webParsed.summary.services} 个` },
                      { term: '代理路由', description: `${webParsed.summary.routes} 条` },
                    ]}
                  />
                ) : null}
              </>
            ) : null}

            <Callout tone="info">
              容器镜像必须来自平台私有仓库并通过签名与漏洞扫描,登记时后端会校验。
            </Callout>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={submitting}>
              登记工具
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** 各类工具的选型说明,与列表页同源口径。 */
const KIND_HINTS: Record<SandboxToolKind, string> = {
  [SandboxToolKind.BUILTIN]: '平台自带的面板,只需要入口地址模板',
  [SandboxToolKind.TERMINAL]: '直连运行时容器的终端,不需要任何额外声明',
  [SandboxToolKind.WEB_EMBED]: '独立容器提供网页界面,要声明容器、服务与代理路由',
  [SandboxToolKind.COMMAND]: '在白名单内执行命令,要声明一个执行容器与允许的命令',
}

/** ParsedComponents 是命令工具容器清单的解析结果。 */
interface ParsedComponents {
  components: WorkloadComponent[]
  error?: string
}

/**
 * parseComponents 解析命令工具的容器清单。
 * 后端要求恰好一个容器且每个容器有名字,这两条能在本地判定,故在此挡住。
 */
function parseComponents(text: string): ParsedComponents {
  const trimmed = text.trim()
  if (trimmed === '') return { components: [], error: '请填写执行环境配置。' }

  let raw: unknown
  try {
    raw = JSON.parse(trimmed)
  } catch {
    return { components: [], error: '内容不是合法的声明格式,检查是否漏了逗号或引号。' }
  }
  if (!Array.isArray(raw)) return { components: [], error: '执行环境配置格式不正确,请按示例填写多个条目。' }
  if (raw.length !== 1) {
    return { components: [], error: '命令工具只能声明一个执行容器。' }
  }
  const first = asRecord(raw[0])
  if (!first || typeof first.name !== 'string' || first.name.trim() === '') {
    return { components: [], error: '容器要有名字(name)。' }
  }
  return { components: raw as WorkloadComponent[] }
}

/** ParsedWebEmbed 是网页工具清单的解析结果。 */
interface ParsedWebEmbed {
  spec?: SandboxToolResourceSpec
  summary?: { components: number; services: number; routes: number }
  error?: string
}

/**
 * parseWebEmbedSpec 解析网页工具清单。
 * 后端要求容器、服务、代理路由三段都非空,且不能带命令策略;这几条在本地判定。
 */
function parseWebEmbedSpec(text: string): ParsedWebEmbed {
  const trimmed = text.trim()
  if (trimmed === '') return { error: '请填写网页工具配置。' }

  let raw: unknown
  try {
    raw = JSON.parse(trimmed)
  } catch {
    return { error: '内容不是合法的声明格式,检查是否漏了逗号或引号。' }
  }
  const spec = asRecord(raw)
  if (!spec) return { error: '清单最外层要是一个对象。' }

  const components = Array.isArray(spec.components) ? spec.components : []
  const services = Array.isArray(spec.services) ? spec.services : []
  const routes = Array.isArray(spec.routes) ? spec.routes : []

  if (components.length === 0) return { error: '至少要声明一个容器。' }
  if (services.length === 0) return { error: '至少要声明一个服务,否则平台无法访问容器端口。' }
  if (routes.length === 0) return { error: '至少要声明一条代理路由,否则工具页面无法嵌入工作台。' }
  if (spec.command_policy !== undefined) {
    return { error: '网页工具不能带命令白名单,那是命令工具的字段。' }
  }

  return {
    spec: spec as unknown as SandboxToolResourceSpec,
    summary: { components: components.length, services: services.length, routes: routes.length },
  }
}

/** readComponentField 读容器声明里的字符串字段,用于摘要展示。 */
function readComponentField(component: WorkloadComponent, key: 'name' | 'image_url'): string {
  const value = component[key]
  return typeof value === 'string' ? value : ''
}

/** asRecord 把未知值收敛成对象;非对象回 undefined,调用方按缺失处理。 */
function asRecord(value: unknown): Record<string, unknown> | undefined {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined
}
