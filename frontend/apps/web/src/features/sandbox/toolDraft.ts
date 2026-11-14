// toolDraft.ts 实验工具抽屉的表单初值与提交声明(设计 §7.6:整行打开工具大抽屉,详情与修改同一处)。
// 工具按短名覆盖登记(后端 RegisterTool 是 upsert),所以「修改」就是用同一短名带完整声明再登记一次:
// 表单只编辑它认识的字段,其余声明(资源上限、探针、能力、预拉取命令、连接绑定等)从已有声明原样带回,
// 不因为改了一个名称就被清空。
// 表单表达不了的工具(基础设施组件、目录同步写入的停用组件、多组件或多端口的网页工具)只能查看,
// 由 toolEditBlocker 说明原因。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 工具类型取值,与 @chaimir/api-client 的 SandboxToolKind 一致 */
const KIND_BUILTIN = 1
const KIND_WEB_EMBED = 3
const KIND_COMMAND = 4
const KIND_INFRA = 5
/** 状态取值,与 ToolStatus 一致 */
const STATUS_AVAILABLE = 1

type Spec = Record<string, unknown>

/** 抽屉要用到的工具字段(与接口返回的工具定义同形) */
export interface ToolLike {
  code: string
  name: string
  kind: number
  eco_tags: string[]
  status: number
  /** 工具声明(接口的 SandboxToolResourceSpec 也可直接传入) */
  resource_spec: object
}

/** 表单的全部字段(文本形态) */
export interface ToolDraft {
  code: string
  name: string
  kind: string
  ecoTags: string
  status: string
  builtinEndpoint: string
  allowedArgv: string[][]
  defaultTimeout: string
  maxTimeout: string
  componentName: string
  componentImage: string
  componentCommand: string[]
  webComponentName: string
  webComponentImage: string
  webPortName: string
  webContainerPort: string
  webServicePort: string
  webServiceName: string
  webRoutePrefix: string
}

/** asRecord 是对象时读成对象,否则为 undefined */
const asRecord = (value: unknown): Spec | undefined =>
  typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as Spec) : undefined
/** asList 是数组时原样读出,否则为空数组 */
const asList = (value: unknown): unknown[] => (Array.isArray(value) ? value : [])
/** text 文字原样读出,否则用给定的默认值 */
const text = (value: unknown, fallback = ''): string =>
  typeof value === 'string' ? value : fallback
/** numberText 有限数值写成文字,否则用给定的默认值 */
const numberText = (value: unknown, fallback: string): string =>
  typeof value === 'number' && Number.isFinite(value) ? String(value) : fallback
/** stringList 只保留数组里的文字 */
const stringList = (value: unknown): string[] =>
  asList(value).filter((item): item is string => typeof item === 'string')

/** toolEditBlocker 这条工具不能在抽屉里改时返回原因;能改时返回 undefined */
export function toolEditBlocker(tool: ToolLike): string | undefined {
  if (tool.kind === KIND_INFRA) return '基础设施组件由平台目录同步维护,这里只能查看'
  const spec = tool.resource_spec as Spec
  if (text(spec.disabled_reason).trim() !== '') {
    return '这个组件由平台目录同步维护,当前不可部署,这里只能查看'
  }
  if (tool.kind === KIND_WEB_EMBED) {
    const components = asList(spec.components)
    const ports = asList(asRecord(components[0])?.ports)
    if (
      components.length !== 1 ||
      ports.length !== 1 ||
      asList(spec.services).length !== 1 ||
      asList(spec.routes).length !== 1 ||
      asList(spec.network_rules).length > 0
    ) {
      return '这个工具的声明超出了表单能编辑的范围(多个组件、端口、路由或网络规则),这里只能查看'
    }
  }
  return undefined
}

/** toolDraft 从已有工具读回表单值;新登记时给各类型的默认值 */
export function toolDraft(tool: ToolLike | undefined, builtinPrefix = ''): ToolDraft {
  const spec = (tool?.resource_spec ?? {}) as Spec
  const component = asRecord(asList(spec.components)[0])
  const port = asRecord(asList(component?.ports)[0])
  const service = asRecord(asList(spec.services)[0])
  const route = asRecord(asList(spec.routes)[0])
  const policy = asRecord(spec.command_policy)
  const argv = asList(policy?.allowed_argv)
    .map(stringList)
    .filter((row) => row.length > 0)
  const command = stringList(component?.command)
  const isCommand = tool?.kind === KIND_COMMAND
  const isWeb = tool?.kind === KIND_WEB_EMBED
  return {
    code: tool?.code ?? '',
    name: tool?.name ?? '',
    kind: String(tool?.kind ?? KIND_COMMAND),
    ecoTags: (tool?.eco_tags ?? []).join(', '),
    status: String(tool?.status ?? STATUS_AVAILABLE),
    builtinEndpoint:
      tool?.kind === KIND_BUILTIN ? text(spec.builtin_endpoint, builtinPrefix) : builtinPrefix,
    allowedArgv: argv.length > 0 ? argv : [['']],
    defaultTimeout: numberText(policy?.default_timeout_seconds, '30'),
    maxTimeout: numberText(policy?.max_timeout_seconds, '120'),
    componentName: isCommand ? text(component?.name, 'compiler') : 'compiler',
    componentImage: isCommand ? text(component?.image_url) : '',
    componentCommand: isCommand && command.length > 0 ? command : ['sleep'],
    webComponentName: isWeb ? text(component?.name, 'editor') : 'editor',
    webComponentImage: isWeb ? text(component?.image_url) : '',
    webPortName: isWeb ? text(port?.name, 'http') : 'http',
    webContainerPort: isWeb ? numberText(port?.container_port, '8080') : '8080',
    webServicePort: isWeb ? numberText(port?.service_port, '8080') : '8080',
    webServiceName: isWeb ? text(service?.name, 'editor') : 'editor',
    webRoutePrefix: isWeb ? text(route?.path_prefix, '/') : '/',
  }
}

/** trimmed 去掉每项首尾空白并丢掉空项 */
const trimmed = (items: string[]) => items.map((item) => item.trim()).filter((item) => item !== '')

/**
 * toolResourceSpec 按类型组装要提交的声明。base 是已有工具的声明:表单编辑的字段覆盖进去,其余原样保留;
 * 新登记时没有 base,只带该类型必需的字段(后端对每类工具都有禁填项,不带多余字段)。
 */
export function toolResourceSpec(draft: ToolDraft, base?: Spec): Spec {
  const kind = Number(draft.kind)
  const kept: Spec = { ...(base ?? {}) }
  delete kept.disabled_reason
  if (kind === KIND_BUILTIN) return { ...kept, builtin_endpoint: draft.builtinEndpoint.trim() }
  if (kind === KIND_COMMAND) {
    const component = asRecord(asList(kept.components)[0]) ?? { mount_workspace: true }
    return {
      ...kept,
      components: [
        {
          ...component,
          name: draft.componentName.trim(),
          image_url: draft.componentImage.trim(),
          command: trimmed(draft.componentCommand),
        },
      ],
      command_policy: {
        allowed_argv: draft.allowedArgv.map(trimmed),
        default_timeout_seconds: Number(draft.defaultTimeout),
        max_timeout_seconds: Number(draft.maxTimeout),
      },
    }
  }
  if (kind === KIND_WEB_EMBED) {
    const component = asRecord(asList(kept.components)[0]) ?? { mount_workspace: true }
    const componentPort = asRecord(asList(component.ports)[0]) ?? { protocol: 'TCP' }
    const service = asRecord(asList(kept.services)[0]) ?? {}
    const servicePort = asRecord(asList(service.ports)[0]) ?? { protocol: 'TCP' }
    const route = asRecord(asList(kept.routes)[0]) ?? {}
    const portName = draft.webPortName.trim()
    const serviceName = draft.webServiceName.trim()
    const componentName = draft.webComponentName.trim()
    return {
      ...kept,
      components: [
        {
          ...component,
          name: componentName,
          image_url: draft.webComponentImage.trim(),
          ports: [
            {
              ...componentPort,
              name: portName,
              container_port: Number(draft.webContainerPort),
              service_port: Number(draft.webServicePort),
            },
          ],
        },
      ],
      services: [
        {
          ...service,
          name: serviceName,
          component: componentName,
          ports: [
            {
              ...servicePort,
              name: portName,
              port: Number(draft.webServicePort),
              target_port: portName,
            },
          ],
        },
      ],
      routes: [
        {
          ...route,
          path_prefix: draft.webRoutePrefix.trim(),
          service: serviceName,
          port: portName,
        },
      ],
    }
  }
  // 终端工具不带任何工作负载声明(后端禁填),已有的其他声明原样保留
  return kept
}
