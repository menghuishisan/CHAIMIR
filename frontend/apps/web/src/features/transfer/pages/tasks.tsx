// 任务与下载页(共享入口,{prefix}/tasks)。
// 四端共用同一实现:transfer 是横切能力,任务归属由后端按会话账号与租户判定
// (平台身份只见 tenant_id=0 的平台任务,租户账号只见本租户任务)。
//
// 取件经统一文件服务:downloadArtifact 内部签发一次性授权并交给 api.storage 消费,
// 保存文件名取自响应头,页面不拼接对象存储地址、不自造文件名。

import { useCallback, useState } from 'react'
import { Download, ListChecks, RefreshCw } from 'lucide-react'
import { TRANSFER_STATUS, type TransferChannel, type TransferTask } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  DataPanel,
  FilterBar,
  FilterField,
  IconButton,
  MetricStrip,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { usePagedResource, useResourceTotal } from '../../../hooks'
import { downloadAttachment } from '../../../utils/downloadAttachment'
import { formatFileSize, formatShortDateTime } from '../../../utils/formatters'
import {
  transferChannelLabel,
  transferTaskStatusLabel,
  transferTaskSubjectLabel,
} from '../../../utils/labels/transfer'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { TRANSFER_ACTIVE_STATUSES } from '../../../utils/transfer'
import { transferTaskStatusTone } from '../../../utils/transfer'

/** 通道筛选项:值为空串表示不过滤。 */
const CHANNEL_FILTERS = [
  { value: '', label: '全部' },
  { value: 'import', label: '导入' },
  { value: 'export', label: '导出' },
] as const

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
      api.transfer.listTasks({
        channel: channel ? (channel as TransferChannel) : undefined,
        ...params,
      }),
    [channel],
  )

  // 指标带取服务端全量口径,与表格的通道筛选无关:这里回答「我的任务整体如何」,
  // 表格回答「我筛的这一段是什么」。状态全集是 pending/running/retrying/succeeded/failed,
  // 因此「进行中」= 总数 − 已完成 − 失败 是精确减法,不是近似。
  const totalCount = useResourceTotal((params) => api.transfer.listTasks(params), [])
  const succeededCount = useResourceTotal(
    (params) => api.transfer.listTasks({ status: TRANSFER_STATUS.SUCCEEDED, ...params }),
    [],
  )
  const failedCount = useResourceTotal(
    (params) => api.transfer.listTasks({ status: TRANSFER_STATUS.FAILED, ...params }),
    [],
  )
  const activeCount =
    totalCount === undefined || succeededCount === undefined || failedCount === undefined
      ? undefined
      : totalCount - succeededCount - failedCount

  /** downloadArtifact 取件:每次重新签发一次性授权,不复用旧 token。 */
  const downloadArtifact = useCallback(async (task: TransferTask) => {
    setDownloadingId(task.task_id)
    setActionError(undefined)
    try {
      const file = await api.transfer.downloadArtifact(task.task_id)
      downloadAttachment(file)
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
          {TRANSFER_ACTIVE_STATUSES.has(task.status) ? (
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
          ) : !TRANSFER_ACTIVE_STATUSES.has(task.status) ? (
            <span className="text-sm text-ink-faint">无可下载文件</span>
          ) : null}
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
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

      {/* 指标降为内联摘要(§6.5.3 第 ① 族):本页主体是任务记录 */}
      <MetricStrip
        label="任务总量摘要"
        className="mb-5"
        items={[
          { label: '任务总数', value: totalCount ?? '—', hint: '不受下方筛选影响' },
          {
            label: '进行中',
            value: activeCount ?? '—',
            hint:
              activeCount !== undefined && activeCount > 0 ? '处理完成后可刷新查看' : '暂无排队任务',
          },
          { label: '已完成', value: succeededCount ?? '—', hint: '结果文件可下载' },
          { label: '处理失败', value: failedCount ?? '—', hint: '可查看失败原因' },
        ]}
      />

      {/* 动作失败就近内联(§6.7 C) */}
      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {/* 筛选井、数据表、分页同处一块抬起片(§6.5.2) */}
      <DataPanel
        label="任务记录"
        filter={
          <FilterBar label="任务筛选">
            <FilterField label="任务类型" group>
              <SegmentedControl
                aria-label="按任务类型筛选"
                size="sm"
                options={CHANNEL_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={channel}
                onValueChange={setChannel}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={tasks.page}
            pageSize={tasks.pageSize}
            total={tasks.total}
            onPageChange={tasks.setPage}
          />
        }
      >
          <ResourceState
            resource={tasks}
            emptyIcon={ListChecks}
            emptyTitle="暂无任务"
            emptyDescription="发起导入或导出后,任务进度和结果文件会显示在这里。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(task) => task.task_id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):任务名一行、类型与更新时间一行,状态在右
                mobileCard={(task) => ({
                  title: transferChannelLabel(task.channel),
                  meta: `${formatShortDateTime(task.updated_at)}`,
                  badge: (
                    <StatusIndicator
                      tone={transferTaskStatusTone(task.status)}
                      label={transferTaskStatusLabel(task.status)}
                    />
                  ),
                })}
              />
            )}
          </ResourceState>
      </DataPanel>
    </PageScaffold>
  )
}
