// 漏洞题源(平台侧栏,/platform-admin/vulnerabilities)。设计 §7.6:
// 页头 +「添加全局来源」;列表行显示来源名、类型、启用状态、上次同步时间;整行打开来源编辑抽屉;
// 密钥类配置不回显(由后端加密保存)。指标条、平铺属性的卡片和常驻说明卡去掉。
//
// 这里只维护全平台共享的来源(所有学校的教师都能从它同步案例)。同步案例、做成赛题、预验证与确认
// 都是学校内的出题工作,在教师端的漏洞题工坊完成,平台这里不出现这些动作。

import { useState } from 'react'
import { Database, Plus } from 'lucide-react'
import type { VulnSource } from '@chaimir/api-client'
import { Button, ListRow, PageHeader, PageScaffold, Skeleton, StatusIndicator } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { vulnSourceTypeLabel } from '../../../../utils/labels/contest'
import { VulnSourceFormModal } from '../../components/VulnSourceFormModal'

/** PlatformVulnerabilitiesPage 全局漏洞来源清单,整行打开编辑抽屉 */
export default function PlatformVulnerabilitiesPage() {
  const [target, setTarget] = useState<{ source?: VulnSource }>()
  const sources = useAsyncResource(
    () => api.contest.listPlatformVulnSources(),
    [],
    (list) => list.length === 0
  )
  const total = sources.status === 'success' ? sources.data?.length : undefined

  return (
    <PageScaffold>
      <PageHeader
        title="漏洞题源"
        meta={total !== undefined ? [{ value: `${total} 个全局来源` }] : undefined}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setTarget({})}>
            添加全局来源
          </Button>
        }
      />

      <ResourceState
        resource={sources}
        emptyIcon={Database}
        emptyTitle="还没有配置全局漏洞来源"
        emptyDescription="配置后,所有学校的教师都能从这里同步漏洞案例做成赛题。"
        emptyAction={
          <Button variant="primary" leftIcon={Plus} onClick={() => setTarget({})}>
            添加全局来源
          </Button>
        }
        skeleton={<Skeleton variant="line" lines={4} />}
      >
        {(items) => (
          <ul className="divide-y divide-line rounded-lg bg-surface px-2 py-1 shadow-xs">
            {items.map((source) => (
              <li key={source.id}>
                <ListRow
                  title={source.name}
                  meta={
                    <span>
                      {vulnSourceTypeLabel(source.type)} ·{' '}
                      {source.last_sync_at ? (
                        <span title={formatDateTime(source.last_sync_at)}>
                          {formatRelativeOccurrence(source.last_sync_at)}同步过
                        </span>
                      ) : (
                        '还没有学校同步过'
                      )}
                    </span>
                  }
                  trailing={
                    <StatusIndicator
                      tone={source.enabled ? 'success' : 'neutral'}
                      label={source.enabled ? '已启用' : '已停用'}
                    />
                  }
                  onSelect={() => setTarget({ source })}
                />
              </li>
            ))}
          </ul>
        )}
      </ResourceState>

      {target ? (
        <VulnSourceFormModal
          source={target.source}
          global
          onClose={() => setTarget(undefined)}
          onSaved={() => {
            setTarget(undefined)
            sources.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}
