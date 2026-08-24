// 备份记录页(平台侧栏,/platform-admin/backups)。
//
// 备份由受控运维任务执行(定时任务或运维手动触发),平台端只读结果 ——
// 后端没有给出「从界面触发备份」的接口,这是有意的:备份要在集群侧带着凭据跑,
// 从浏览器发起意味着把那条链路暴露给前端。故本页不做触发按钮,只做结果核对。
//
// 最近一次成功备份的时间是最该盯的数字,故单独放在指标带首位。

import { Save } from 'lucide-react'
import { BackupStatus, type BackupRecord } from '@chaimir/api-client'
import {
  Breadcrumb,
  Callout,
  DataPanel,
  MetricStrip,
  PageHeader,
  PageScaffold,
  Pagination,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatDateTime, formatDuration, formatFileSize } from '../../../../utils/formatters'
import { backupStatusLabel, backupTypeLabel } from '../../../../utils/labels/admin'
import { backupStatusTone } from '../../statusPresentation'

/**
 * PlatformBackupsPage 呈现备份任务的执行结果。
 */
export default function PlatformBackupsPage() {
  const backups = usePagedResource<BackupRecord>((params) => api.admin.listBackups(params), [])

  // 指标带取服务端全量口径。「最近一次成功」单独按成功结果取第一条:
  // 只看当前页会在这一页恰好没有成功记录时误报「暂无成功记录」。
  const latestSucceeded = useAsyncResource(
    () => api.admin.listBackups({ status: BackupStatus.SUCCEEDED, page: 1, size: 1 }),
    [],
    () => false,
  )
  const succeededCount = useResourceTotal(
    (params) => api.admin.listBackups({ status: BackupStatus.SUCCEEDED, ...params }),
    [],
  )
  const failedCount = useResourceTotal(
    (params) => api.admin.listBackups({ status: BackupStatus.FAILED, ...params }),
    [],
  )
  const runningCount = useResourceTotal(
    (params) => api.admin.listBackups({ status: BackupStatus.RUNNING, ...params }),
    [],
  )
  const latest = latestSucceeded.data ? latestSucceeded.data.list[0] : undefined

  const columns: TableColumn<BackupRecord>[] = [
    {
      key: 'started_at',
      header: '开始时间',
      render: (record) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(record.started_at)}
        </span>
      ),
    },
    {
      key: 'type',
      header: '备份类型',
      render: (record) => <span className="text-ink">{backupTypeLabel(record.type)}</span>,
    },
    {
      key: 'status',
      header: '结果',
      render: (record) => (
        <StatusIndicator
          tone={backupStatusTone(record.status)}
          label={backupStatusLabel(record.status)}
        />
      ),
    },
    {
      key: 'size_bytes',
      header: '备份大小',
      align: 'right',
      render: (record) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatFileSize(record.size_bytes)}
        </span>
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
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }]} />}
        title="备份记录"
        description="备份由运维侧的定时任务执行,这里核对结果。连续失败要尽快查原因。"
        icon={Save}
      />

      {/*
        归族:资源列表族(§6.5.3 第 ①)。备份记录是「在一批同类记录里核对」,
        故指标退为一行内联摘要,不占 Stat 大卡 —— 那是看板族才保留的形态。
        四项都取服务端全量口径(§6.5.4),不用当前页切片统计。
      */}
      <MetricStrip
        label="备份结果摘要"
        className="mb-5"
        items={[
          {
            label: '最近一次成功',
            value: latest ? formatDateTime(latest.started_at) : '暂无',
            hint: latest ? formatFileSize(latest.size_bytes) : '备份成功后显示时间与大小',
          },
          { label: '已成功', value: succeededCount ?? '—', hint: '不受下方翻页影响' },
          {
            label: '已失败',
            value: failedCount ?? '—',
            hint: failedCount === 0 ? '暂无失败' : '需要查原因',
          },
          { label: '进行中', value: runningCount ?? '—', hint: '正在执行的备份' },
        ]}
      />

      {/* 数据表与分页同处一块抬起片(§6.5.2)。本页没有筛选项:后端列表只按状态过滤,
          而四种状态的条数已在上方摘要里给出,再排一条筛选井只是把同一件事说第二遍。 */}
      <DataPanel
        label="执行记录"
        footer={
          <Pagination
            page={backups.page}
            pageSize={backups.pageSize}
            total={backups.total}
            onPageChange={backups.setPage}
          />
        }
      >
        <ResourceState
          resource={backups}
          emptyIcon={Save}
          emptyTitle="还没有备份记录"
          emptyDescription="备份任务执行后会在这里留下记录。"
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(page) => (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(item) => item.id}
              elevated={false}
              // <md 换行卡(§6.4.1 规则 3):开始时间一行、类型与大小耗时一行,结果在右
              mobileCard={(item) => ({
                title: formatDateTime(item.started_at),
                meta: `${backupTypeLabel(item.type)} · ${formatFileSize(item.size_bytes)} · ${durationText(item)}`,
                badge: (
                  <StatusIndicator
                    tone={backupStatusTone(item.status)}
                    label={backupStatusLabel(item.status)}
                  />
                ),
              })}
            />
          )}
        </ResourceState>
      </DataPanel>

      <Callout tone="info" className="mt-4">
        备份不能从界面触发:执行要在集群侧带着数据库凭据进行,这条链路不对浏览器开放。
        需要临时备份请联系运维。
      </Callout>
    </PageScaffold>
  )
}

/**
 * durationText 给出备份耗时;还没结束的显示进行中。
 * 秒级完成的备份按「不足 1 分钟」表达 —— 通用时长文案把 0 秒当成没记录,
 * 但这里 0 秒是真的很快,不是缺数据。
 */
function durationText(record: BackupRecord): string {
  if (!record.finished_at) return '进行中'
  const seconds = Math.round(
    (new Date(record.finished_at).getTime() - new Date(record.started_at).getTime()) / 1000,
  )
  if (!Number.isFinite(seconds) || seconds < 0) return '—'
  return seconds < 60 ? '不足 1 分钟' : formatDuration(seconds)
}
