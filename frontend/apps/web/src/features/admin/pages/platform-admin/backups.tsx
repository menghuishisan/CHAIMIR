// 备份记录页(平台侧栏,/platform-admin/backups)。
//
// 备份由受控运维任务执行(定时任务或运维手动触发),平台端只读结果 ——
// 后端没有给出「从界面触发备份」的接口,这是有意的:备份要在集群侧带着凭据跑,
// 从浏览器发起意味着把那条链路暴露给前端。故本页不做触发按钮,只做结果核对。
//
// 最近一次成功备份的时间是最该盯的数字,故单独放在指标带首位。

import { useMemo } from 'react'
import { CircleCheck, CircleX, Database, Save } from 'lucide-react'
import { BackupStatus, type BackupRecord } from '@chaimir/api-client'
import {
  Breadcrumb,
  Callout,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  Stat,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { formatDateTime, formatDuration, formatFileSize } from '../../../../utils/formatters'
import {
  backupStatusLabel,
  backupStatusTone,
  backupTypeLabel,
} from '../../../../utils/labels/admin'

/**
 * PlatformBackupsPage 呈现备份任务的执行结果。
 */
export default function PlatformBackupsPage() {
  const backups = usePagedResource<BackupRecord>((params) => api.admin.listBackups(params), [])

  const stats = useMemo(() => {
    const list = backups.data ? backups.data.list : []
    const succeeded = list.filter((item) => item.status === BackupStatus.SUCCEEDED)
    const latest = succeeded.reduce<BackupRecord | undefined>((newest, item) => {
      if (!newest) return item
      return new Date(item.started_at) > new Date(newest.started_at) ? item : newest
    }, undefined)
    return {
      latest,
      succeeded: succeeded.length,
      failed: list.filter((item) => item.status === BackupStatus.FAILED).length,
      running: list.filter((item) => item.status === BackupStatus.RUNNING).length,
    }
  }, [backups.data])

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
        kicker={<Breadcrumb items={[{ label: '底层资源' }, { label: '备份记录' }]} />}
        title="备份记录"
        description="备份由运维侧的定时任务执行,这里核对结果。连续失败要尽快查原因。"
        icon={Save}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat
            label="最近一次成功"
            value={stats.latest ? formatDateTime(stats.latest.started_at) : '本页没有成功记录'}
            icon={CircleCheck}
            hint={stats.latest ? formatFileSize(stats.latest.size_bytes) : '往前翻页查看'}
          />
          <Stat label="本页成功" value={stats.succeeded} icon={CircleCheck} />
          <Stat
            label="本页失败"
            value={stats.failed}
            icon={CircleX}
            hint={stats.failed > 0 ? '需要查原因' : '暂无失败'}
          />
          <Stat label="本页进行中" value={stats.running} icon={Database} />
        </div>
      </PageSection>

      <PageSection
        title="执行记录"
        description={`共 ${backups.total} 条,按开始时间从新到旧排列。`}
      >
        <div className="flex flex-col gap-4">
          <ResourceState
            resource={backups}
            emptyIcon={Save}
            emptyTitle="还没有备份记录"
            emptyDescription="备份任务执行后会在这里留下记录。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={backups.page}
                  pageSize={backups.pageSize}
                  total={backups.total}
                  onPageChange={backups.setPage}
                />
              </>
            )}
          </ResourceState>

          <Callout tone="info">
            备份不能从界面触发:执行要在集群侧带着数据库凭据进行,这条链路不对浏览器开放。
            需要临时备份请联系运维。
          </Callout>
        </div>
      </PageSection>
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
