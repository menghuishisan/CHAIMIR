// AuditLogPage 是校管「审计日志」与平台「平台审计」共用的审计查询页(设计 §7.7)。
// 布局:工具条(对象类型、动作、操作人、时间范围,平台端另有学校;全部写进地址)+ 右上「导出」;
// 表格列为时间(精确到秒)、操作人、动作、对象、来源地址、追踪编号(可复制),整行打开详情抽屉看完整字段。
// 动作下拉里的数字来自接口返回的分组计数(与当前其他筛选同口径的全量)。只能查询与导出,不能修改或删除。
// 两个入口只在四处不同,由调用方传入:操作人怎么显示与筛选、对象类型清单、学校筛选(只有平台端)、任务与下载页的路径。

import { useMemo, useState, type ReactNode } from 'react'
import { Download, FileText } from 'lucide-react'
import type { AuditLogEntry } from '@chaimir/api-client'
import {
  Button,
  Callout,
  DescriptionList,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  Input,
  PageHeader,
  PageScaffold,
  Pagination,
  Select,
  Table,
  toast,
  type DescriptionItem,
  type TableColumn,
} from '@chaimir/ui'
import { Link } from 'react-router'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { usePagedResource, useUrlState } from '../../../hooks'
import { facetGroup } from '../../../utils/facets'
import { formatDateTimeSeconds, formatRelativeOccurrence } from '../../../utils/formatters'
import {
  auditActionLabel,
  auditActorRoleLabel,
  auditTargetTypeLabel,
} from '../../../utils/labels/admin'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { auditDetailRows, auditObjectHint, type AuditDetailLookups } from '../auditDetail'
import { auditActionFilter, auditActorFilter, auditDayRange } from '../auditFilters'
import { TraceIdCell } from './TraceIdCell'

/** 「不限」在下拉框里的取值(Select 不接受空串作选项值) */
export const AUDIT_ALL = 'all'

/** 地址参数:target 对象类型、action 动作、actor 操作人、school 学校(平台端)、from/to 起止日期(YYYY-MM-DD);页码在 page */
const AUDIT_QUERY = {
  target: { defaultValue: AUDIT_ALL },
  action: { defaultValue: AUDIT_ALL },
  actor: { defaultValue: '' },
  school: { defaultValue: '' },
  from: { defaultValue: '' },
  to: { defaultValue: '' },
} as const

/** 一个对象类型筛选项:值是 target_type,文字取自标签表 */
export interface AuditTargetOption {
  value: string
  label: string
}

/** 操作人筛选:学校端按人筛,平台端没有跨校的人员名单,不提供 */
export interface AuditActorFilter {
  /** 渲染操作人选择器;value 为空串表示不限 */
  render: (value: string, onChange: (accountId: string) => void) => ReactNode
}

export interface AuditLogPageProps {
  title: string
  /** 页头下的一句范围说明 */
  description: string
  /** 对象类型筛选项(不含「全部」) */
  targetOptions: readonly AuditTargetOption[]
  /** 表格里「操作人」一列的内容 */
  renderActor: (entry: AuditLogEntry) => ReactNode
  /** 详情抽屉里「操作人」一行的文字 */
  actorText: (entry: AuditLogEntry) => string
  /** 详情里需要查表的附加项(如学生姓名);不传则这些项不展示 */
  detailLookups?: AuditDetailLookups
  /** 打开一条记录的详情时调用:页面可借此按需读取详情里要查表的项(如那位学生是谁) */
  onDetailOpen?: (entry: AuditLogEntry) => void
  /** 操作人筛选;不传则工具条上没有这一项 */
  actorFilter?: AuditActorFilter
  /** 学校筛选(只有平台端):渲染学校选择器,value 为空串表示全平台;学校端不传 */
  schoolFilter?: AuditActorFilter
  /** 平台端额外的一列与一行(所属学校);学校端不传 */
  tenantColumn?: {
    header: string
    render: (entry: AuditLogEntry) => ReactNode
    text: (entry: AuditLogEntry) => string
  }
  /** 任务与下载页路径(导出后去那里取文件) */
  taskPath: string
}

/** AuditLogPage 审计查询:筛选全部写进地址,导出按当前筛选生成文件 */
export function AuditLogPage({
  title,
  description,
  targetOptions,
  renderActor,
  actorText,
  detailLookups,
  onDetailOpen,
  actorFilter,
  schoolFilter,
  tenantColumn,
  taskPath,
}: AuditLogPageProps) {
  const [query, setQuery] = useUrlState(AUDIT_QUERY)
  const [detail, setDetail] = useState<AuditLogEntry>()
  /** openDetail 打开一条记录的详情,并告诉页面打开的是哪一条 */
  const openDetail = (entry: AuditLogEntry) => {
    setDetail(entry)
    onDetailOpen?.(entry)
  }
  const [exporting, setExporting] = useState(false)
  const [exportError, setExportError] = useState<string>()

  // 地址是外部输入:认不出的对象类型、动作、操作人和日期都当作不筛选
  const target = targetOptions.some((option) => option.value === query.target)
    ? query.target
    : undefined
  const action = auditActionFilter(query.action)
  const actor = actorFilter ? auditActorFilter(query.actor) : undefined
  // 学校编号与账号编号同为雪花编号,复用同一校验;学校端没有这一项
  const school = schoolFilter ? auditActorFilter(query.school) : undefined
  const range = auditDayRange(query.from, query.to)
  const filters = useMemo(
    () => ({
      target_type: target,
      action,
      tenant_id: school,
      actor_id: actor,
      from: range.params.from,
      to: range.params.to,
    }),
    [action, actor, range.params.from, range.params.to, school, target]
  )
  const filtered = Object.values(filters).some((value) => value !== undefined)

  const logs = usePagedResource<AuditLogEntry>(
    (params) => api.admin.queryAudit({ ...filters, ...params }),
    [filters],
    { urlPageKey: 'page' }
  )

  // 动作下拉:接口按当前筛选返回各动作的全量条数,按条数从多到少排;当前选中但不在分组里的动作也保留
  const actionCounts = facetGroup(
    logs.total === undefined ? undefined : logs.data?.facets,
    'action'
  )
  const actionOptions = useMemo(() => {
    const entries = Object.entries(actionCounts ?? {}).sort(
      (a, b) => b[1] - a[1] || a[0].localeCompare(b[0])
    )
    if (action && !entries.some(([value]) => value === action)) entries.unshift([action, 0])
    return [
      { value: AUDIT_ALL, label: '全部动作' },
      ...entries.map(([value, count]) => ({
        value,
        label: `${auditActionLabel(value)}(${count})`,
      })),
    ]
  }, [action, actionCounts])

  /** exportLogs 按当前筛选创建导出任务;文件在任务与下载页取 */
  const exportLogs = async () => {
    setExporting(true)
    setExportError(undefined)
    try {
      await api.admin.exportAudit(filters)
      toast.success('导出任务已创建,完成后可在「任务与下载」取文件')
    } catch (error) {
      setExportError(userFacingErrorMessage(error, '导出任务没有创建成功,请稍后重试。'))
    } finally {
      setExporting(false)
    }
  }

  const columns: TableColumn<AuditLogEntry>[] = [
    {
      key: 'created_at',
      header: '时间',
      render: (entry) => (
        <span
          className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub"
          title={formatRelativeOccurrence(entry.created_at)}
        >
          {formatDateTimeSeconds(entry.created_at)}
        </span>
      ),
    },
    ...(tenantColumn
      ? [{ key: 'tenant_id', header: tenantColumn.header, render: tenantColumn.render }]
      : []),
    { key: 'actor_id', header: '操作人', render: renderActor },
    {
      key: 'action',
      header: '动作',
      primary: true,
      render: (entry) => (
        <span className="font-medium text-ink">{auditActionLabel(entry.action)}</span>
      ),
    },
    {
      key: 'target_type',
      header: '对象',
      render: (entry) => {
        const hint = auditObjectHint(entry.detail)
        return (
          <span className="min-w-0">
            <span className="block text-sm text-ink">
              {auditTargetTypeLabel(entry.target_type)}
            </span>
            {hint ? (
              <span className="block max-w-56 truncate text-xs text-ink-sub">{hint}</span>
            ) : null}
          </span>
        )
      },
    },
    {
      key: 'ip',
      header: '来源地址',
      render: (entry) => (
        <span className="whitespace-nowrap font-mono text-xs text-ink-sub">{entry.ip || '—'}</span>
      ),
    },
    {
      key: 'trace_id',
      header: '追踪编号',
      render: (entry) => <TraceIdCell traceId={entry.trace_id} />,
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        title={title}
        description={description}
        actions={
          <Button
            variant="outline"
            leftIcon={Download}
            loading={exporting}
            onClick={() => void exportLogs()}
          >
            导出
          </Button>
        }
      />

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <Select
          aria-label="按对象类型筛选"
          value={target ?? AUDIT_ALL}
          onValueChange={(value) => setQuery({ target: value })}
          options={[{ value: AUDIT_ALL, label: '全部对象' }, ...targetOptions]}
          className="w-full sm:w-40"
        />
        <Select
          aria-label="按动作筛选"
          value={action ?? AUDIT_ALL}
          onValueChange={(value) => setQuery({ action: value })}
          options={actionOptions}
          className="w-full sm:w-56"
        />
        {schoolFilter ? (
          <div className="w-full sm:w-56">
            {schoolFilter.render(school ?? '', (tenantId) => setQuery({ school: tenantId }))}
          </div>
        ) : null}
        {actorFilter ? (
          <div className="w-full sm:w-56">
            {actorFilter.render(actor ?? '', (accountId) => setQuery({ actor: accountId }))}
          </div>
        ) : null}
        <div className="flex w-full items-center gap-2 sm:w-auto">
          <Input
            type="date"
            aria-label="开始日期"
            value={range.from}
            max={range.to || undefined}
            onChange={(event) => setQuery({ from: event.target.value })}
            className="sm:w-40"
          />
          <span className="shrink-0 text-sm text-ink-sub">至</span>
          <Input
            type="date"
            aria-label="结束日期"
            value={range.to}
            min={range.from || undefined}
            onChange={(event) => setQuery({ to: event.target.value })}
            className="sm:w-40"
          />
        </div>
        {filtered ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              setQuery({ target: AUDIT_ALL, action: AUDIT_ALL, actor: '', from: '', to: '' })
            }
          >
            清空筛选
          </Button>
        ) : null}
      </div>

      {exportError ? (
        <Callout tone="danger" className="mb-4">
          {exportError}
        </Callout>
      ) : null}

      <ResourceState
        resource={logs}
        emptyIcon={FileText}
        emptyTitle={filtered ? '没有符合条件的记录' : '还没有审计记录'}
        emptyDescription={
          filtered
            ? '换个条件再试,或清空筛选查看全部记录。'
            : '账号、配置、成绩等敏感操作发生后会记录在这里。'
        }
        skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
      >
        {(page) => (
          <>
            <Table
              columns={columns}
              data={page.list}
              rowKey={(entry) => entry.id}
              onRowClick={openDetail}
              // <md 换行卡:动作一行;时间、操作人、对象一行
              mobileCard={(entry) => ({
                title: auditActionLabel(entry.action),
                meta: `${formatDateTimeSeconds(entry.created_at)} · ${actorText(entry)} · ${auditTargetTypeLabel(entry.target_type)}`,
              })}
            />
            <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
              <Pagination
                page={logs.page}
                pageSize={logs.pageSize}
                total={logs.total}
                onPageChange={logs.setPage}
              />
              <Link
                to={taskPath}
                className="text-sm text-ink-sub underline-offset-4 hover:text-ink hover:underline"
              >
                导出的文件在「任务与下载」
              </Link>
            </div>
          </>
        )}
      </ResourceState>

      {detail ? (
        <AuditDetailDrawer
          entry={detail}
          actorText={actorText}
          tenantText={tenantColumn?.text}
          detailLookups={detailLookups}
          onClose={() => setDetail(undefined)}
        />
      ) : null}
    </PageScaffold>
  )
}

interface AuditDetailDrawerProps {
  entry: AuditLogEntry
  actorText: (entry: AuditLogEntry) => string
  tenantText?: (entry: AuditLogEntry) => string
  detailLookups?: AuditDetailLookups
  onClose: () => void
}

/**
 * AuditDetailDrawer 一条审计记录的完整字段:时间、操作人与身份、动作、对象、来源地址、追踪编号,
 * 以及 detail 里登记过的附加项(如题目编码、版本、导入条数)。只读,没有修改和删除。
 */
function AuditDetailDrawer({
  entry,
  actorText,
  tenantText,
  detailLookups,
  onClose,
}: AuditDetailDrawerProps) {
  const items: DescriptionItem[] = [
    { term: '时间', description: formatDateTimeSeconds(entry.created_at), mono: true },
    ...(tenantText ? [{ term: '所属', description: tenantText(entry) }] : []),
    { term: '操作人', description: actorText(entry) },
    { term: '操作人身份', description: auditActorRoleLabel(entry.actor_role) },
    { term: '动作', description: auditActionLabel(entry.action) },
    { term: '对象类型', description: auditTargetTypeLabel(entry.target_type) },
    ...auditDetailRows(entry.detail, detailLookups).map((row) => ({
      term: row.term,
      description: row.value,
      mono: row.mono,
    })),
    { term: '来源地址', description: entry.ip || '没有记录', mono: Boolean(entry.ip) },
    { term: '追踪编号', description: <TraceIdCell traceId={entry.trace_id} full /> },
  ]
  return (
    <Drawer open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>{auditActionLabel(entry.action)}</DrawerTitle>
          <DrawerDescription>
            审计记录只能查看,不能修改或删除。报障时请提供追踪编号。
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody>
          <DescriptionList items={items} />
        </DrawerBody>
      </DrawerContent>
    </Drawer>
  )
}
