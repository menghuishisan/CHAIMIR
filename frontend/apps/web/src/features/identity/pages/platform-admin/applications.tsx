// 入驻申请页(平台侧栏,/platform-admin/applications)。
//
// 学校在公共入驻页提交申请(匿名接口),平台在这里审核。通过即在一个事务里创建租户、
// 开通首个学校管理员并签发一次性激活码 —— 后果重且不可撤销,故核对资料与填写开通信息
// 都在详情深页完成,列表页只做「谁在等、谁已处理」的分诊。
//
// 后端 GET /platform/applications 回全量列表(不分页),故筛选与统计在本页内完成。

import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { CircleCheck, CircleX, Inbox, Search } from 'lucide-react'
import { ApplicationStatus, type TenantApplication } from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  Callout,
  Input,
  PageHeader,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Stat,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  applicationStatusLabel,
  applicationStatusTone,
  schoolTypeLabel,
} from '../../../../utils/labels/identity'

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(ApplicationStatus.PENDING), label: '待审核' },
  { value: String(ApplicationStatus.APPROVED), label: '已开通' },
  { value: String(ApplicationStatus.REJECTED), label: '已驳回' },
] as const

/**
 * PlatformApplicationsPage 列出学校入驻申请并进入审核详情。
 */
export default function PlatformApplicationsPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<string>(String(ApplicationStatus.PENDING))
  const [keyword, setKeyword] = useState('')

  const applications = useAsyncResource(
    () =>
      api.identity.getApplications(
        statusFilter ? { status: Number(statusFilter) as ApplicationStatus } : undefined,
      ),
    [statusFilter],
  )

  const list = useMemo(() => applications.data ?? [], [applications.data])

  const visible = useMemo(() => {
    const text = keyword.trim().toLowerCase()
    if (text === '') return list
    return list.filter(
      (item) =>
        item.school_name.toLowerCase().includes(text) ||
        item.contact_name.toLowerCase().includes(text),
    )
  }, [keyword, list])

  const stats = useMemo(
    () => ({
      pending: list.filter((item) => item.status === ApplicationStatus.PENDING).length,
      approved: list.filter((item) => item.status === ApplicationStatus.APPROVED).length,
      rejected: list.filter((item) => item.status === ApplicationStatus.REJECTED).length,
    }),
    [list],
  )

  const columns: TableColumn<TenantApplication>[] = [
    {
      key: 'school_name',
      header: '学校',
      render: (item) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{item.school_name}</div>
          <div className="truncate text-xs text-ink-sub">{schoolTypeLabel(item.school_type)}</div>
        </div>
      ),
    },
    {
      key: 'contact_name',
      header: '联系人',
      render: (item) => (
        <div className="min-w-0">
          <div className="truncate text-ink">{item.contact_name}</div>
          <div className="truncate font-mono text-xs text-ink-sub">{item.contact_phone}</div>
        </div>
      ),
    },
    {
      key: 'submitted_at',
      header: '提交时间',
      render: (item) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(item.submitted_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (item) => (
        <div className="flex flex-col gap-1">
          <StatusIndicator
            tone={applicationStatusTone(item.status)}
            label={applicationStatusLabel(item.status)}
          />
          {item.reviewed_at ? (
            <span className="font-mono text-xs text-ink-faint">{formatDateTime(item.reviewed_at)}</span>
          ) : null}
        </div>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (item) => (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => navigate(`/platform-admin/applications/${item.application_id}`)}
        >
          {item.status === ApplicationStatus.PENDING ? '去审核' : '看详情'}
        </Button>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '租户' }, { label: '入驻申请' }]} />}
        title="入驻申请"
        description="学校提交的开通申请。审核通过会创建学校并开通首个管理员账号,请先核对联系人身份。"
        icon={Inbox}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat
            label="待审核"
            value={stats.pending}
            icon={Inbox}
            hint={stats.pending > 0 ? '需要你处理' : '暂时没有积压'}
          />
          <Stat label="已开通" value={stats.approved} icon={CircleCheck} />
          <Stat label="已驳回" value={stats.rejected} icon={CircleX} />
        </div>
      </PageSection>

      <PageSection
        title="申请记录"
        description="按学校名称或联系人搜索。审核在详情页完成,那里能看到完整联系方式。"
        actions={
          <div className="flex flex-wrap items-end gap-2">
            <SegmentedControl
              aria-label="按审核状态筛选"
              size="sm"
              options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
              value={statusFilter}
              onValueChange={setStatusFilter}
            />
            <Input
              aria-label="按学校名称或联系人搜索"
              leftIcon={Search}
              value={keyword}
              placeholder="搜索申请"
              onChange={(event) => setKeyword(event.target.value)}
            />
          </div>
        }
      >
        <div className="flex flex-col gap-4">
          <ResourceState
            resource={applications}
            emptyIcon={Inbox}
            emptyTitle={statusFilter ? '这个状态下没有申请' : '还没有入驻申请'}
            emptyDescription={
              statusFilter ? '换个状态看看。' : '学校在公共入驻页提交申请后会出现在这里。'
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {() => (
              <Table
                columns={columns}
                data={visible}
                rowKey={(item) => item.application_id}
                empty={<span className="text-sm text-ink-sub">没有匹配的申请,换个关键词看看。</span>}
              />
            )}
          </ResourceState>

          <Callout tone="info">
            申请里的联系方式由学校自行填写,开通前请通过其他渠道核实身份 —— 通过后会立刻创建可登录的管理员账号。
          </Callout>
        </div>
      </PageSection>
    </PageScaffold>
  )
}
