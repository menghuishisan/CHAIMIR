// TaskCenterButton 顶栏任务中心(规范 §6.2 顶栏四件套之一):
// 下拉展示最近的导入导出任务及其真实状态,进行中的任务在按钮上以玉色点提示。
// 数据来自基础层 transfer 模块;角标与面板共用同一份读取,打开面板时重新拉取以拿到最新进度。

import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { ListChecks } from 'lucide-react'
import type { TransferTask } from '@chaimir/api-client'
import {
  Button,
  Empty,
  IconButton,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../app/api'
import { useAsyncResource } from '../../hooks'
import { formatShortDateTime } from '../../utils/formatters'
import { TRANSFER_ACTIVE_STATUSES } from '../../utils/transfer'
import {
  transferTaskStatusLabel,
  transferTaskStatusTone,
  transferTaskSubjectLabel,
} from '../../utils/labels/transfer'

/** 下拉内展示的任务条数:面板是「最近」摘要,全量在任务中心页 */
const RECENT_SIZE = 5

export interface TaskCenterButtonProps {
  /** 任务中心页路径(角色区内) */
  tasksPath: string
}

/**
 * TaskCenterButton 组合任务提示按钮与下拉面板。
 * 角标需要在关闭态常显,故读取放在按钮层;打开面板时重新拉取,面板复用同一份数据。
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

  /** 打开面板时刷新:任务进度随时间变化,展开看到的必须是当前状态 */
  useEffect(() => {
    if (open) reload()
  }, [open, reload])

  // 首次读取完成前没有任务数据,此时按钮不带提示点
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
            aria-label={activeCount > 0 ? `任务中心,${activeCount} 个任务进行中` : '任务中心'}
          />
          {activeCount > 0 ? (
            <span
              aria-hidden="true"
              className="pointer-events-none absolute right-1 top-1 size-2 rounded-full bg-accent"
            />
          ) : null}
        </span>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80 p-0">
        <div className="flex flex-col">
          <div className="border-b border-line px-4 py-3 text-sm font-medium text-ink">
            导入导出任务
          </div>

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
                  <p className="font-mono text-xs text-ink-faint">
                    如需帮助,请提供编号 {tasks.error.traceId}
                  </p>
                ) : null}
                <Button variant="outline" size="sm" onClick={reload}>
                  重新加载
                </Button>
              </div>
            ) : null}

            {tasks.status === 'empty' ? (
              <Empty
                icon={ListChecks}
                title="暂无任务"
                description="发起导入或导出后,进度会显示在这里。"
              />
            ) : null}

            {tasks.status === 'success' && tasks.data
              ? tasks.data.list.map((task) => <TaskRow key={task.task_id} task={task} />)
              : null}
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

/**
 * TaskRow 渲染单条任务:业务名 + 真实状态 + 最近更新时间。
 */
function TaskRow({ task }: { task: TransferTask }) {
  return (
    <div className="flex flex-col gap-1 border-b border-line px-4 py-3 last:border-b-0">
      <span className="flex items-center justify-between gap-2">
        <span className="min-w-0 flex-1 truncate text-sm text-ink">
          {transferTaskSubjectLabel(task.subject)}
        </span>
        <StatusIndicator
          tone={transferTaskStatusTone(task.status)}
          label={transferTaskStatusLabel(task.status)}
          loading={task.status === 'running'}
        />
      </span>
      <span className="font-mono text-xs text-ink-faint">
        {formatShortDateTime(task.updated_at)}
      </span>
    </div>
  )
}
