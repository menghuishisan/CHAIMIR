// toolDraft.test.ts 验证工具抽屉的表单初值:哪些工具能在这里改、改时从已有声明读回哪些字段。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { toolDraft, toolEditBlocker, toolResourceSpec } from './toolDraft.ts'
import { validCommand } from './dynamicSpec.ts'

// 取值与 @chaimir/api-client 一致:类型 1 内置 / 2 终端 / 3 网页 / 4 命令 / 5 基础设施
const BUILTIN = 1
const TERMINAL = 2
const WEB = 3
const COMMAND = 4
const INFRA = 5

const base = {
  code: 'code-server',
  name: '在线编辑器',
  eco_tags: ['ethereum', 'fabric'],
  status: 1,
}

test('基础设施组件与目录同步写入的停用组件不能在这里改', () => {
  assert.equal(
    toolEditBlocker({ ...base, kind: INFRA, resource_spec: { components: [{}] } }),
    '基础设施组件由平台目录同步维护,这里只能查看'
  )
  assert.equal(
    toolEditBlocker({
      ...base,
      kind: COMMAND,
      resource_spec: { disabled_reason: '镜像缺少签名证明' },
    }),
    '这个组件由平台目录同步维护,当前不可部署,这里只能查看'
  )
  assert.equal(toolEditBlocker({ ...base, kind: TERMINAL, resource_spec: {} }), undefined)
})

test('多组件、多端口或带网络规则的网页工具超出表单能表达的范围', () => {
  const web = (spec: Record<string, unknown>) => ({ ...base, kind: WEB, resource_spec: spec })
  assert.equal(
    toolEditBlocker(web({ components: [{}, {}], services: [{}], routes: [{}] })),
    '这个工具的声明超出了表单能编辑的范围(多个组件、端口、路由或网络规则),这里只能查看'
  )
  assert.equal(
    toolEditBlocker(web({ components: [{ ports: [{}, {}] }], services: [{}], routes: [{}] })),
    '这个工具的声明超出了表单能编辑的范围(多个组件、端口、路由或网络规则),这里只能查看'
  )
  assert.equal(
    toolEditBlocker(
      web({ components: [{ ports: [{}] }], services: [{}], routes: [{}], network_rules: [{}] })
    ),
    '这个工具的声明超出了表单能编辑的范围(多个组件、端口、路由或网络规则),这里只能查看'
  )
  assert.equal(
    toolEditBlocker(
      web({ components: [{ ports: [{}] }], services: [{ ports: [{}] }], routes: [{}] })
    ),
    undefined
  )
})

test('读回通用字段:生态用逗号连起来', () => {
  const draft = toolDraft({ ...base, kind: TERMINAL, resource_spec: {} })
  assert.equal(draft.code, 'code-server')
  assert.equal(draft.name, '在线编辑器')
  assert.equal(draft.kind, '2')
  assert.equal(draft.ecoTags, 'ethereum, fabric')
  assert.equal(draft.status, '1')
})

test('读回内置工具的入口模板', () => {
  const draft = toolDraft({
    ...base,
    kind: BUILTIN,
    resource_spec: { builtin_endpoint: '/panel-prefix/{id}/panel' },
  })
  assert.equal(draft.builtinEndpoint, '/panel-prefix/{id}/panel')
})

test('读回命令工具的白名单、超时与执行组件', () => {
  const draft = toolDraft({
    ...base,
    kind: COMMAND,
    resource_spec: {
      components: [
        { name: 'compiler', image_url: 'reg/solc@sha256:abc', command: ['sleep', 'infinity'] },
      ],
      command_policy: {
        allowed_argv: [['solc', '--version'], ['solc']],
        default_timeout_seconds: 20,
        max_timeout_seconds: 90,
      },
    },
  })
  assert.deepEqual(draft.allowedArgv, [['solc', '--version'], ['solc']])
  assert.equal(draft.defaultTimeout, '20')
  assert.equal(draft.maxTimeout, '90')
  assert.equal(draft.componentName, 'compiler')
  assert.equal(draft.componentImage, 'reg/solc@sha256:abc')
  assert.deepEqual(draft.componentCommand, ['sleep', 'infinity'])
})

test('读回网页工具的组件、端口、服务与路由', () => {
  const draft = toolDraft({
    ...base,
    kind: WEB,
    resource_spec: {
      components: [
        {
          name: 'editor',
          image_url: 'reg/cs@sha256:def',
          prepull_command: ['service-check', '--version'],
          ports: [{ name: 'http', container_port: 8443, service_port: 80 }],
        },
      ],
      services: [
        {
          name: 'editor-svc',
          component: 'editor',
          ports: [{ name: 'http', port: 80, target_port: 'http' }],
        },
      ],
      routes: [{ path_prefix: '/ide', service: 'editor-svc', port: 'http' }],
    },
  })
  assert.equal(draft.webComponentName, 'editor')
  assert.equal(draft.webComponentImage, 'reg/cs@sha256:def')
  assert.deepEqual(draft.prepullCommand, ['service-check', '--version'])
  assert.equal(draft.webPortName, 'http')
  assert.equal(draft.webContainerPort, '8443')
  assert.equal(draft.webServicePort, '80')
  assert.equal(draft.webServiceName, 'editor-svc')
  assert.equal(draft.webRoutePrefix, '/ide')
})

test('新登记时给出各类型的默认值', () => {
  const draft = toolDraft(undefined, '/panel-prefix/')
  assert.equal(draft.code, '')
  assert.equal(draft.kind, '4')
  assert.equal(draft.status, '1')
  assert.equal(draft.builtinEndpoint, '/panel-prefix/')
  assert.deepEqual(draft.allowedArgv, [['']])
  assert.equal(draft.defaultTimeout, '30')
  assert.equal(draft.maxTimeout, '120')
  assert.equal(draft.webRoutePrefix, '/')
})

const commandDraft = {
  ...toolDraft(undefined),
  kind: '4',
  allowedArgv: [[' solc ', '--version'], ['solc']],
  defaultTimeout: '20',
  maxTimeout: '90',
  componentName: ' compiler ',
  componentImage: ' reg/solc@sha256:abc ',
  componentCommand: ['sleep', ' infinity '],
  prepullCommand: [' solc ', '--version'],
}

test('命令声明保留空参数的位置，不在提交时悄悄改写命令', () => {
  const spec = toolResourceSpec({
    ...commandDraft,
    componentCommand: ['tool', '', '--check'],
    prepullCommand: ['tool', '', '--selftest'],
    allowedArgv: [['tool', '', '--check']],
  })
  const component = (spec.components as Record<string, unknown>[])[0]
  assert.deepEqual(component.command, ['tool', '', '--check'])
  assert.deepEqual(component.prepull_command, ['tool', '', '--selftest'])
  assert.deepEqual((spec.command_policy as Record<string, unknown>).allowed_argv, [
    ['tool', '', '--check'],
  ])
})

test('命令完整性逐项校验，不用一个有效参数掩盖空项或控制字符', () => {
  assert.equal(validCommand(['tool', '--check']), true)
  assert.equal(validCommand(['tool', 'two words']), true)
  for (const command of [
    [],
    ['tool', ''],
    ['tool', ' '],
    ['tool', 'a\nb'],
    ['tool', 'a\rb'],
    ['tool', 'a\0b'],
  ]) {
    assert.equal(validCommand(command), false, JSON.stringify(command))
  }
})

test('新登记的命令工具:一个挂载工作区的执行组件 + 命令白名单', () => {
  assert.deepEqual(toolResourceSpec(commandDraft), {
    components: [
      {
        mount_workspace: true,
        name: 'compiler',
        image_url: 'reg/solc@sha256:abc',
        command: ['sleep', 'infinity'],
        prepull_command: ['solc', '--version'],
      },
    ],
    command_policy: {
      allowed_argv: [['solc', '--version'], ['solc']],
      default_timeout_seconds: 20,
      max_timeout_seconds: 90,
    },
  })
})

test('修改命令工具时更新自检命令并保留资源和能力', () => {
  const base = {
    components: [
      {
        name: 'old',
        image_url: 'reg/old',
        command: ['old'],
        prepull_command: ['old', 'selftest'],
        mount_workspace: true,
        resources: { cpu: '500m' },
      },
    ],
    command_policy: { allowed_argv: [['old']], default_timeout_seconds: 5, max_timeout_seconds: 6 },
    capabilities: { provides: ['compile'] },
  }
  assert.deepEqual(toolResourceSpec(commandDraft, base), {
    components: [
      {
        name: 'compiler',
        image_url: 'reg/solc@sha256:abc',
        command: ['sleep', 'infinity'],
        prepull_command: ['solc', '--version'],
        mount_workspace: true,
        resources: { cpu: '500m' },
      },
    ],
    command_policy: {
      allowed_argv: [['solc', '--version'], ['solc']],
      default_timeout_seconds: 20,
      max_timeout_seconds: 90,
    },
    capabilities: { provides: ['compile'] },
  })
})

const webDraft = {
  ...toolDraft(undefined),
  kind: '3',
  webComponentName: 'editor',
  webComponentImage: 'reg/cs@sha256:def',
  prepullCommand: ['service-check', '--version'],
  webPortName: 'http',
  webContainerPort: '8443',
  webServicePort: '80',
  webServiceName: 'editor-svc',
  webRoutePrefix: '/ide',
}

test('新登记的网页工具:组件、服务、代理路由各一个', () => {
  assert.deepEqual(toolResourceSpec(webDraft), {
    components: [
      {
        mount_workspace: true,
        name: 'editor',
        image_url: 'reg/cs@sha256:def',
        prepull_command: ['service-check', '--version'],
        ports: [{ protocol: 'TCP', name: 'http', container_port: 8443, service_port: 80 }],
      },
    ],
    services: [
      {
        name: 'editor-svc',
        component: 'editor',
        ports: [{ protocol: 'TCP', name: 'http', port: 80, target_port: 'http' }],
      },
    ],
    routes: [{ path_prefix: '/ide', service: 'editor-svc', port: 'http' }],
  })
})

test('修改网页工具时保留组件、端口、服务与路由上的其他声明', () => {
  const base = {
    components: [
      {
        name: 'old',
        image_url: 'reg/old',
        mount_workspace: true,
        readiness_probe: { path: '/healthz' },
        ports: [
          {
            name: 'old',
            container_port: 1,
            service_port: 1,
            protocol: 'TCP',
            app_protocol: 'http',
          },
        ],
      },
    ],
    services: [
      {
        name: 'old-svc',
        component: 'old',
        ports: [{ name: 'old', port: 1, target_port: 'old', protocol: 'TCP' }],
      },
    ],
    routes: [{ path_prefix: '/', service: 'old-svc', port: 'old', strip_prefix: true }],
    bindings: [{ name: 'rpc' }],
  }
  assert.deepEqual(toolResourceSpec(webDraft, base), {
    components: [
      {
        name: 'editor',
        image_url: 'reg/cs@sha256:def',
        prepull_command: ['service-check', '--version'],
        mount_workspace: true,
        readiness_probe: { path: '/healthz' },
        ports: [
          {
            name: 'http',
            container_port: 8443,
            service_port: 1,
            protocol: 'TCP',
            app_protocol: 'http',
          },
        ],
      },
    ],
    services: [
      {
        name: 'editor-svc',
        component: 'editor',
        ports: [{ name: 'old', port: 80, target_port: 'http', protocol: 'TCP' }],
      },
    ],
    routes: [{ path_prefix: '/ide', service: 'editor-svc', port: 'old', strip_prefix: true }],
    bindings: [{ name: 'rpc' }],
  })
})

test('内置工具只改入口模板,终端工具原样保留', () => {
  const builtin = {
    ...toolDraft(undefined),
    kind: '1',
    builtinEndpoint: ' /panel-prefix/{id}/panel ',
  }
  assert.deepEqual(toolResourceSpec(builtin), { builtin_endpoint: '/panel-prefix/{id}/panel' })
  assert.deepEqual(
    toolResourceSpec(builtin, { builtin_endpoint: '/old', capabilities: { provides: ['panel'] } }),
    {
      builtin_endpoint: '/panel-prefix/{id}/panel',
      capabilities: { provides: ['panel'] },
    }
  )
  const terminal = { ...toolDraft(undefined), kind: '2' }
  assert.deepEqual(toolResourceSpec(terminal), {})
  assert.deepEqual(toolResourceSpec(terminal, { capabilities: { provides: ['shell'] } }), {
    capabilities: { provides: ['shell'] },
  })
})

const aliasedWeb = {
  ...base,
  kind: WEB,
  resource_spec: {
    components: [
      {
        name: 'editor',
        image_url: 'registry/tool@sha256:' + 'a'.repeat(64),
        command: ['editor'],
        prepull_command: ['editor', '--version'],
        mount_workspace: true,
        ports: [{ name: 'http', container_port: 8443, service_port: 0, protocol: 'TCP' }],
      },
    ],
    services: [
      {
        name: 'editor-svc',
        component: 'editor',
        ports: [{ name: 'web', port: 80, target_port: 'http', protocol: 'TCP' }],
      },
    ],
    routes: [{ path_prefix: '/ide', service: 'editor-svc', port: 'web' }],
  },
}

test('网页表单读取实际服务端口，未改执行配置时完整保留端口映射', () => {
  const draft = toolDraft(aliasedWeb)
  assert.equal(toolEditBlocker(aliasedWeb), undefined)
  assert.equal(draft.webServicePort, '80')
  assert.deepEqual(
    toolResourceSpec({ ...draft, name: '只改名称' }, aliasedWeb.resource_spec),
    aliasedWeb.resource_spec
  )
})

test('网页工具修改路径和容器端口名仍保留服务别名', () => {
  const draft = {
    ...toolDraft(aliasedWeb),
    webRoutePrefix: '/new',
    webPortName: 'browser',
    webServicePort: '8080',
  }
  const spec = toolResourceSpec(draft, aliasedWeb.resource_spec)
  assert.deepEqual(spec.services, [
    {
      name: 'editor-svc',
      component: 'editor',
      ports: [{ name: 'web', port: 8080, target_port: 'browser', protocol: 'TCP' }],
    },
  ])
  assert.deepEqual(spec.routes, [{ path_prefix: '/new', service: 'editor-svc', port: 'web' }])
  assert.equal(
    (spec.components as typeof aliasedWeb.resource_spec.components)[0].ports[0].service_port,
    0
  )
})

test('一个服务的多个端口超出编辑范围，不能静默删除第二个端口', () => {
  const tool = structuredClone(aliasedWeb)
  tool.resource_spec.services[0].ports.push({
    name: 'internal',
    port: 8080,
    target_port: 'http',
    protocol: 'TCP',
  })
  assert.ok(toolEditBlocker(tool))
})
