// AuditLogPage 统一 admin 领域的平台与学校审计查询、导出、分页和结果展示骨架。
// 调用页只提供各自权限范围内的筛选项、主体列和说明,本组件不推断当前角色。

import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
import { Download, FileText } from 'lucide-react'
import type { AuditLogEntry, AuditQueryParams } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  FilterBar,
  FilterField,
  Input,
  MetricStrip,
  PageHeader,
  PageScaffold,
  Pagination,
  Select,
  Table,
  toast,
  type BreadcrumbItem,
  type SelectOption,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { usePagedResource } from '../../../hooks'
import { formatDateTime } from '../../../utils/formatters'
import { auditActionLabel, auditTargetTypeLabel } from '../../../utils/labels/admin'
import { facetGroup, facetTopEntries } from '../../../utils/facets'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { TraceIdCell } from './TraceIdCell'

interface AuditLogPageProps {
  idPrefix: string
  breadcrumbItems: BreadcrumbItem[]
  title: string
  description: string
  actionOptions: readonly SelectOption[]
  targetTypeOptions: readonly SelectOption[]
  subjectColumn: TableColumn<AuditLogEntry>
  actorOptions?: SelectOption[]
  taskPath: string
  scopeNote?: ReactNode
}

/** AuditLogPage 承载两种管理范围共用的审计只读工作流。 */
export function AuditLogPage({
  idPrefix,
  breadcrumbItems,
  title,
  description,
  actionOptions,
  targetTypeOptions,
  subjectColumn,
  actorOptions,
  taskPath,
  scopeNote,
}: AuditLogPageProps) {
  const navigate = useNavigate()
  const [actorId, setActorId] = useState('')
  const [action, setAction] = useState('')
  const [targetType, setTargetType] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [query, setQuery] = useState<AuditQueryParams>({})
  const [exporting, setExporting] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const logs = usePagedResource<AuditLogEntry>(
    (params) => api.admin.queryAudit({ ...query, ...params }),
    [query],
  )
  // 动作分布来自后端聚合契约:facets 与筛选同口径,且是全量分组计数(§6.5.4)
  const actionFacets = useMemo(() => facetGroup(logs.data?.facets, 'action'), [logs.data])
  const actionKinds = Object.keys(actionFacets).length
  const topAction = useMemo(
    () => facetTopEntries(logs.data?.facets, 'action', 1)[0],
    [logs.data],
  )


  /** applyFilters 把界面筛选值收敛为后端审计查询契约。 */
  const applyFilters = useCallback(() => {
    setQuery({
      actor_id: actorOptions && actorId ? actorId : undefined,
      action: action || undefined,
      target_type: targetType || undefined,
      from: from ? new Date(from).toISOString() : undefined,
      to: to ? new Date(to).toISOString() : undefined,
    })
  }, [action, actorId, actorOptions, from, targetType, to])

  /** clearFilters 同时清空编辑态和已提交查询,避免界面与结果口径不一致。 */
  const clearFilters = useCallback(() => {
    setActorId('')
    setAction('')
    setTargetType('')
    setFrom('')
    setTo('')
    setQuery({})
  }, [])

  /** exportLogs 按当前已提交查询创建异步导出任务。 */
  const exportLogs = useCallback(async () => {
    setExporting(true)
    setActionError(undefined)
    try {
      await api.admin.exportAudit(query)
      toast.success('导出任务已创建,完成后到任务与下载页取件')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '导出任务创建失败,请稍后重试。'))
    } finally {
      setExporting(false)
    }
  }, [query])

  const hasFilters = Object.values(query).some((value) => value !== undefined)
  const filterScope = actorOptions ? '按人、按动作、按对象或按时间范围筛选' : '按动作、对象类型或时间范围筛选'

  const columns = useMemo<TableColumn<AuditLogEntry>[]>(
    () => [
      {
        key: 'created_at',
        header: '时间',
        render: (entry) => (
          <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
            {formatDateTime(entry.created_at)}
          </span>
        ),
      },
      subjectColumn,
      {
        key: 'action',
        header: '操作',
        render: (entry) => <Badge tone="neutral">{auditActionLabel(entry.action)}</Badge>,
      },
      {
        key: 'target_type',
        header: '操作对象',
        render: (entry) => (
          <span className="text-sm text-ink-sub">{auditTargetTypeLabel(entry.target_type)}</span>
        ),
      },
      {
        key: 'ip',
        header: '来源地址',
        render: (entry) => (
          <span className="whitespace-nowrap font-mono text-xs text-ink-sub">{entry.ip ?? '—'}</span>
        ),
      },
      {
        key: 'trace_id',
        header: '操作编号',
        render: (entry) => <TraceIdCell traceId={entry.trace_id} />,
      },
    ],
    [subjectColumn],
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={breadcrumbItems} />}
        title={title}
        description={description}
        icon={FileText}
        actions={
          <Button variant="outline" leftIcon={Download} loading={exporting} onClick={() => void exportLogs()}>
            导出当前结果
          </Button>
        }
      />

      {/*
        归族:资源列表族(§6.5.3 第 ①)。审计是「在一批记录里找一条」,
        故指标退为一行内联摘要 —— 记录总数与筛选口径都是这一页要先交代的两句话。
        动作分布取后端 facets.action(与当前筛选同口径的全量分组计数),
        不用当前页切片去数 —— 那在总量更大时是错数(§6.5.4)。
      */}
      <MetricStrip
        label="审计记录摘要"
        className="mb-5"
        items={[
          { label: '记录条数', value: logs.total, hint: hasFilters ? '当前条件下' : '全部记录' },
          { label: '涵盖动作类型', value: actionKinds, hint: '按当前筛选统计' },
          {
            label: '最常见动作',
            value: topAction ? auditActionLabel(topAction.value) : '—',
            hint: topAction ? `共 ${topAction.count} 条` : '暂无记录',
          },
          { label: '每页条数', value: logs.pageSize, hint: `第 ${logs.page} 页` },
        ]}
      />

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {/* 筛选井、数据表、分页同处一块抬起片(§6.5.2):审计条件需要确认才生效,故走 FilterBar 的 onSubmit */}
      <DataPanel
        label="操作记录"
        filter={
          <FilterBar
            label="审计条件"
            onSubmit={applyFilters}
            onReset={hasFilters || actorId || action || targetType || from || to ? clearFilters : undefined}
          >
            {actorOptions ? (
              <FilterField label="操作者" htmlFor={`${idPrefix}-actor`}>
                <Select
                  id={`${idPrefix}-actor`}
                  options={actorOptions}
                  value={actorId}
                  placeholder="全部操作者"
                  onValueChange={setActorId}
                />
              </FilterField>
            ) : null}

            <FilterField label="操作类型" htmlFor={`${idPrefix}-action`}>
              <Select
                id={`${idPrefix}-action`}
                options={[...actionOptions]}
                value={action}
                placeholder="全部动作"
                onValueChange={setAction}
              />
            </FilterField>

            <FilterField label="操作对象类型" htmlFor={`${idPrefix}-target`}>
              <Select
                id={`${idPrefix}-target`}
                options={[...targetTypeOptions]}
                value={targetType}
                placeholder="全部对象"
                onValueChange={setTargetType}
              />
            </FilterField>

            <FilterField label="开始时间" htmlFor={`${idPrefix}-from`}>
              <Input
                id={`${idPrefix}-from`}
                type="datetime-local"
                value={from}
                onChange={(event) => setFrom(event.target.value)}
              />
            </FilterField>

            <FilterField label="结束时间" htmlFor={`${idPrefix}-to`}>
              <Input
                id={`${idPrefix}-to`}
                type="datetime-local"
                value={to}
                onChange={(event) => setTo(event.target.value)}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={logs.page}
            pageSize={logs.pageSize}
            total={logs.total}
            onPageChange={logs.setPage}
          />
        }
      >
        <ResourceState
          resource={logs}
          emptyIcon={FileText}
          emptyTitle={hasFilters ? '没有匹配的记录' : '暂无审计记录'}
          emptyDescription={
            hasFilters ? '换个条件再试,或清空条件查看全部。' : '敏感操作发生后会记录在这里。'
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(page) => (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(item) => item.id}
              elevated={false}
              // <md 换行卡(§6.4.1 规则 3):时间一行、动作与对象一行,操作编号在右
              mobileCard={(item) => ({
                title: formatDateTime(item.created_at),
                meta: `${auditActionLabel(item.action)} · ${auditTargetTypeLabel(item.target_type)}${item.ip ? ` · ${item.ip}` : ''}`,
                badge: <TraceIdCell traceId={item.trace_id} />,
              })}
            />
          )}
        </ResourceState>
      </DataPanel>

      {/* 取件出口与查询范围说明排在片外:它们讲的是「这页之外的事」,不属于数据区 */}
      <div className="mt-4 flex flex-col gap-3">
        {scopeNote ? <Callout tone="info">{scopeNote}</Callout> : null}
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm text-ink-sub">
            {filterScope}。导出的文件在任务与下载页取件。
          </span>
          <Button variant="ghost" size="sm" onClick={() => navigate(taskPath)}>
            去任务与下载
          </Button>
        </div>
      </div>
    </PageScaffold>
  )
}
