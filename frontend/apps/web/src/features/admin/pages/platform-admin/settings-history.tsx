// 配置变更历史页(平台深页,/platform-admin/settings/:configKey/history)。
//
// 为什么是整页而不是弹窗:变更记录是分页数据,而浮层里不出现分页 ——
// 需要分页说明它其实是一页(规范 §6.5.5 A)。回滚也不是「顺手一点」的动作:
// 它把内容写回某次改动之前,值得占一整幅版面把「回滚到哪一刻、影响哪些字段」说清。
//
// 单读走 getConfig(key):返回脱敏值与当前 version。
// 回滚要带当前 version(乐观锁):别人在这期间改过就会冲突并要求重新读取。

import { useCallback, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { History, RotateCcw, TriangleAlert } from 'lucide-react'
import { AdminScope, type ConfigChangeLog, type SystemConfig } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  ObjectIdentity,
  PageHeader,
  PageScaffold,
  Pagination,
  Skeleton,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import { configKeyDescription, configKeyLabel } from '../../../../utils/labels/admin'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { changedKeys } from '../../configValue'

/**
 * PlatformConfigHistoryPage 列出一项平台配置的变更记录并承载回滚。
 */
export default function PlatformConfigHistoryPage() {
  const { configKey = '' } = useParams<{ configKey: string }>()

  // 单读走 getConfig:深链首屏不再拉全量配置列表在浏览器里筛这一项。
  // 返回的是脱敏值与当前 version,回滚就带这个 version(对齐清单 §6.1)
  const config = useAsyncResource(
    () => api.admin.getConfig(configKey, { scope: AdminScope.GLOBAL }),
    [configKey],
    () => false,
  )

  return (
    <PageScaffold>
      {/*
        归族:详情族(§6.5.3 第 ④)。h1 由 ObjectIdentity 的配置项名承担,
        故页面头只出面包屑,末节到「系统配置」为止(§6.5.0 通则 1)。
      */}
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '底层资源' },
              { label: '系统配置', href: '/platform-admin/settings' },
            ]}
          />
        }
      />

      <ResourceState
        resource={config}
        emptyIcon={History}
        emptyTitle="配置项不存在"
        emptyDescription="这一项配置可能已被移除,回系统配置页重新选择一项。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="block" />
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {(data) => <ConfigHistoryContent config={data} onRolledBack={config.reload} />}
      </ResourceState>
    </PageScaffold>
  )
}

interface ConfigHistoryContentProps {
  config: SystemConfig
  onRolledBack: () => void
}

/**
 * ConfigHistoryContent 渲染配置身份区与变更记录表。
 */
function ConfigHistoryContent({ config, onRolledBack }: ConfigHistoryContentProps) {
  const navigate = useNavigate()
  const [target, setTarget] = useState<ConfigChangeLog>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const history = usePagedResource<ConfigChangeLog>(
    (params) => api.admin.listConfigHistory(config.key, { scope: AdminScope.GLOBAL, ...params }),
    [config.key],
  )

  const rollback = useCallback(async () => {
    if (!target) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.admin.rollbackConfig(config.key, {
        scope: AdminScope.GLOBAL,
        version: config.version,
        change_log_id: target.id,
      })
      toast.success('已回滚到这次改动前的内容')
      setTarget(undefined)
      onRolledBack()
      history.reload()
    } catch (error) {
      setActionError(
        userFacingErrorMessage(
          error,
          '回滚没有成功。如果这一项刚被别人改过,请回配置页重新进入再试。',
        ),
      )
    } finally {
      setWorking(false)
    }
  }, [config.key, config.version, history, onRolledBack, target])

  const columns: TableColumn<ConfigChangeLog>[] = [
    {
      key: 'created_at',
      header: '改动时间',
      render: (log) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(log.created_at)}
        </span>
      ),
    },
    {
      key: 'changed_keys',
      header: '改了哪些字段',
      render: (log) => {
        const keys = changedKeys(log.old_value, log.new_value)
        return keys.length > 0 ? (
          <span className="flex flex-wrap gap-1">
            {keys.map((key) => (
              <Badge key={key} tone="neutral">
                {key}
              </Badge>
            ))}
          </span>
        ) : (
          <span className="text-sm text-ink-sub">内容未变</span>
        )
      },
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (log) => (
        <Button variant="ghost" size="sm" leftIcon={RotateCcw} onClick={() => setTarget(log)}>
          回滚到改动前
        </Button>
      ),
    },
  ]

  return (
    <>
      <ObjectIdentity
        name={`${configKeyLabel(config.key)}的变更历史`}
        subtitle={configKeyDescription(config.key) ?? '使用方自定义的配置键,平台不额外登记说明。'}
        actions={
          <Button variant="outline" onClick={() => navigate('/platform-admin/settings')}>
            返回系统配置
          </Button>
        }
        properties={[
          { label: '配置键', value: <span className="font-mono">{config.key}</span> },
          { label: '当前版本', value: config.version },
          { label: '变更次数', value: history.total },
          { label: '最近更新', value: formatDateTime(config.updated_at) },
        ]}
      />

      {actionError ? (
        <Callout tone="danger" className="mt-4">
          {actionError}
        </Callout>
      ) : null}

      {/*
        就地二次确认(§7.2 B):回滚会立刻改变全平台生效的运行参数,不可轻易撤销,
        故点了「回滚到改动前」先在这里停一下,把回滚到哪一刻说清再落。
      */}
      {target ? (
        <div className="mt-4 flex flex-col gap-3 rounded-lg bg-surface p-5 shadow-xs">
          <div className="flex flex-wrap items-center gap-2">
            <TriangleAlert aria-hidden="true" className="size-4 text-warning" />
            <span className="text-base text-ink">
              确认回滚到 {formatDateTime(target.created_at)} 这次改动之前?
            </span>
          </div>
          <p className="text-sm text-ink-sub">
            回滚本身也会记一条新的变更记录,所以随时可以再回滚回来。凭据字段按当时的加密值写回。
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="danger" loading={working} onClick={() => void rollback()}>
              确认回滚
            </Button>
            <Button variant="ghost" onClick={() => setTarget(undefined)}>
              先不回滚
            </Button>
          </div>
        </div>
      ) : null}

      {/* 数据表与分页同处一块抬起片(§6.5.2)。变更记录只按时间排,没有可筛的维度 */}
      <DataPanel
        label="变更记录"
        className="mt-6"
        footer={
          <Pagination
            page={history.page}
            pageSize={history.pageSize}
            total={history.total}
            onPageChange={history.setPage}
          />
        }
      >
        <ResourceState
          resource={history}
          emptyIcon={History}
          emptyTitle="还没有变更记录"
          emptyDescription="这一项自创建以来没有改动过。"
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(page) => (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(item) => item.id}
              elevated={false}
              // <md 换行卡(§6.4.1 规则 3):改动时间一行、改了哪些字段一行,回滚按钮在右
              mobileCard={(item) => ({
                title: formatDateTime(item.created_at),
                meta:
                  changedKeys(item.old_value, item.new_value).join('、') || '内容未变',
                action: (
                  <Button
                    variant="ghost"
                    size="sm"
                    leftIcon={RotateCcw}
                    onClick={() => setTarget(item)}
                  >
                    回滚
                  </Button>
                ),
              })}
            />
          )}
        </ResourceState>
      </DataPanel>
    </>
  )
}
