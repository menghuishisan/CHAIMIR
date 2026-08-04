// 监控面板页(平台侧栏,/platform-admin/monitoring)。
//
// 后端只给出「面板名 + 地址」两项(GET /admin/platform/monitoring/panels),
// 地址指向外部监控系统。这里做成受控跳转入口而不是内嵌画面:
//   1. 外部监控系统普遍设了禁止被嵌入的响应头,内嵌只会得到一块空白;
//   2. 把第三方页面嵌进后台会把它的脚本放进管理端同一个浏览上下文,
//      为了一块图放弃这层隔离不划算。
// 故本页只呈现入口并在新标签打开(不带来源信息),真正的画面在监控系统里看。
//
// 平台自己的运营数字在平台看板;这里是基础设施层的指标(节点、队列、数据库)。

import { useMemo } from 'react'
import { ExternalLink, LayoutDashboard, Monitor } from 'lucide-react'
import type { MonitoringPanel } from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  Stat,
} from '@chaimir/ui'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'

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

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }, { label: '监控面板' }]} />}
        title="监控面板"
        description="基础设施层的实时监控入口。面板由运维在部署时配置,这里只做受控跳转。"
        icon={Monitor}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2">
          <Stat label="可用面板" value={list.length} icon={Monitor} />
          <Stat
            label="业务运营数据"
            value="在平台看板"
            icon={LayoutDashboard}
            hint="学校、账号、课程与竞赛规模"
          />
        </div>
      </PageSection>

      <PageSection
        title="监控入口"
        description="点开会在新标签页打开监控系统。需要单独登录时按运维给的账号进入。"
      >
        <div className="flex flex-col gap-4">
          <ResourceState
            resource={panels}
            emptyIcon={Monitor}
            emptyTitle="还没有配置监控面板"
            emptyDescription="监控地址由运维在部署配置里指定,配置后会出现在这里。"
            skeleton={<Skeleton variant="line" lines={3} />}
          >
            {(items) => (
              <div className="grid gap-4 lg:grid-cols-2">
                {items.map((panel) => (
                  <PanelCard key={`${panel.name}-${panel.url}`} panel={panel} />
                ))}
              </div>
            )}
          </ResourceState>

          <Callout tone="info">
            监控画面不内嵌在后台里:外部系统通常禁止被嵌入,而且把第三方页面放进管理端同一个浏览环境会削弱隔离。
          </Callout>

          <div className="flex flex-wrap items-center gap-2 rounded-md border border-line bg-surface-sunken p-3">
            <span className="text-sm text-ink-sub">资源告警在告警中心处理,备份结果在备份记录里看。</span>
            <Button variant="ghost" size="sm" onClick={() => navigate('/platform-admin/alerts')}>
              去告警中心
            </Button>
            <Button variant="ghost" size="sm" onClick={() => navigate('/platform-admin/backups')}>
              去备份记录
            </Button>
          </div>
        </div>
      </PageSection>
    </PageScaffold>
  )
}

/**
 * PanelCard 渲染一个监控入口。
 * 新标签打开并断开来源与窗口引用(noopener,noreferrer):
 * 不把管理端地址带给外部系统,也不让外部页面拿到本页的窗口句柄。
 */
function PanelCard({ panel }: { panel: MonitoringPanel }) {
  return (
    <Card>
      <CardHeader title={panel.name} description="外部监控系统" />
      <CardBody className="flex flex-col gap-3">
        <p className="truncate font-mono text-xs text-ink-sub">{panel.url}</p>
        <div>
          <Button
            variant="outline"
            size="sm"
            leftIcon={ExternalLink}
            onClick={() => window.open(panel.url, '_blank', 'noopener,noreferrer')}
          >
            在新标签打开
          </Button>
        </div>
      </CardBody>
    </Card>
  )
}
