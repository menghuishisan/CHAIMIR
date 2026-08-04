// 导入记录(账号管理页内页)。
//
// 展示服务端持久化的导入批次:每次导入的文件、总数、成功与失败数。
// 归属账号管理而不是侧栏(对齐清单 §3.3:导入记录是账号管理内页)。

import { useMemo } from 'react'
import { FileClock } from 'lucide-react'
import { ImportBatchStatus, ImportTarget, type ImportBatch } from '@chaimir/api-client'
import {
  Badge,
  PageSection,
  StatusIndicator,
  Table,
  type StatusTone,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'

/** 导入对象文案:只在本页用到,不进全局 labels(其他端无导入记录视图)。 */
const TARGET_LABELS: Record<ImportTarget, string> = {
  [ImportTarget.TEACHER]: '教师账号',
  [ImportTarget.STUDENT]: '学生账号',
  [ImportTarget.ORG]: '组织架构',
}

/** 批次状态文案与语义色。 */
const BATCH_STATUS_LABELS: Record<ImportBatchStatus, string> = {
  [ImportBatchStatus.PROCESSING]: '处理中',
  [ImportBatchStatus.COMPLETED]: '已完成',
  [ImportBatchStatus.FAILED]: '处理失败',
}

const BATCH_STATUS_TONES: Record<ImportBatchStatus, StatusTone> = {
  [ImportBatchStatus.PROCESSING]: 'info',
  [ImportBatchStatus.COMPLETED]: 'success',
  [ImportBatchStatus.FAILED]: 'danger',
}

/**
 * AccountImportBatches 列出历史导入批次。
 */
export function AccountImportBatches() {
  const batches = useAsyncResource(
    () => api.identity.listAccountImportBatches(),
    [],
    (value) => value.length === 0,
  )

  const columns: TableColumn<ImportBatch>[] = useMemo(
    () => [
      {
        key: 'file_name',
        header: '导入文件',
        render: (batch) => (
          <div className="min-w-0">
            <div className="truncate font-medium text-ink">{batch.file_name}</div>
            <div className="truncate text-xs text-ink-sub">{TARGET_LABELS[batch.target_type]}</div>
          </div>
        ),
      },
      {
        key: 'created_at',
        header: '导入时间',
        render: (batch) => (
          <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
            {formatDateTime(batch.created_at)}
          </span>
        ),
      },
      { key: 'total', header: '总行数', align: 'right', mono: true },
      {
        key: 'result',
        header: '结果',
        render: (batch) => (
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge tone="success">成功 {batch.success}</Badge>
            {batch.failed > 0 ? <Badge tone="danger">失败 {batch.failed}</Badge> : null}
          </div>
        ),
      },
      {
        key: 'status',
        header: '状态',
        render: (batch) => (
          <StatusIndicator
            tone={BATCH_STATUS_TONES[batch.status]}
            label={BATCH_STATUS_LABELS[batch.status]}
            loading={batch.status === ImportBatchStatus.PROCESSING}
          />
        ),
      },
    ],
    [],
  )

  return (
    <PageSection title="导入记录" description="按导入时间从新到旧排列。">
      <ResourceState
        resource={batches}
        emptyIcon={FileClock}
        emptyTitle="还没有导入记录"
        emptyDescription="用批量导入开通账号后,每次导入的结果会记录在这里。"
        skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
      >
        {(list) => <Table columns={columns} data={list} rowKey={(item) => item.id} />}
      </ResourceState>
    </PageSection>
  )
}
