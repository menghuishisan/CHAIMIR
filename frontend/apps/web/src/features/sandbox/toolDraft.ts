import { SandboxToolKind, ToolStatus, type SandboxToolRequest } from '@chaimir/api-client'
import { asRecord, stringArray, numberText, stringValue, type DynamicSpec } from './dynamicSpec.ts'

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
  prepullCommand: string[]
  webComponentName: string
  webComponentImage: string
  webPortName: string
  webContainerPort: string
  webServicePort: string
  webServiceName: string
  webRoutePrefix: string
}

const asList = (value: unknown): unknown[] => (Array.isArray(value) ? value : [])

export function toolEditBlocker(tool: SandboxToolRequest): string | undefined {
  if (tool.kind === SandboxToolKind.INFRA) return '基础设施组件由平台目录同步维护,这里只能查看'
  const spec = tool.resource_spec as DynamicSpec
  if (stringValue(spec.disabled_reason).trim() !== '') {
    return '这个组件由平台目录同步维护,当前不可部署,这里只能查看'
  }
  if (tool.kind === SandboxToolKind.WEB_EMBED) {
    const components = asList(spec.components)
    const ports = asList(asRecord(components[0])?.ports)
    if (
      components.length !== 1 ||
      ports.length !== 1 ||
      asList(spec.services).length !== 1 ||
      asList(asRecord(asList(spec.services)[0])?.ports).length !== 1 ||
      asList(spec.routes).length !== 1 ||
      asList(spec.network_rules).length > 0
    ) {
      return '这个工具的声明超出了表单能编辑的范围(多个组件、端口、路由或网络规则),这里只能查看'
    }
  }
  return undefined
}

export function toolDraft(tool: SandboxToolRequest | undefined, builtinPrefix = ''): ToolDraft {
  const spec = (tool?.resource_spec ?? {}) as DynamicSpec
  const component = asRecord(asList(spec.components)[0])
  const port = asRecord(asList(component?.ports)[0])
  const service = asRecord(asList(spec.services)[0])
  const servicePort = asRecord(asList(service?.ports)[0])
  const route = asRecord(asList(spec.routes)[0])
  const policy = asRecord(spec.command_policy)
  const argv = asList(policy?.allowed_argv)
    .map(stringArray)
    .filter((row) => row.length > 0)
  const command = stringArray(component?.command)
  const isCommand = tool?.kind === SandboxToolKind.COMMAND
  const isWeb = tool?.kind === SandboxToolKind.WEB_EMBED
  return {
    code: tool?.code ?? '',
    name: tool?.name ?? '',
    kind: String(tool?.kind ?? SandboxToolKind.COMMAND),
    ecoTags: (tool?.eco_tags ?? []).join(', '),
    status: String(tool?.status ?? ToolStatus.AVAILABLE),
    builtinEndpoint:
      tool?.kind === SandboxToolKind.BUILTIN
        ? stringValue(spec.builtin_endpoint, builtinPrefix)
        : builtinPrefix,
    allowedArgv: argv.length > 0 ? argv : [['']],
    defaultTimeout: numberText(policy?.default_timeout_seconds, '30'),
    maxTimeout: numberText(policy?.max_timeout_seconds, '120'),
    componentName: isCommand ? stringValue(component?.name, 'compiler') : 'compiler',
    componentImage: isCommand ? stringValue(component?.image_url) : '',
    componentCommand: isCommand && command.length > 0 ? command : ['sleep'],
    prepullCommand: stringArray(component?.prepull_command),
    webComponentName: isWeb ? stringValue(component?.name, 'editor') : 'editor',
    webComponentImage: isWeb ? stringValue(component?.image_url) : '',
    webPortName: isWeb ? stringValue(port?.name, 'http') : 'http',
    webContainerPort: isWeb ? numberText(port?.container_port, '8080') : '8080',
    webServicePort: isWeb ? numberText(servicePort?.port, '8080') : '8080',
    webServiceName: isWeb ? stringValue(service?.name, 'editor') : 'editor',
    webRoutePrefix: isWeb ? stringValue(route?.path_prefix, '/') : '/',
  }
}

// 保留参数位置，空项由表单与服务端拒绝。
const trimmed = (items: string[]) => items.map((item) => item.trim())

/** 覆盖表单管理的字段，保留既有工作负载的其余声明。 */
export function toolResourceSpec(draft: ToolDraft, base?: DynamicSpec): DynamicSpec {
  const kind = Number(draft.kind)
  const kept: DynamicSpec = { ...(base ?? {}) }
  delete kept.disabled_reason
  if (kind === SandboxToolKind.BUILTIN)
    return { ...kept, builtin_endpoint: draft.builtinEndpoint.trim() }
  if (kind === SandboxToolKind.COMMAND) {
    const component = asRecord(asList(kept.components)[0]) ?? { mount_workspace: true }
    return {
      ...kept,
      components: [
        {
          ...component,
          name: draft.componentName.trim(),
          image_url: draft.componentImage.trim(),
          command: trimmed(draft.componentCommand),
          prepull_command: trimmed(draft.prepullCommand),
        },
      ],
      command_policy: {
        allowed_argv: draft.allowedArgv.map(trimmed),
        default_timeout_seconds: Number(draft.defaultTimeout),
        max_timeout_seconds: Number(draft.maxTimeout),
      },
    }
  }
  if (kind === SandboxToolKind.WEB_EMBED) {
    const component = asRecord(asList(kept.components)[0]) ?? { mount_workspace: true }
    const componentPort = asRecord(asList(component.ports)[0]) ?? { protocol: 'TCP' }
    const service = asRecord(asList(kept.services)[0]) ?? {}
    const servicePort = asRecord(asList(service.ports)[0]) ?? { protocol: 'TCP' }
    const route = asRecord(asList(kept.routes)[0]) ?? {}
    const portName = draft.webPortName.trim()
    const servicePortName = stringValue(servicePort.name, portName)
    const serviceName = draft.webServiceName.trim()
    const componentName = draft.webComponentName.trim()
    return {
      ...kept,
      components: [
        {
          ...component,
          name: componentName,
          image_url: draft.webComponentImage.trim(),
          prepull_command: trimmed(draft.prepullCommand),
          ports: [
            {
              ...componentPort,
              name: portName,
              container_port: Number(draft.webContainerPort),
              service_port: componentPort.service_port ?? Number(draft.webServicePort),
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
              name: servicePortName,
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
          port: servicePortName,
        },
      ],
    }
  }
  return kept
}
