// 任务与下载页(共享入口,{prefix}/tasks)。设计 §7.7:
// 分段「全部 / 导入 / 导出」;列表行显示任务名与文件名、类型、发起时间、状态;进行中的显示进行中指示,
// 完成的给「下载」,失败的写原因。页面可见且有进行中的任务时,每 5 秒自动读取一次状态,
// 全部结束或页面不可见时停止(任务进度没有推送通道,只能轮询;间隔遵守规范 §4.1)。分段写进地址。
//
// 四端共用同一实现:任务归属由后端按会话账号与租户判定。取件经统一文件服务(useTaskArtifactDownload),
// 页面不拼接对象存储地址、不自造文件名。任务快照不带进度百分比与失败原因文字,
// 故进行中用不定量指示、失败写明「尝试 N 次后仍未完成」(设计 §10 数据缺口)。

import { useCallback } from 'react'
import { Download, FileDown, FileUp, ListChecks, LoaderCircle } from 'lucide-react'
import { TRANSFER_STATUS, type TransferChannel, type TransferTask } from '@chaimir/api-client'
import { Button, Callout, Icon, PageHeader, PageScaffold, Pagination, SegmentedControl, Skeleton, StatusIndicator } from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceError } from '../../../components/ResourceState'
import { usePagedResource, usePageLocation, useUrlState, useVisiblePolling } from '../../../hooks'
import { formatDateTime, formatFileSize, formatRelativeOccurrence } from '../../../utils/formatters'
import { transferChannelLabel, transferTaskStatusLabel, transferTaskSubjectLabel } from '../../../utils/labels/transfer'
import { TRANSFER_ACTIVE_STATUSES, transferTaskStatusTone } from '../../../utils/transfer'
import { useTaskArtifactDownload } from '../useTaskArtifactDownload'

/** 有进行中的任务时的自动读取间隔 */
const ACTIVE_POLL_MS = 5_000

/** 地址参数:channel 是任务类型分段 */
const TASKS_QUERY = {
  channel: { defaultValue: 'all', allowed: ['all', 'import', 'export'] },
} as const

/**
 * TransferTasksPage 列出导入导出任务并提供产物下载。
 */
export default function TransferTasksPage() {
  usePageLocation([{ label: '任务与下载' }])
  const [query, setQuery] = useUrlState(TASKS_QUERY)
  const { downloadingId, downloadError, download } = useTaskArtifactDownload()

  const tasks = usePagedResource<TransferTask>(
    (params) =>
      api.transfer.listTasks({
        channel: query.channel === 'all' ? undefined : (query.channel as TransferChannel),
        ...params,
      }),
    [query.channel],
    { urlPageKey: 'page' },
  )
  const list = tasks.data?.list ?? []
  const hasActive = list.some((task) => TRANSFER_ACTIVE_STATUSES.has(task.status))
  // 有进行中的任务且页面可见时每 5 秒重读当前页;全部结束即停
  const reload = tasks.reload
  const poll = useCallback(() => reload(), [reload])
  useVisiblePolling(hasActive, poll, ACTIVE_POLL_MS)

  return (
    <PageScaffold>
      <div className="max-w-4xl">
        <PageHeader title="任务与下载" description="你发起的导入和导出都在这里,处理完成后可以下载结果文件。" />

        <SegmentedControl
          aria-label="按任务类型筛选"
          options={[
            { value: 'all', label: '全部' },
            { value: 'import', label: '导入' },
            { value: 'export', label: '导出' },
          ]}
          value={query.channel}
          onValueChange={(value) => setQuery({ channel: value })}
          className="mb-5"
        />

        {downloadError ? (
          <Callout tone="danger" className="mb-4">
            {downloadError}
          </Callout>
        ) : null}

        {tasks.status === 'error' ? (
          <ResourceError
            message={tasks.error?.message ?? '任务暂时读取不到'}
            traceId={tasks.error?.traceId}
            onRetry={tasks.reload}
          />
        ) : tasks.status === 'loading' && !tasks.data ? (
          <div aria-busy="true" className="rounded-lg bg-surface p-4 shadow-xs">
            <Skeleton variant="line" lines={5} />
          </div>
        ) : list.length === 0 ? (
          <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
            <Icon icon={ListChecks} size="lg" className="text-ink-faint" />
            <p className="mt-3 text-md font-semibold text-ink">还没有任务</p>
            <p className="mt-1.5 text-sm text-ink-sub">在各页面发起导入或导出后,进度和结果文件会显示在这里。</p>
          </div>
        ) : (
          <>
            <ul className="divide-y divide-line overflow-hidden rounded-lg bg-surface shadow-xs" aria-live="polite">
              {list.map((task) => (
                <li key={task.task_id}>
                  <TaskRow
                    task={task}
                    downloading={downloadingId === task.task_id}
                    onDownload={() => void download(task)}
                  />
                </li>
              ))}
            </ul>
            <div className="mt-4">
              <Pagination page={tasks.page} pageSize={tasks.pageSize} total={tasks.total} onPageChange={tasks.setPage} />
            </div>
            {hasActive ? (
              <p className="mt-3 text-sm text-ink-sub">有任务在处理,页面会每 5 秒更新一次状态。</p>
            ) : null}
          </>
        )}
      </div>
    </PageScaffold>
  )
}

/** TaskRow 一条任务:类型图标、任务名与文件名、发起时间、状态;完成给下载,失败写原因 */
function TaskRow({ task, downloading, onDownload }: { task: TransferTask; downloading: boolean; onDownload: () => void }) {
  const active = TRANSFER_ACTIVE_STATUSES.has(task.status)
  const failed = task.status === TRANSFER_STATUS.FAILED
  const done = task.status === TRANSFER_STATUS.SUCCEEDED
  return (
    <div className="flex flex-wrap items-center gap-3 px-4 py-3.5 sm:flex-nowrap">
      <span className="grid size-10 shrink-0 place-items-center rounded-md bg-surface-hover text-ink-sub">
        <Icon icon={task.channel === 'import' ? FileUp : FileDown} size="md" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-base font-medium text-ink">
          {transferTaskSubjectLabel(task.subject)}
          <span className="ml-2 text-sm font-normal text-ink-sub">{transferChannelLabel(task.channel)}</span>
        </p>
        <p className="truncate text-sm text-ink-sub">
          <span title={task.file_name}>{task.file_name}</span>
          {' · '}
          <time dateTime={task.created_at} title={formatDateTime(task.created_at)}>
            {formatRelativeOccurrence(task.created_at)}
          </time>
          发起
          {done && task.artifact_size ? ` · ${formatFileSize(task.artifact_size)}` : ''}
        </p>
        {failed ? (
          <p className="mt-1 text-sm text-danger">
            尝试 {task.attempt_count} 次后仍未完成,请检查文件后重新发起;仍不行请联系管理员。
          </p>
        ) : null}
        {task.status === TRANSFER_STATUS.RETRYING ? (
          <p className="mt-1 text-sm text-ink-sub">
            第 {task.attempt_count} / {task.max_attempts} 次尝试,稍后自动重试
          </p>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-3">
        {active ? (
          <span className="flex items-center gap-1.5 text-sm text-ink-sub">
            <Icon icon={LoaderCircle} size="sm" className="animate-spin text-primary" />
            {transferTaskStatusLabel(task.status)}
          </span>
        ) : (
          <StatusIndicator tone={transferTaskStatusTone(task.status)} label={transferTaskStatusLabel(task.status)} />
        )}
        {done && task.artifact_file_name ? (
          <Button variant="secondary" size="sm" leftIcon={Download} loading={downloading} onClick={onDownload}>
            下载
          </Button>
        ) : null}
      </div>
    </div>
  )
}
