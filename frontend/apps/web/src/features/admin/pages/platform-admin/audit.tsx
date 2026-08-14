// 平台审计页(平台侧栏,/platform-admin/audit)。
//
// 审计只有 M9 一个查询入口。平台身份读到的是全平台记录(后端 QueryAudit 对平台身份
// 放开租户过滤),所以这里比学校侧多一列「学校」,少一个「操作者」筛选:
//   学校名从 GET /platform/tenants 解析 —— 那是平台自己的接口;
//   操作者姓名解析不了 —— 账号列表接口在租户组(GET /accounts 要求学校管理员身份),
//   平台账号调用会被拒。故这里按角色呈现操作者身份,姓名去对应学校的审计页看。
// 界面不显示内部账号编号:显示它既不可读也等于泄露标识。
//
// 导出走 transfer 任务而不是直接下文件(大量日志同步生成会拖住请求)。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Building, Download, FileText, Search } from 'lucide-react'
import type { AuditLogEntry, AuditQueryParams, Tenant } from '@chaimir/api-client'
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
  Stat,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  auditActionLabel,
  auditActorRoleLabel,
  auditTargetTypeLabel,
} from '../../../../utils/labels/admin'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 学校名解析一次取回的条数:后端分页上限 100。 */
const TENANT_PICKER_SIZE = 100

/** 可筛选的对象类型:取自 labels 里已登记的键,不让管理员手填内部标识。 */
const TARGET_TYPE_OPTIONS = [
  { value: '', label: '全部对象' },
  { value: 'tenant', label: '学校' },
  { value: 'account', label: '账号' },
  { value: 'config', label: '系统配置' },
  { value: 'alert', label: '告警' },
  { value: 'sim.package', label: '仿真场景包' },
  { value: 'session', label: '登录会话' },
] as const

/** 可筛选的动作:只列平台侧常查的已登记动作;未登记动作仍能在结果里正确显示中文名。 */
const ACTION_OPTIONS = [
  { value: '', label: '全部动作' },
  { value: 'auth.login', label: '登录' },
  { value: 'platform.application.approve', label: '通过入驻申请' },
  { value: 'platform.application.reject', label: '驳回入驻申请' },
  { value: 'platform.tenant.update', label: '修改学校信息' },
  { value: 'admin.config.update', label: '修改系统配置' },
  { value: 'admin.config.rollback', label: '回滚系统配置' },
  { value: 'admin.alert.handle', label: '处理告警' },
  { value: 'sim.review.approve', label: '通过仿真包审核' },
  { value: 'sim.review.reject', label: '退回仿真包' },
] as const

/**
 * PlatformAuditPage 查询全平台审计日志并导出。
 */
export default function PlatformAuditPage() {
  const navigate = useNavigate()
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

  // 审计只回 tenant_id,学校名在此解析,不把内部编号抛到界面上
  const tenants = useAsyncResource(
    () => api.identity.getTenants({ page: 1, size: TENANT_PICKER_SIZE }),
    [],
    () => false,
  )

  const tenantNameById = useMemo(
    () =>
      new Map(
        (tenants.data?.list ?? []).map((tenant: Tenant) => [
          tenant.id,
          tenant.display_name || tenant.name,
        ]),
      ),
    [tenants.data],
  )

  const applyFilters = useCallback(() => {
    setQuery({
      action: action || undefined,
      target_type: targetType || undefined,
      from: from ? new Date(from).toISOString() : undefined,
      to: to ? new Date(to).toISOString() : undefined,
    })
  }, [action, from, targetType, to])

  /** exportLogs 创建导出任务:大量日志同步生成会拖住请求,故走 transfer 任务。 */
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

  const columns: TableColumn<AuditLogEntry>[] = [
    {
      key: 'created_at',
      header: '时间',
      render: (entry) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(entry.created_at)}
        </span>
      ),
    },
    {
      key: 'tenant_id',
      header: '学校',
      render: (entry) =>
        entry.tenant_id ? (
          <span className="text-ink">
            {tenantNameById.get(entry.tenant_id) ?? '已移除的学校'}
          </span>
        ) : (
          <Badge tone="cinnabar">平台</Badge>
        ),
    },
    {
      key: 'actor_role',
      header: '操作者身份',
      render: (entry) => (
        <span className="text-sm text-ink-sub">{auditActorRoleLabel(entry.actor_role)}</span>
      ),
    },
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
      render: (entry) =>
        entry.trace_id ? (
          <span className="truncate font-mono text-xs text-ink-faint">{entry.trace_id}</span>
        ) : (
          <span className="text-ink-sub">—</span>
        ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }, { label: '平台审计' }]} />}
        title="平台审计"
        description="全平台的敏感操作记录,含各学校内部的操作。只能查询与导出,不能修改或删除。"
        icon={FileText}
        actions={
          <Button
            variant="outline"
            leftIcon={Download}
            loading={exporting}
            onClick={() => void exportLogs()}
          >
            导出当前结果
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="查询到的记录" value={logs.total} icon={FileText} />
          <Stat
            label="筛选条件"
            value={hasFilters ? '已设置' : '无'}
            icon={Search}
            hint={hasFilters ? '导出会按当前条件' : '导出会包含全部记录'}
          />
          <Stat
            label="可查询范围"
            value="全平台"
            icon={Building}
            hint="含各学校内部操作"
          />
        </div>
      </PageSection>

      <PageSection title="查询条件" description="按动作、对象类型或时间范围筛选。不填即查全部。">
        <form
          className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
          onSubmit={(event) => {
            event.preventDefault()
            applyFilters()
          }}
        >
          <FormField label="操作类型" htmlFor="platform-audit-action">
            <Select
              id="platform-audit-action"
              options={ACTION_OPTIONS.map((item) => ({ value: item.value, label: item.label }))}
              value={action}
              placeholder="全部动作"
              onValueChange={setAction}
            />
          </FormField>

          <FormField label="操作对象类型" htmlFor="platform-audit-target">
            <Select
              id="platform-audit-target"
              options={TARGET_TYPE_OPTIONS.map((item) => ({ value: item.value, label: item.label }))}
              value={targetType}
              placeholder="全部对象"
              onValueChange={setTargetType}
            />
          </FormField>

          <div className="flex items-end gap-2">
            <Button type="submit" variant="primary" leftIcon={Search}>
              查询
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                setAction('')
                setTargetType('')
                setFrom('')
                setTo('')
                setQuery({})
              }}
            >
              清空条件
            </Button>
          </div>

          <FormField label="开始时间" htmlFor="platform-audit-from">
            <Input
              id="platform-audit-from"
              type="datetime-local"
              value={from}
              onChange={(event) => setFrom(event.target.value)}
            />
          </FormField>

          <FormField label="结束时间" htmlFor="platform-audit-to">
            <Input
              id="platform-audit-to"
              type="datetime-local"
              value={to}
              onChange={(event) => setTo(event.target.value)}
            />
          </FormField>
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

          <Callout tone="info">
            这里按身份呈现操作者(平台管理员、学校管理员、教师、学生)。要看具体是谁,
            请在那所学校的管理端审计页按人筛选 —— 跨学校的账号姓名不在平台侧解析。
          </Callout>

          <div className="flex flex-wrap items-center gap-2 rounded-md border border-line bg-surface-sunken p-3">
            <span className="text-sm text-ink-sub">导出的文件在任务与下载页取件。</span>
            <Button variant="ghost" size="sm" onClick={() => navigate('/platform-admin/tasks')}>
              去任务与下载
            </Button>
          </div>
        </div>
      </PageSection>
    </PageScaffold>
  )
}
