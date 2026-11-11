// TaskCenterButton 顶栏「任务与下载」(设计 §7.8):有进行中的导入导出任务时按钮带数字;
// 弹出层列出最近任务与状态,已完成的直接给「下载」,底部「查看全部」。
// 弹出层打开时重读一次:任务进度随时间变化,展开看到的必须是当前状态。

import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { Download, ListChecks } from 'lucide-react'
import { TRANSFER_STATUS, type TransferTask } from '@chaimir/api-client'
import {
  Button,
  Empty,
  IconButton,
  ListRow,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../app/api'
import { useTaskArtifactDownload } from '../../features/transfer/useTaskArtifactDownload'
import { useAsyncResource } from '../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../utils/formatters'
import { transferTaskStatusLabel, transferTaskSubjectLabel } from '../../utils/labels/transfer'
import { TRANSFER_ACTIVE_STATUSES, transferTaskStatusTone } from '../../utils/transfer'

/** 弹出层里的任务条数:这里是「最近」摘要,全量在任务与下载页 */
const RECENT_SIZE = 5

export interface TaskCenterButtonProps {
  /** 任务与下载页路径(角色区内) */
  tasksPath: string
}

/**
 * TaskCenterButton 组合带数字的按钮与最近任务弹出层。
 * 进行中的数量取自最近一页任务:它只用来提示「有事在跑」,精确的全量在任务与下载页。
 */
export function TaskCenterButton({ tasksPath }: TaskCenterButtonProps) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const tasks = useAsyncResource(
    () => api.transfer.listTasks({ page: 1, size: RECENT_SIZE }),
    [],
    (value) => value.list.length === 0,
  )
  const { reload } = tasks
  const { downloadingId, downloadError, download } = useTaskArtifactDownload()

  /** 打开弹出层时刷新:任务进度随时间变化,展开看到的必须是当前状态 */
  useEffect(() => {
    if (open) reload()
  }, [open, reload])

  // 首次读取完成前没有任务数据,此时按钮不带数字
  const activeCount = tasks.data
    ? tasks.data.list.filter((task) => TRANSFER_ACTIVE_STATUSES.has(task.status)).length
    : 0

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <span className="relative inline-flex">
          <IconButton
            variant="on-dark"
            icon={ListChecks}
            aria-label={activeCount > 0 ? `任务与下载,${activeCount} 个进行中` : '任务与下载'}
            className="border-transparent"
          />
          {activeCount > 0 ? (
            // 数字由 aria-label 承载,角标本身对读屏隐藏
            <span
              aria-hidden="true"
              className="pointer-events-none absolute -right-0.5 -top-0.5 flex min-w-4 items-center justify-center rounded-full bg-accent px-1 text-xs font-semibold leading-4 text-substrate"
            >
              {activeCount}
            </span>
          ) : null}
        </span>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-topbar-popover p-0">
        <div className="flex flex-col">
          <div className="border-b border-line px-4 py-2.5 text-sm font-semibold text-ink">任务与下载</div>

          <div className="max-h-96 overflow-y-auto">
            {tasks.status === 'loading' ? (
              <div className="flex flex-col gap-3 p-4">
                <Skeleton variant="line" lines={3} />
              </div>
            ) : null}

            {tasks.status === 'error' ? (
              <div className="flex flex-col items-center gap-3 px-4 py-8 text-center">
                <p className="text-sm text-ink-sub">暂时无法获取任务进度,请稍后重试。</p>
                {tasks.error?.traceId ? (
                  <p className="font-mono text-xs text-ink-sub">如需帮助,请提供编号 {tasks.error.traceId}</p>
                ) : null}
                <Button variant="outline" size="sm" onClick={reload}>
                  重新加载
                </Button>
              </div>
            ) : null}

            {tasks.status === 'empty' ? (
              <Empty icon={ListChecks} title="还没有任务" description="发起导入或导出后,进度和可下载的文件会显示在这里。" />
            ) : null}

            {tasks.status === 'success' && tasks.data ? (
              <ul className="divide-y divide-line">
                {tasks.data.list.map((task) => (
                  <li key={task.task_id}>
                    <TaskRow
                      task={task}
                      downloading={downloadingId === task.task_id}
                      onDownload={() => void download(task)}
                    />
                  </li>
                ))}
              </ul>
            ) : null}

            {downloadError ? (
              <p role="alert" className="border-t border-line px-4 py-2.5 text-sm text-danger">
                {downloadError}
              </p>
            ) : null}
          </div>

          <div className="border-t border-line p-2">
            <Button
              variant="ghost"
              size="sm"
              className="w-full"
              onClick={() => {
                setOpen(false)
                navigate(tasksPath)
              }}
            >
              查看全部
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}

interface TaskRowProps {
  task: TransferTask
  downloading: boolean
  onDownload: () => void
}

/**
 * TaskRow 一条任务:业务名 + 最近更新时间,右侧是状态;已完成且有文件的直接给「下载」。
 */
function TaskRow({ task, downloading, onDownload }: TaskRowProps) {
  const downloadable = task.status === TRANSFER_STATUS.SUCCEEDED && Boolean(task.artifact_file_name)
  return (
    <ListRow
      title={transferTaskSubjectLabel(task.subject)}
      meta={
        <time dateTime={task.updated_at} title={formatDateTime(task.updated_at)}>
          {formatRelativeOccurrence(task.updated_at)}
        </time>
      }
      trailing={
        downloadable ? (
          <Button variant="secondary" size="sm" leftIcon={Download} loading={downloading} onClick={onDownload}>
            下载
          </Button>
        ) : (
          <StatusIndicator
            tone={transferTaskStatusTone(task.status)}
            label={transferTaskStatusLabel(task.status)}
            loading={task.status === 'running'}
          />
        )
      }
    />
  )
}
