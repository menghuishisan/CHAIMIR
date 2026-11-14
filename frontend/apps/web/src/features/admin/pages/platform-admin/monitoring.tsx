// 监控面板(平台侧栏,/platform-admin/monitoring)。设计 §7.6:
// 跳转到外部监控系统看基础设施。一句说明「监控画面在外部监控系统里」+ 监控面板卡片网格
// (面板名、所在地址、「在新窗口打开」);未配置面板时说明由运维在部署时配置;底部相关入口(告警中心、备份记录、平台看板)。
//
// 不内嵌画面:外部监控系统通常禁止被嵌入,而且把第三方页面放进管理端同一个浏览环境会削弱隔离。
// 打开时断开来源与窗口引用,不把管理端地址带给外部系统;地址不是安全的 https 地址时不给打开。
// 接口只返回面板名与地址,没有用途说明,所以卡片上写面板所在的站点,便于认出是哪套系统。

import { Link } from 'react-router'
import { ExternalLink, Monitor } from 'lucide-react'
import type { MonitoringPanel } from '@chaimir/api-client'
import { Button, buttonVariants, Card, PageHeader, PageScaffold, Skeleton } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { safeMonitoringUrl } from '../../../../utils/safeNavigation'

/** 相关入口:基础设施之外的事在平台里的去处 */
const RELATED_LINKS = [
  { label: '告警中心', path: '/platform-admin/alerts' },
  { label: '备份记录', path: '/platform-admin/backups' },
  { label: '平台看板', path: '/platform-admin/dashboard' },
] as const

/** PlatformMonitoringPage 外部监控入口的卡片网格 */
export default function PlatformMonitoringPage() {
  // 没有面板是正常情况(由运维配置),仍要显示说明和相关入口,不当作空态
  const panels = useAsyncResource(
    () => api.admin.monitoringPanels(),
    [],
    () => false
  )

  return (
    <PageScaffold>
      <PageHeader title="监控面板" description="监控画面在外部监控系统里,点面板会在新窗口打开。" />

      <ResourceState
        resource={panels}
        emptyIcon={Monitor}
        emptyTitle="还没有配置监控面板"
        emptyDescription="监控地址由运维在部署时配置。"
        skeleton={<Skeleton variant="block" />}
      >
        {(list) =>
          list.length === 0 ? (
            <p className="rounded-lg bg-surface px-4 py-10 text-center text-sm text-ink-sub shadow-xs">
              还没有配置监控面板。监控地址由运维在部署时配置,配置后会出现在这里。
            </p>
          ) : (
            <ul className="card-grid gap-4">
              {list.map((panel) => (
                <li key={`${panel.name}-${panel.url}`}>
                  <PanelCard panel={panel} />
                </li>
              ))}
            </ul>
          )
        }
      </ResourceState>

      <section aria-label="相关入口" className="mt-8 flex flex-wrap items-center gap-2">
        <span className="text-sm text-ink-sub">相关入口:</span>
        {RELATED_LINKS.map((link) => (
          <Link
            key={link.path}
            to={link.path}
            className={buttonVariants({ variant: 'ghost', size: 'sm' })}
          >
            {link.label}
          </Link>
        ))}
      </section>
    </PageScaffold>
  )
}

/** PanelCard 一个监控面板:名称、所在站点、「在新窗口打开」;地址不安全时说明并不给打开 */
function PanelCard({ panel }: { panel: MonitoringPanel }) {
  const safeUrl = safeMonitoringUrl(panel.url)
  const host = safeUrl ? new URL(safeUrl).host : undefined
  return (
    <Card className="flex h-full flex-col gap-3 p-5">
      <span className="flex items-start gap-3">
        <Monitor aria-hidden="true" className="mt-0.5 size-5 shrink-0 text-ink-sub" />
        <span className="min-w-0">
          <span className="block text-lg font-bold text-ink">{panel.name}</span>
          <span className="block truncate font-mono text-xs text-ink-sub">
            {host ?? '地址不是安全的 https 地址,不能打开'}
          </span>
        </span>
      </span>
      <div className="mt-auto pt-2">
        <Button
          variant="outline"
          size="sm"
          leftIcon={ExternalLink}
          disabled={!safeUrl}
          onClick={() => {
            if (safeUrl) window.open(safeUrl, '_blank', 'noopener,noreferrer')
          }}
        >
          在新窗口打开
        </Button>
      </div>
    </Card>
  )
}
