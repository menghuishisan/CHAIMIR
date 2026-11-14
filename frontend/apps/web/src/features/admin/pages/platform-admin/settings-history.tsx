// 配置变更历史(平台深页,/platform-admin/settings/:configKey/history)。设计 §7.6:
// 顶栏位置「系统配置 › 配置名」;页头为配置名 + 当前版本;主体是按时间倒序的变更时间线,
// 每条显示时间(精确到秒)、改前和改后的值对照(凭据不显示明文)、「回滚到改前」;
// 回滚确认弹窗写清会变成什么值。指标条和「返回系统配置」按钮去掉。
// 每条显示操作人姓名(变更历史随记录给出,账号已移除时写「已移除的账号」)。
//
// 单读走 getConfig(key):返回脱敏值与当前版本;回滚带当前版本(乐观锁),别人在这期间改过就会被拒绝。

import { useState } from 'react'
import { useParams } from 'react-router'
import { History, RotateCcw } from 'lucide-react'
import { AdminScope, type ConfigChangeLog, type SystemConfig } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Pagination,
  Skeleton,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import {
  locationLabel,
  useAsyncResource,
  usePagedResource,
  usePageLocation,
} from '../../../../hooks'
import { formatDateTimeSeconds, formatRelativeOccurrence } from '../../../../utils/formatters'
import { configKeyDescription, configKeyLabel } from '../../../../utils/labels/admin'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { changeRows } from '../../configValue'

/** PlatformConfigHistoryPage 一项平台配置的变更时间线与回滚 */
export default function PlatformConfigHistoryPage() {
  const { configKey = '' } = useParams<{ configKey: string }>()
  const config = useAsyncResource(
    () => api.admin.getConfig(configKey, { scope: AdminScope.GLOBAL }),
    [configKey],
    () => false
  )

  usePageLocation([
    { label: '系统配置', to: '/platform-admin/settings' },
    { label: locationLabel(config, (data) => configKeyLabel(data.key), '变更历史') },
  ])

  return (
    <PageScaffold>
      <ResourceState
        resource={config}
        emptyIcon={History}
        emptyTitle="没有找到这项配置"
        emptyDescription="这一项配置可能已被移除,回系统配置重新选择。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="title" width="half" />
            <Skeleton variant="line" lines={6} />
          </div>
        }
      >
        {(data) => <ConfigHistory config={data} onRolledBack={config.reload} />}
      </ResourceState>
    </PageScaffold>
  )
}

/** ConfigHistory 页头与时间线 */
function ConfigHistory({
  config,
  onRolledBack,
}: {
  config: SystemConfig
  onRolledBack: () => void
}) {
  const [target, setTarget] = useState<ConfigChangeLog>()
  const history = usePagedResource<ConfigChangeLog>(
    (params) => api.admin.listConfigHistory(config.key, { scope: AdminScope.GLOBAL, ...params }),
    [config.key],
    { urlPageKey: 'page' }
  )

  return (
    <>
      <PageHeader
        title={configKeyLabel(config.key)}
        meta={[
          { value: <span className="font-mono">{config.key}</span> },
          { label: '当前', value: `第 ${config.version} 版` },
          {
            label: '最近更新',
            value: (
              <span title={formatDateTimeSeconds(config.updated_at)}>
                {formatRelativeOccurrence(config.updated_at)}
              </span>
            ),
          },
        ]}
        description={configKeyDescription(config.key)}
      />

      <ResourceState
        resource={history}
        emptyIcon={History}
        emptyTitle="还没有变更记录"
        emptyDescription="这一项自创建以来没有改动过。"
        skeleton={<Skeleton variant="line" lines={6} />}
      >
        {(page) => (
          <>
            <ol aria-label="变更时间线" className="flex flex-col gap-3">
              {page.list.map((log) => (
                <ChangeEntry key={log.id} log={log} onRollback={() => setTarget(log)} />
              ))}
            </ol>
            <div className="mt-4">
              <Pagination
                page={history.page}
                pageSize={history.pageSize}
                total={history.total}
                onPageChange={history.setPage}
              />
            </div>
          </>
        )}
      </ResourceState>

      {target ? (
        <RollbackModal
          config={config}
          log={target}
          onClose={() => setTarget(undefined)}
          onDone={() => {
            setTarget(undefined)
            onRolledBack()
            history.reload()
          }}
        />
      ) : null}
    </>
  )
}

/** ChangeEntry 一次改动:时间(精确到秒)、每个变化的键改前与改后、「回滚到改前」 */
function ChangeEntry({ log, onRollback }: { log: ConfigChangeLog; onRollback: () => void }) {
  const rows = changeRows(log.old_value, log.new_value)
  return (
    <li className="rounded-lg bg-surface p-4 shadow-xs">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="flex flex-wrap items-baseline gap-x-2">
          <span className="font-mono text-sm tabular-nums text-ink">{formatDateTimeSeconds(log.created_at)}</span>
          <span className="text-sm text-ink-sub">{log.operator_name ?? '已移除的账号'}</span>
        </span>
        <Button variant="ghost" size="sm" leftIcon={RotateCcw} onClick={onRollback}>
          回滚到改前
        </Button>
      </div>
      {rows.length === 0 ? (
        <p className="mt-2 text-sm text-ink-sub">这次保存没有改动内容。</p>
      ) : (
        <ChangeTable rows={rows} />
      )}
    </li>
  )
}

/** ChangeTable 改前与改后对照(窄屏可横向滚动) */
function ChangeTable({ rows }: { rows: ReturnType<typeof changeRows> }) {
  return (
    <div className="mt-3 overflow-x-auto">
      <table className="w-full min-w-96 text-left text-sm">
        <thead>
          <tr className="text-xs text-ink-sub">
            <th scope="col" className="w-1/4 py-1 pr-3 font-medium">
              字段
            </th>
            <th scope="col" className="py-1 pr-3 font-medium">
              改前
            </th>
            <th scope="col" className="py-1 font-medium">
              改后
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.key} className="border-t border-line align-top">
              <td className="py-1.5 pr-3 font-mono text-xs text-ink-sub">{row.key}</td>
              <td className="break-all py-1.5 pr-3 font-mono text-xs text-ink-sub">{row.before}</td>
              <td className="break-all py-1.5 font-mono text-xs text-ink">{row.after}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

interface RollbackModalProps {
  config: SystemConfig
  log: ConfigChangeLog
  onClose: () => void
  onDone: () => void
}

/**
 * RollbackModal 回滚到某次改动之前:写清回滚后各字段会变成什么。
 * 回滚本身也记一条新的变更,随时可以再回滚回来;凭据字段按当时的加密值写回,不显示明文。
 */
function RollbackModal({ config, log, onClose, onDone }: RollbackModalProps) {
  const [working, setWorking] = useState(false)
  const [error, setError] = useState<string>()
  // 回滚后的值就是这次改动之前的值:相对现在的变化 = 现在的值 → 改动前的值
  const rows = changeRows(config.value, log.old_value)

  /** submit 带当前版本回滚,别人在这期间改过就会被拒绝 */
  const submit = async () => {
    setWorking(true)
    setError(undefined)
    try {
      await api.admin.rollbackConfig(config.key, {
        scope: AdminScope.GLOBAL,
        version: config.version,
        change_log_id: log.id,
      })
      toast.success('已回滚,改动立即对全平台生效')
      onDone()
    } catch (cause) {
      setError(userFacingErrorMessage(cause, '回滚没有成功。如果这一项刚被别人改过,请刷新后再试。'))
    } finally {
      setWorking(false)
    }
  }

  return (
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>回滚到 {formatDateTimeSeconds(log.created_at)} 这次改动之前</ModalTitle>
          <ModalDescription>
            回滚后立即对全平台生效,并记一条新的变更,随时可以再回滚回来。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          {rows.length === 0 ? (
            <p className="text-sm text-ink-sub">
              现在的内容和那次改动之前一样,回滚不会改变任何值。
            </p>
          ) : (
            <div>
              <p className="text-sm text-ink">回滚后会这样变化:</p>
              <ChangeTable
                rows={rows.map((row) => ({ key: row.key, before: row.before, after: row.after }))}
              />
            </div>
          )}
          {error ? <Callout tone="danger">{error}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" disabled={working} onClick={onClose}>
            取消
          </Button>
          <Button
            variant="danger"
            leftIcon={RotateCcw}
            loading={working}
            onClick={() => void submit()}
          >
            确认回滚
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
