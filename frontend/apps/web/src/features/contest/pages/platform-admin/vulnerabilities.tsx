// 漏洞题源页(平台侧栏,/platform-admin/vulnerabilities)。
//
// 这里只维护「全平台共享的漏洞来源」:tenant_id=0 的全局源,所有学校都能从它同步案例。
// 边界在后端就是分开的两组路由 —— 平台组只有 GET/POST /contest/platform/vuln-sources,
// 同步案例(POST /vuln-sources/{id}/sync)、漏洞题草稿、预验证与固化都在 teacher 组,
// 是租户内的出题工作流。故本页不出现这些动作(对齐清单 §3.4)。
//
// 表单与教师侧共用同一实现,只是落到不同归属,由 global 显式声明。

import { useMemo, useState } from 'react'
import { Bug, Database, Globe, Plus, Settings2 } from 'lucide-react'
import { type VulnSource } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  Stat,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  VULN_SOURCE_CONFIG_FIELDS,
  vulnLevelLabel,
  vulnSourceTypeLabel,
} from '../../../../utils/labels/contest'
import { VulnSourceFormModal } from '../vuln-source-form'

/**
 * PlatformVulnerabilitiesPage 维护全平台共享的漏洞来源。
 */
export default function PlatformVulnerabilitiesPage() {
  const [formTarget, setFormTarget] = useState<{ source?: VulnSource } | undefined>()

  const sources = useAsyncResource(
    () => api.contest.listPlatformVulnSources(),
    [],
    (value) => value.length === 0,
  )

  const list = useMemo(() => sources.data ?? [], [sources.data])

  const stats = useMemo(
    () => ({
      enabled: list.filter((item) => item.enabled).length,
      synced: list.filter((item) => item.last_sync_at !== undefined).length,
    }),
    [list],
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }, { label: '漏洞题源' }]} />}
        title="漏洞题源"
        description="全平台共享的漏洞案例来源。学校的教师从这些来源同步案例,再转化成本校的赛题。"
        icon={Bug}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
            添加全局来源
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="全局来源" value={list.length} icon={Globe} />
          <Stat label="已启用" value={stats.enabled} icon={Database} hint="学校可以从它同步" />
          <Stat label="曾同步过" value={stats.synced} icon={Bug} />
        </div>
      </PageSection>

      <PageSection
        title="全局来源"
        description="来源是一个返回漏洞案例列表的公开接口。密钥类配置由后端加密保存,不回显。"
      >
        <div className="flex flex-col gap-4">
          <ResourceState
            resource={sources}
            emptyIcon={Database}
            emptyTitle="还没有配置全局漏洞来源"
            emptyDescription="配置后所有学校的教师都能从这里同步漏洞案例做成赛题。"
            emptyAction={
              <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
                添加全局来源
              </Button>
            }
            skeleton={<Skeleton variant="line" lines={4} />}
          >
            {(items) => (
              <div className="grid gap-4 lg:grid-cols-2">
                {items.map((source) => (
                  <VulnSourceCard
                    key={source.id}
                    source={source}
                    onEdit={() => setFormTarget({ source })}
                  />
                ))}
              </div>
            )}
          </ResourceState>

          <Callout tone="info">
            同步案例、预验证与固化进题库都是学校内的出题工作,由教师在漏洞题工坊完成 ——
            平台这里只负责来源本身可用。
          </Callout>
        </div>
      </PageSection>

      {formTarget ? (
        <VulnSourceFormModal
          source={formTarget.source}
          global
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            sources.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface VulnSourceCardProps {
  source: VulnSource
  onEdit: () => void
}

/**
 * VulnSourceCard 展示单个全局来源的配置概要。
 * 同步按钮不在这里:同步是租户动作,落到本校的漏洞题草稿上,平台没有归属可落。
 */
function VulnSourceCard({ source, onEdit }: VulnSourceCardProps) {
  const endpoint = readString(source.config, VULN_SOURCE_CONFIG_FIELDS.endpoint)
  const method = readString(source.config, VULN_SOURCE_CONFIG_FIELDS.method)

  return (
    <Card>
      <CardHeader
        title={source.name}
        description={vulnSourceTypeLabel(source.type)}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone="jade">全平台共享</Badge>
            {source.enabled ? <Badge tone="success">已启用</Badge> : <Badge tone="neutral">已停用</Badge>}
          </div>
        }
      />
      <CardBody className="flex flex-col gap-3">
        <DescriptionList
          dense
          items={[
            { term: '默认分级', description: vulnLevelLabel(source.default_level) },
            { term: '接口地址', description: endpoint || '未配置', mono: true },
            { term: '请求方式', description: method || 'GET' },
            {
              term: '上次同步',
              description: source.last_sync_at ? formatDateTime(source.last_sync_at) : '尚未同步',
              mono: true,
            },
          ]}
        />
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="ghost" size="sm" leftIcon={Settings2} onClick={onEdit}>
            修改配置
          </Button>
          {source.enabled ? null : (
            <span className="text-sm text-ink-sub">停用后学校不能再从这个来源同步。</span>
          )}
        </div>
      </CardBody>
    </Card>
  )
}

/** readString 从来源配置里读字符串字段;被后端脱敏的字段回空串,不把掩码对象抛到界面上。 */
function readString(source: Record<string, unknown>, key: string): string {
  const value = source[key]
  return typeof value === 'string' ? value : ''
}
