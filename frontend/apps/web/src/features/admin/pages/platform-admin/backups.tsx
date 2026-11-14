// 备份记录(平台侧栏,/platform-admin/backups)。设计 §7.6:
// 顶部一行「最近一次成功备份:N 小时前 · 大小」(超过 48 小时没有成功用警示色);状态分段(带数字,写进地址);
// 表格列为开始时间、类型、状态、耗时、快照大小(耗时由开始和结束时间算出)。
// 「已成功 / 已失败 / 进行中」指标条与健康判断卡去掉(与分段计数重复)。
//
// 备份由受控运维任务执行(定时或运维手动触发),平台端只读结果:从浏览器发起备份意味着把带凭据的集群链路
// 暴露给前端,所以这里没有触发按钮。

import { Save } from 'lucide-react'
import { BackupStatus, type BackupRecord } from '@chaimir/api-client'
import {
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import {
  useAsyncResource,
  usePagedResource,
  useUrlState,
} from '../../../../hooks'
import { segmentCounts } from '../../../../utils/facets'
import { formatDateTimeSeconds, formatDuration, formatFileSize } from '../../../../utils/formatters'
import { backupStatusLabel, backupTypeLabel } from '../../../../utils/labels/admin'
import { backupFreshness } from '../../backupFreshness'
import { backupStatusTone } from '../../statusPresentation'

/** 地址参数:status 分段(默认全部);页码在 page */
const BACKUPS_QUERY = {
  status: { defaultValue: 'all', allowed: ['all', 'running', 'succeeded', 'failed'] },
} as const

const STATUS_SEGMENTS = [
  { value: 'all', label: '全部' },
  { value: 'running', label: '进行中', status: BackupStatus.RUNNING },
  { value: 'succeeded', label: '已成功', status: BackupStatus.SUCCEEDED },
  { value: 'failed', label: '已失败', status: BackupStatus.FAILED },
] as const

/** PlatformBackupsPage 备份执行记录:最近一次成功在最上面,按状态核对 */
export default function PlatformBackupsPage() {
  const [query, setQuery] = useUrlState(BACKUPS_QUERY)
  const segment = STATUS_SEGMENTS.find((item) => item.value === query.status)
  const statusFilter = segment && 'status' in segment ? segment.status : undefined

  const backups = usePagedResource<BackupRecord>(
    (params) => api.admin.listBackups({ status: statusFilter, ...params }),
    [statusFilter],
    { urlPageKey: 'page' }
  )
  // 最近一次成功单独按成功结果取第一条(列表按时间倒序),不受当前分段与页码影响
  const latestSucceeded = useAsyncResource(
    () => api.admin.listBackups({ status: BackupStatus.SUCCEEDED, page: 1, size: 1 }),
    [backups.refreshVersion],
    () => false
  )
  // 分段计数取列表接口的 facets(接口总览「分段计数」);读取中或失败时不显示数字
  const facets = backups.status === 'success' || backups.status === 'empty' ? backups.data?.facets : undefined
  const counts = segmentCounts(facets, 'status', STATUS_SEGMENTS, (item) => ('status' in item ? item.status : undefined))

  const columns: TableColumn<BackupRecord>[] = [
    {
      key: 'started_at',
      header: '开始时间',
      render: (record) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTimeSeconds(record.started_at)}
        </span>
      ),
    },
    {
      key: 'type',
      header: '类型',
      render: (record) => <span className="text-ink">{backupTypeLabel(record.type)}</span>,
    },
    {
      key: 'status',
      header: '状态',
      render: (record) => (
        <StatusIndicator
          tone={backupStatusTone(record.status)}
          label={backupStatusLabel(record.status)}
        />
      ),
    },
    {
      key: 'duration',
      header: '耗时',
      align: 'right',
      render: (record) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {durationText(record)}
        </span>
      ),
    },
    {
      key: 'size_bytes',
      header: '快照大小',
      align: 'right',
      render: (record) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {record.status === BackupStatus.SUCCEEDED ? formatFileSize(record.size_bytes) : '—'}
        </span>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader title="备份记录" />
      <LatestBackupLine resource={latestSucceeded} />

      <SegmentedControl
        aria-label="按备份状态筛选"
        options={STATUS_SEGMENTS.map((item) => ({
          value: item.value,
          label: item.label,
          count: counts[item.value],
        }))}
        value={query.status}
        onValueChange={(value) => setQuery({ status: value })}
        className="mb-4 max-w-full overflow-x-auto"
      />

      <ResourceState
        resource={backups}
        emptyIcon={Save}
        emptyTitle={query.status === 'all' ? '还没有备份记录' : '这个状态下没有记录'}
        emptyDescription={
          query.status === 'all' ? '备份任务执行后会在这里留下记录。' : '换个状态看看。'
        }
        skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
      >
        {(page) => (
          <>
            <Table
              columns={columns}
              data={page.list}
              rowKey={(item) => item.id}
              // <md 换行卡:开始时间一行;类型、耗时与大小一行;状态在右
              mobileCard={(item) => ({
                title: formatDateTimeSeconds(item.started_at),
                meta: `${backupTypeLabel(item.type)} · ${durationText(item)}${item.status === BackupStatus.SUCCEEDED ? ` · ${formatFileSize(item.size_bytes)}` : ''}`,
                badge: (
                  <StatusIndicator
                    tone={backupStatusTone(item.status)}
                    label={backupStatusLabel(item.status)}
                  />
                ),
              })}
            />
            <div className="mt-4">
              <Pagination
                page={backups.page}
                pageSize={backups.pageSize}
                total={backups.total}
                onPageChange={backups.setPage}
              />
            </div>
          </>
        )}
      </ResourceState>
    </PageScaffold>
  )
}

/** LatestBackupLine 「最近一次成功备份:N 小时前 · 大小」;超过 48 小时或从未成功用警示色 */
function LatestBackupLine({
  resource,
}: {
  resource: ReturnType<typeof useAsyncResource<{ list: BackupRecord[] }>>
}) {
  if (resource.status === 'loading')
    return <Skeleton variant="line" width="half" className="mb-4" />
  if (resource.status === 'error') {
    return <p className="mb-4 text-sm text-ink-sub">最近一次成功备份暂时读不到。</p>
  }
  const latest = resource.data?.list[0]
  if (!latest) {
    return (
      <p className="mb-4 text-sm font-medium text-warning">
        还没有成功的备份,请联系运维确认备份任务。
      </p>
    )
  }
  const freshness = backupFreshness(latest.started_at)
  return (
    <p
      className={`mb-4 text-sm ${freshness.stale ? 'font-medium text-warning' : 'text-ink'}`}
      title={formatDateTimeSeconds(latest.started_at)}
    >
      最近一次成功备份:{freshness.text} · {formatFileSize(latest.size_bytes)}
      {freshness.stale ? '。已经超过 48 小时没有成功备份,请联系运维检查。' : ''}
    </p>
  )
}

/**
 * durationText 备份耗时,由开始和结束时间算出;还没结束的写进行中。
 * 秒级完成的备份写「不足 1 分钟」:通用时长文案把 0 秒当成没有记录,这里 0 秒是真的很快。
 */
function durationText(record: BackupRecord): string {
  if (!record.finished_at) return '进行中'
  const seconds = Math.round(
    (new Date(record.finished_at).getTime() - new Date(record.started_at).getTime()) / 1000
  )
  if (!Number.isFinite(seconds) || seconds < 0) return '—'
  return seconds < 60 ? '不足 1 分钟' : formatDuration(seconds)
}
