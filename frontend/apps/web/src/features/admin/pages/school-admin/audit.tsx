// 审计日志页(校管侧栏,/school-admin/audit)。
//
// 审计只有 M9 一个查询入口(M1 只负责写入 audit_log)。这里是只读查询 + 导出:
// 按操作者、动作、对象类型与时间范围筛选,导出走 transfer 任务而不是直接下文件
// (大量日志同步生成会拖住请求,故后端返回任务快照,到任务与下载页取件)。
//
// 界面不直出内部标识:action 与 target_type 是各模块自取的开放字符串,
// 经 labels 翻成中文动作名;未登记的按模块拼出可读说明,不暴露点分标识。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Download, FileText, Search, UserSearch } from 'lucide-react'
import {
  type Account,
  type AuditLogEntry,
  type AuditQueryParams,
} from '@chaimir/api-client'
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

/** 账号选择器一次取回的条数:后端分页上限 100。 */
const ACCOUNT_PICKER_SIZE = 100

/** 可筛选的对象类型:取自 labels 里已登记的键,不让管理员手填内部标识。 */
const TARGET_TYPE_OPTIONS = [
  { value: '', label: '全部对象' },
  { value: 'account', label: '账号' },
  { value: 'course', label: '课程' },
  { value: 'assignment', label: '作业' },
  { value: 'experiment', label: '实验' },
  { value: 'contest', label: '竞赛' },
  { value: 'grade.review', label: '成绩审核' },
  { value: 'grade.appeal', label: '成绩申诉' },
  { value: 'content.item', label: '题目' },
  { value: 'session', label: '登录会话' },
] as const

/** 可筛选的动作:同样只列已登记的,未登记动作仍能在结果里正确显示中文名。 */
const ACTION_OPTIONS = [
  { value: '', label: '全部动作' },
  { value: 'auth.login', label: '登录' },
  { value: 'account.create', label: '创建账号' },
  { value: 'account.disable', label: '停用账号' },
  { value: 'account.reset_password', label: '重置账号密码' },
  { value: 'account.grant_admin', label: '授予管理员' },
  { value: 'account.import', label: '批量导入账号' },
  { value: 'tenant.config.update', label: '修改学校配置' },
  { value: 'grade.review.approve', label: '通过成绩审核' },
  { value: 'grade.review.unlock', label: '解锁成绩审核' },
  { value: 'grade.appeal.accept', label: '受理成绩申诉' },
] as const

/**
 * SchoolAdminAuditPage 查询本校审计日志并导出。
 */
export default function SchoolAdminAuditPage() {
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

  // 审计只回 actor_id,姓名在此解析,不把内部编号当操作者显示
  const accounts = useAsyncResource(
    () => api.identity.getAccounts({ page: 1, size: ACCOUNT_PICKER_SIZE }),
    [],
    () => false,
  )

  const accountById = useMemo(
    () => new Map((accounts.data?.list ?? []).map((account: Account) => [account.id, account])),
    [accounts.data],
  )

  const applyFilters = useCallback(() => {
    setQuery({
      actor_id: actorId || undefined,
      action: action || undefined,
      target_type: targetType || undefined,
      from: from ? new Date(from).toISOString() : undefined,
      to: to ? new Date(to).toISOString() : undefined,
    })
  }, [action, actorId, from, targetType, to])

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
      key: 'actor_id',
      header: '操作者',
      render: (entry) => {
        const account = accountById.get(entry.actor_id)
        return (
          <div className="min-w-0">
            <div className="truncate text-ink">{account ? account.name : '系统或已离校人员'}</div>
            <div className="truncate text-xs text-ink-sub">{auditActorRoleLabel(entry.actor_role)}</div>
          </div>
        )
      },
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
        kicker={<Breadcrumb items={[{ label: '系统配置' }, { label: '审计日志' }]} />}
        title="审计日志"
        description="本校范围内的敏感操作记录。只能查询与导出,不能修改或删除。"
        icon={FileText}
        actions={
          <Button variant="outline" leftIcon={Download} loading={exporting} onClick={() => void exportLogs()}>
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
          <Stat label="可查询范围" value="本校" icon={UserSearch} hint="平台级操作在平台端审计" />
        </div>
      </PageSection>

      <PageSection
        title="查询条件"
        description="按人、按动作、按对象或按时间范围筛选。不填即查全部。"
      >
        <form
          className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
          onSubmit={(event) => {
            event.preventDefault()
            applyFilters()
          }}
        >
          <FormField label="操作者" htmlFor="audit-actor">
            <Select
              id="audit-actor"
              options={[
                { value: '', label: '全部操作者' },
                ...(accounts.data?.list ?? []).map((account: Account) => ({
                  value: account.id,
                  label: account.no ? `${account.name} · ${account.no}` : account.name,
                })),
              ]}
              value={actorId}
              placeholder="全部操作者"
              onValueChange={setActorId}
            />
          </FormField>

          <FormField label="操作类型" htmlFor="audit-action">
            <Select
              id="audit-action"
              options={ACTION_OPTIONS.map((item) => ({ value: item.value, label: item.label }))}
              value={action}
              placeholder="全部动作"
              onValueChange={setAction}
            />
          </FormField>

          <FormField label="操作对象类型" htmlFor="audit-target">
            <Select
              id="audit-target"
              options={TARGET_TYPE_OPTIONS.map((item) => ({ value: item.value, label: item.label }))}
              value={targetType}
              placeholder="全部对象"
              onValueChange={setTargetType}
            />
          </FormField>

          <FormField label="开始时间" htmlFor="audit-from">
            <Input
              id="audit-from"
              type="datetime-local"
              value={from}
              onChange={(event) => setFrom(event.target.value)}
            />
          </FormField>

          <FormField label="结束时间" htmlFor="audit-to">
            <Input
              id="audit-to"
              type="datetime-local"
              value={to}
              onChange={(event) => setTo(event.target.value)}
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
                setActorId('')
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

          <div className="flex flex-wrap items-center gap-2 rounded-md border border-line bg-surface-sunken p-3">
            <span className="text-sm text-ink-sub">导出的文件在任务与下载页取件。</span>
            <Button variant="ghost" size="sm" onClick={() => navigate('/school-admin/tasks')}>
              去任务与下载
            </Button>
          </div>
        </div>
      </PageSection>
    </PageScaffold>
  )
}
