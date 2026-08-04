// 任务与下载页(共享入口,{prefix}/tasks)。
// 四端共用同一实现:transfer 是横切能力,任务归属由后端按会话账号与租户判定
// (平台身份只见 tenant_id=0 的平台任务,租户账号只见本租户任务)。
//
// 取件经统一文件服务:downloadArtifact 内部签发一次性授权并交给 api.storage 消费,
// 保存文件名取自响应头,页面不拼接对象存储地址、不自造文件名。

import { useCallback, useMemo, useState } from 'react'
import { Download, FileDown, ListChecks, RefreshCw } from 'lucide-react'
import { TRANSFER_STATUS, type TransferChannel, type TransferTask } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  IconButton,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Stat,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { usePagedResource } from '../../../hooks'
import { formatShortDateTime } from '../../../utils/formatters'
import {
  transferChannelLabel,
  transferTaskStatusLabel,
  transferTaskStatusTone,
  transferTaskSubjectLabel,
} from '../../../utils/labels/transfer'
import { formatFileSize } from '../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

/** 通道筛选项:值为空串表示不过滤。 */
const CHANNEL_FILTERS = [
  { value: '', label: '全部' },
  { value: 'import', label: '导入' },
  { value: 'export', label: '导出' },
] as const

/** 处理中的状态:决定指标带的「进行中」计数与刷新提示。 */
const ACTIVE_STATUSES: ReadonlySet<TransferTask['status']> = new Set([
  TRANSFER_STATUS.PENDING,
  TRANSFER_STATUS.RUNNING,
  TRANSFER_STATUS.RETRYING,
])

/**
 * TransferTasksPage 列出导入导出任务并提供产物下载。
 */
export default function TransferTasksPage() {
  const [channel, setChannel] = useState<string>('')
  const [downloadingId, setDownloadingId] = useState<string>()
  const [checkingId, setCheckingId] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const tasks = usePagedResource<TransferTask>(
    (params) =>
      api.transfer
        .listTasks({
          channel: channel ? (channel as TransferChannel) : undefined,
          ...params,
        })
        // transfer 列表响应是 { items, page, size } 而非统一分页信封:
        // 在此收敛成分页 Hook 的形状,总数按当前页推算(后端未提供 total)
        .then((response) => ({
          list: response.items,
          total: response.items.length + (response.page - 1) * response.size,
          page: response.page,
          size: response.size,
        })),
    [channel],
  )

  /** downloadArtifact 取件:每次重新签发一次性授权,不复用旧 token。 */
  const downloadArtifact = useCallback(async (task: TransferTask) => {
    setDownloadingId(task.task_id)
    setActionError(undefined)
    try {
      const file = await api.transfer.downloadArtifact(task.task_id)
      const url = URL.createObjectURL(file.blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = file.fileName
      anchor.click()
      URL.revokeObjectURL(url)
      toast.success('文件已开始下载')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '下载没有完成,请稍后重试。'))
    } finally {
      setDownloadingId(undefined)
    }
  }, [])

  /**
   * checkTask 只重读这一条任务的进度。
   * 处理中的任务由后端异步推进,页面不轮询(规范 §4.1);想知道「这一条到哪了」时
   * 单条重读比整页刷新轻,也不会把已经翻到的页码带回第一页。
   */
  const checkTask = useCallback(
    async (task: TransferTask) => {
      setCheckingId(task.task_id)
      setActionError(undefined)
      try {
        const current = await api.transfer.getTask(task.task_id)
        toast.success(
          `${transferTaskSubjectLabel(current.subject)}:${transferTaskStatusLabel(current.status)}`,
        )
        // 状态变了就整页重读一次:表格里的行要跟着更新
        if (current.status !== task.status) tasks.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '任务进度暂时读不到,请稍后重试。'))
      } finally {
        setCheckingId(undefined)
      }
    },
    [tasks],
  )

  const stats = useMemo(() => {
    const list = tasks.data ? tasks.data.list : []
    return {
      activeCount: list.filter((task) => ACTIVE_STATUSES.has(task.status)).length,
      succeededCount: list.filter((task) => task.status === TRANSFER_STATUS.SUCCEEDED).length,
      failedCount: list.filter((task) => task.status === TRANSFER_STATUS.FAILED).length,
    }
  }, [tasks.data])

  const columns: TableColumn<TransferTask>[] = [
    {
      key: 'subject',
      header: '任务',
      render: (task) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{transferTaskSubjectLabel(task.subject)}</div>
          <div className="truncate text-xs text-ink-sub">{task.file_name}</div>
        </div>
      ),
    },
    {
      key: 'channel',
      header: '类型',
      render: (task) => <Badge tone="neutral">{transferChannelLabel(task.channel)}</Badge>,
    },
    {
      key: 'status',
      header: '状态',
      render: (task) => (
        <div className="flex flex-col gap-1">
          <StatusIndicator
            tone={transferTaskStatusTone(task.status)}
            label={transferTaskStatusLabel(task.status)}
            loading={task.status === TRANSFER_STATUS.RUNNING}
          />
          {task.status === TRANSFER_STATUS.RETRYING ? (
            <span className="text-xs text-ink-sub">
              第 {task.attempt_count} / {task.max_attempts} 次尝试
            </span>
          ) : null}
        </div>
      ),
    },
    {
      key: 'artifact_size',
      header: '文件大小',
      align: 'right',
      mono: true,
      render: (task) => (task.artifact_size ? formatFileSize(task.artifact_size) : '—'),
    },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (task) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(task.updated_at)}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (task) => (
        <div className="flex items-center justify-end gap-1">
          {ACTIVE_STATUSES.has(task.status) ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={RefreshCw}
              loading={checkingId === task.task_id}
              onClick={() => void checkTask(task)}
            >
              查看进度
            </Button>
          ) : null}
          {task.status === TRANSFER_STATUS.SUCCEEDED && task.artifact_file_name ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={Download}
              loading={downloadingId === task.task_id}
              onClick={() => void downloadArtifact(task)}
            >
              下载
            </Button>
          ) : !ACTIVE_STATUSES.has(task.status) ? (
            <span className="text-sm text-ink-faint">无可下载文件</span>
          ) : null}
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '任务与下载' }]} />}
        title="任务与下载"
        description="你发起的导入和导出都在这里。处理完成后可以下载结果文件。"
        icon={ListChecks}
        actions={
          <IconButton
            variant="outline"
            icon={RefreshCw}
            aria-label="刷新任务进度"
            onClick={tasks.reload}
          />
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat
            label="本页进行中"
            value={stats.activeCount}
            icon={RefreshCw}
            hint={stats.activeCount > 0 ? '处理完成后可刷新查看' : undefined}
          />
          <Stat label="本页已完成" value={stats.succeededCount} icon={FileDown} />
          <Stat label="本页处理失败" value={stats.failedCount} icon={ListChecks} />
        </div>
      </PageSection>

      <PageSection
        title="任务记录"
        description="按更新时间从新到旧排列。"
        actions={
          <SegmentedControl
            aria-label="按任务类型筛选"
            size="sm"
            options={CHANNEL_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
            value={channel}
            onValueChange={setChannel}
          />
        }
      >
        <div className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={tasks}
            emptyIcon={ListChecks}
            emptyTitle="暂无任务"
            emptyDescription="发起导入或导出后,任务进度和结果文件会显示在这里。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(task) => task.task_id} />
                <Pagination
                  page={tasks.page}
                  pageSize={tasks.pageSize}
                  total={tasks.total}
                  onPageChange={tasks.setPage}
                />
              </>
            )}
          </ResourceState>
        </div>
      </PageSection>
    </PageScaffold>
  )
}
