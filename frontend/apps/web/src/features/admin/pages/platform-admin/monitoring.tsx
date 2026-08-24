// 监控面板页(平台侧栏,/platform-admin/monitoring)。
//
// 后端只给出「面板名 + 地址」两项(GET /admin/platform/monitoring/panels),
// 地址指向外部监控系统。这里做成受控跳转入口而不是内嵌画面:
//   1. 外部监控系统普遍设了禁止被嵌入的响应头,内嵌只会得到一块空白;
//   2. 把第三方页面嵌进后台会把它的脚本放进管理端同一个浏览上下文,
//      为了一块图放弃这层隔离不划算。
// 故本页只呈现入口并在新标签打开(不带来源信息),真正的画面在监控系统里看。
//
// 归族:引导族(规范 §6.5.3 第 ⑧ 族)。本页自身没有主体数据 —— 面板名与地址不是
// 「要在这里读的记录」,而是「要去别处办事的门」。故不排资源列表骨架、不做指标带,
// 只用一块 GuidePanel 说清「这里能做什么 / 为什么不在这里做 / 从哪儿出去」。
// 平台自己的运营数字在平台看板;这里是基础设施层的指标(节点、队列、数据库)。

import { useMemo } from 'react'
import { ExternalLink, Monitor } from 'lucide-react'
import type { MonitoringPanel } from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  GuidePanel,
  PageHeader,
  PageScaffold,
  Skeleton,
} from '@chaimir/ui'
import { useNavigate } from 'react-router'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { safeMonitoringUrl } from '../../../../utils/safeNavigation'

/** 相邻能力的去处:引导族的出口必须是按钮,不能是正文里的链接(§6.5.3 第 ⑧ 族)。 */
const NEIGHBOUR_EXITS = [
  { label: '去告警中心', path: '/platform-admin/alerts' },
  { label: '去备份记录', path: '/platform-admin/backups' },
  { label: '去平台看板', path: '/platform-admin/dashboard' },
] as const

/**
 * PlatformMonitoringPage 列出受控的外部监控入口。
 */
export default function PlatformMonitoringPage() {
  const navigate = useNavigate()

  const panels = useAsyncResource(
    () => api.admin.monitoringPanels(),
    [],
    (value) => value.length === 0,
  )

  const list = useMemo(() => panels.data ?? [], [panels.data])

  /** 出口按钮组:外部面板用描边按钮(离开本系统),相邻页面用幽灵按钮(仍在本系统内)。 */
  const exits = (
    <>
      {list.map((panel) => (
        <PanelExit key={`${panel.name}-${panel.url}`} panel={panel} />
      ))}
      {NEIGHBOUR_EXITS.map((exit) => (
        <Button key={exit.path} variant="ghost" size="sm" onClick={() => navigate(exit.path)}>
          {exit.label}
        </Button>
      ))}
    </>
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }]} />}
        title="监控面板"
        description="基础设施层的实时监控入口。面板由运维在部署时配置,这里只做受控跳转。"
        icon={Monitor}
      />

      <ResourceState
        resource={panels}
        emptyIcon={Monitor}
        emptyTitle="还没有配置监控面板"
        emptyDescription="监控地址由运维在部署配置里指定,配置后会作为入口出现在这里。"
        skeleton={<Skeleton variant="block" />}
      >
        {() => (
          <GuidePanel
            icon={Monitor}
            title="监控画面在外部监控系统里看"
            reason={
              <>
                外部监控系统通常禁止被嵌入,而且把第三方页面放进管理端同一个浏览环境会削弱隔离,
                所以这里不内嵌画面,只保留受控入口。资源告警在告警中心处理,备份结果在备份记录里看,
                学校与账号规模在平台看板。
              </>
            }
            actions={exits}
            hint={`共 ${list.length} 个面板。点开会在新标签页打开监控系统,需要单独登录时按运维给的账号进入。`}
          />
        )}
      </ResourceState>
    </PageScaffold>
  )
}

/**
 * PanelExit 渲染一个外部监控入口按钮。
 * 新标签打开并断开来源与窗口引用(noopener,noreferrer):
 * 不把管理端地址带给外部系统,也不让外部页面拿到本页的窗口句柄。
 * 地址不合规(非 http/https 或格式非法)时按钮禁用 —— 不让用户点进一个打不开的地方。
 */
function PanelExit({ panel }: { panel: MonitoringPanel }) {
  const safeUrl = safeMonitoringUrl(panel.url)

  return (
    <Button
      variant="outline"
      leftIcon={ExternalLink}
      disabled={!safeUrl}
      onClick={() => {
        if (safeUrl) window.open(safeUrl, '_blank', 'noopener,noreferrer')
      }}
    >
      {panel.name}
    </Button>
  )
}
