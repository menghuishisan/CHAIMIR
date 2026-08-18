// AuditLogPage 统一 admin 领域的平台与学校审计查询、导出、分页和结果展示骨架。
// 调用页只提供各自权限范围内的筛选项、主体列和说明,本组件不推断当前角色。

import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
import { Download, FileText, Search } from 'lucide-react'
import type { AuditLogEntry, AuditQueryParams } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  FormField,
  Input,
  PageHeader,
  PageScaffold,
  PageSection,
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

      <PageSection
        title="查询条件"
        description={`${filterScope}.${hasFilters ? '导出会按当前条件。' : '不填即查全部,导出会包含全部记录。'}`}
      >
        <form
          className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
          onSubmit={(event) => {
            event.preventDefault()
            applyFilters()
          }}
        >
          {actorOptions ? (
            <FormField label="操作者" htmlFor={`${idPrefix}-actor`}>
              <Select
                id={`${idPrefix}-actor`}
                options={actorOptions}
                value={actorId}
                placeholder="全部操作者"
                onValueChange={setActorId}
              />
            </FormField>
          ) : null}

          <FormField label="操作类型" htmlFor={`${idPrefix}-action`}>
            <Select
              id={`${idPrefix}-action`}
              options={[...actionOptions]}
              value={action}
              placeholder="全部动作"
              onValueChange={setAction}
            />
          </FormField>

          <FormField label="操作对象类型" htmlFor={`${idPrefix}-target`}>
            <Select
              id={`${idPrefix}-target`}
              options={[...targetTypeOptions]}
              value={targetType}
              placeholder="全部对象"
              onValueChange={setTargetType}
            />
          </FormField>

          <FormField label="开始时间" htmlFor={`${idPrefix}-from`}>
            <Input
              id={`${idPrefix}-from`}
              type="datetime-local"
              value={from}
              onChange={(event) => setFrom(event.target.value)}
            />
          </FormField>

          <FormField label="结束时间" htmlFor={`${idPrefix}-to`}>
            <Input
              id={`${idPrefix}-to`}
              type="datetime-local"
              value={to}
              onChange={(event) => setTo(event.target.value)}
            />
          </FormField>

          <div className="flex items-end gap-2">
            <Button type="submit" variant="primary" leftIcon={Search}>
              查询
            </Button>
            <Button type="button" variant="ghost" onClick={clearFilters}>
              清空条件
            </Button>
          </div>
        </form>
      </PageSection>

      <PageSection title="操作记录" description={`共 ${logs.total} 条,按时间从新到旧排列。`}>
        <div className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={logs}
            emptyIcon={FileText}
            emptyTitle={hasFilters ? '没有匹配的记录' : '暂无审计记录'}
            emptyDescription={
              hasFilters ? '换个条件再试,或清空条件查看全部。' : '敏感操作发生后会记录在这里。'
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={logs.page}
                  pageSize={logs.pageSize}
                  total={logs.total}
                  onPageChange={logs.setPage}
                />
              </>
            )}
          </ResourceState>

          {scopeNote ? <Callout tone="info">{scopeNote}</Callout> : null}

          <div className="flex flex-wrap items-center gap-2 well p-3">
            <span className="text-sm text-ink-sub">导出的文件在任务与下载页取件。</span>
            <Button variant="ghost" size="sm" onClick={() => navigate(taskPath)}>
              去任务与下载
            </Button>
          </div>
        </div>
      </PageSection>
    </PageScaffold>
  )
}
