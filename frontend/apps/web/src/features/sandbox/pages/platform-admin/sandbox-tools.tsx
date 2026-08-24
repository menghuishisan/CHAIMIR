// 沙箱工具页(平台侧栏,/platform-admin/sandbox-tools)。
//
// 工具是学生在实验环境里能用到的东西:平台内置面板、终端、网页工具(如在线编辑器)、
// 受控命令工具(如编译器)。四类的声明形态完全不同,后端 validateToolResourceSpecShape
// 对每类的必填与禁填都有硬要求,故本页按类型分别呈现,登记表单也按类型分叉。
//
// 只有登记与查看:后端平台组只给了 GET/POST /sandbox/tools,没有删除接口 ——
// 不再使用的工具改成「已停用」,历史环境的引用不会因此断链。

import { useMemo, useState } from 'react'
import { Package, Plus } from 'lucide-react'
import {
  SANDBOX_COMPONENT_CATEGORY,
  SandboxToolKind,
  ToolStatus,
  type SandboxToolDefinition,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  FilterBar,
  FilterField,
  PageHeader,
  MetricStrip,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import {
  ecoTagsLabel,
  sandboxToolKindLabel,
  toolStatusLabel,
} from '../../../../utils/labels/sandbox'
import { toolStatusTone } from '../../statusPresentation'
import { ToolFormModal } from './tool-form'
import { asRecord, readString } from '../../runtimeSpec'

/** 类型筛选项:值为空串表示不过滤。 */
const KIND_FILTERS = [
  { value: '', label: '全部' },
  { value: String(SandboxToolKind.BUILTIN), label: sandboxToolKindLabel(SandboxToolKind.BUILTIN) },
  {
    value: String(SandboxToolKind.TERMINAL),
    label: sandboxToolKindLabel(SandboxToolKind.TERMINAL),
  },
  {
    value: String(SandboxToolKind.WEB_EMBED),
    label: sandboxToolKindLabel(SandboxToolKind.WEB_EMBED),
  },
  { value: String(SandboxToolKind.COMMAND), label: sandboxToolKindLabel(SandboxToolKind.COMMAND) },
] as const

/** 各类工具的一句话说明:登记表单里给更细的要求。 */
const KIND_HINTS: Record<SandboxToolKind, string> = {
  [SandboxToolKind.BUILTIN]: '平台自带的面板,按实验环境编号生成入口,无需额外启动',
  [SandboxToolKind.TERMINAL]: '直接连接实验环境的命令行,不需要额外声明',
  [SandboxToolKind.WEB_EMBED]: '独立环境提供的网页工具,经平台代理嵌入工作台',
  [SandboxToolKind.COMMAND]: '在受控白名单内执行命令,独立运行、不开放网络端口',
  [SandboxToolKind.INFRA]: '只参与组合编排的基础设施,不作为学生工具入口',
}

/**
 * PlatformSandboxToolsPage 列出沙箱工具定义并承载登记。
 */
export default function PlatformSandboxToolsPage() {
  const [kindFilter, setKindFilter] = useState<string>('')
  const [createOpen, setCreateOpen] = useState(false)

  const tools = useAsyncResource(
    () => api.sandbox.listTools(),
    [],
    (value) => value.length === 0
  )

  const list = useMemo(() => tools.data ?? [], [tools.data])

  const visible = useMemo(
    () => (kindFilter ? list.filter((item) => item.kind === Number(kindFilter)) : list),
    [kindFilter, list]
  )

  const stats = useMemo(
    () => ({
      available: list.filter((item) => item.status === ToolStatus.AVAILABLE).length,
      studentTools: list.filter((item) => item.category === SANDBOX_COMPONENT_CATEGORY.TOOL).length,
      infra: list.filter((item) => item.category === SANDBOX_COMPONENT_CATEGORY.INFRA).length,
    }),
    [list]
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }]} />}
        title="实验工具"
        description="学生在实验环境里能用到的工具。教师在编排环境时显式选择,平台按能力依赖补齐必要组件。"
        icon={Package}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
            登记工具
          </Button>
        }
      />

      {/*
        归族:资源列表族的卡片网格形态(§6.5.3 第 ① 族)。指标降为内联摘要 ——
        Stat 大卡属看板族,`<md` 竖排四张大卡违反 §6.4.1 规则 2。
        四项由一次取齐的全量工具定义算出(接口不分页,故是全量口径,§6.5.4)。
      */}
      <MetricStrip
        label="工具总量摘要"
        className="mb-5"
        items={[
          { label: '组件总数', value: list.length, hint: '含已停用' },
          { label: '可用', value: stats.available, hint: '可被实验引用' },
          { label: '学生工具', value: stats.studentTools, hint: '学生能打开' },
          { label: '基础设施', value: stats.infra, hint: '只参与编排' },
        ]}
      />

      <PageSection
        title="工具定义"
        description="按类型筛选。不再使用的工具改成停用,不做删除 —— 历史环境仍引用着它。"
      >
        <div className="flex flex-col gap-4">
          {/* 数据区是一排工具卡(已是抬起片),筛选走 bare 无底形态,避免片里套片(§6.5.2) */}
          <FilterBar label="工具筛选" bare>
            <FilterField label="工具类型" group>
              <SegmentedControl
                aria-label="按工具类型筛选"
                size="sm"
                options={KIND_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={kindFilter}
                onValueChange={setKindFilter}
              />
            </FilterField>
          </FilterBar>

          <ResourceState
            resource={tools}
            emptyIcon={Package}
            emptyTitle="还没有登记沙箱工具"
            emptyDescription="登记工具后,运行时与实验才能把它放进学生的工作台。"
            emptyAction={
              <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
                登记工具
              </Button>
            }
            skeleton={<Skeleton variant="line" lines={4} />}
          >
            {() =>
              visible.length === 0 ? (
                <Callout tone="info">这个类型下没有工具,换个类型看看。</Callout>
              ) : (
                <div className="grid gap-4 lg:grid-cols-2">
                  {visible.map((tool) => (
                    <ToolCard key={tool.id} tool={tool} />
                  ))}
                </div>
              )
            }
          </ResourceState>

          <Callout tone="info">
            登记同一个编码会覆盖原有定义。改动对之后创建的环境生效,已在运行的环境保持原样。
          </Callout>
        </div>
      </PageSection>

      {createOpen ? (
        <ToolFormModal
          onClose={() => setCreateOpen(false)}
          onSaved={() => {
            setCreateOpen(false)
            tools.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

/**
 * ToolCard 展示单个工具定义。
 * 按类型只呈现该类型真正用到的字段:命令工具给白名单与超时,网页工具给容器与路由,
 * 内置工具给入口模板 —— 不把其他类型的空字段摊在界面上。
 */
function ToolCard({ tool }: { tool: SandboxToolDefinition }) {
  const items = useMemo(() => {
    const base = [
      { term: '工具编码', description: tool.code, mono: true },
      { term: '适用生态', description: ecoTagsLabel(tool.eco_tags) },
    ]
    switch (tool.kind) {
      case SandboxToolKind.BUILTIN:
        return [
          ...base,
          {
            term: '入口地址模板',
            description: tool.resource_spec.builtin_endpoint ?? '未配置',
            mono: true,
          },
        ]
      case SandboxToolKind.COMMAND: {
        const policy = tool.resource_spec.command_policy
        return [
          ...base,
          {
            term: '允许的 argv',
            description:
              (policy?.allowed_argv ?? []).map((argv) => JSON.stringify(argv)).join('、') ||
              '未配置',
          },
          {
            term: '超时设置',
            description: policy
              ? `默认 ${policy.default_timeout_seconds} 秒,最长 ${policy.max_timeout_seconds} 秒`
              : '未配置',
          },
          {
            term: '执行环境',
            description:
              readString(asRecord(tool.resource_spec.components?.[0]), 'name') || '未配置',
            mono: true,
          },
        ]
      }
      case SandboxToolKind.WEB_EMBED:
        return [
          ...base,
          { term: '组件数量', description: `${tool.resource_spec.components?.length ?? 0} 个` },
          { term: '代理路由数', description: `${tool.resource_spec.routes?.length ?? 0} 条` },
          {
            term: '网络放行规则',
            description: `${tool.resource_spec.network_rules?.length ?? 0} 条`,
          },
        ]
      default:
        return base
    }
  }, [tool])

  return (
    <Card>
      <CardHeader
        title={tool.name}
        description={sandboxToolKindLabel(tool.kind)}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            {/* 类别与类型是两件事:类别决定它是不是学生工具入口(§7.2),类型决定怎么用 */}
            {tool.category === SANDBOX_COMPONENT_CATEGORY.INFRA ? (
              <Badge tone="info">基础设施</Badge>
            ) : (
              <Badge tone="jade">学生工具</Badge>
            )}
            <Badge tone="neutral">{sandboxToolKindLabel(tool.kind)}</Badge>
            <StatusIndicator
              tone={toolStatusTone(tool.status)}
              label={toolStatusLabel(tool.status)}
            />
          </div>
        }
      />
      <CardBody className="flex flex-col gap-3">
        <p className="text-sm text-ink-sub">{KIND_HINTS[tool.kind]}</p>
        {/*
          停用原因由服务端目录同步写入(镜像缺证明、工作负载声明不合当前契约等),
          原样展示 —— 前端不推断也不隐藏(对齐清单 §6.3 / §8.3)。
        */}
        {tool.resource_spec.disabled_reason ? (
          <Callout tone="warning" title="这个组件当前不可部署">
            {tool.resource_spec.disabled_reason}
          </Callout>
        ) : null}
        <DescriptionList dense items={items} />
      </CardBody>
    </Card>
  )
}
