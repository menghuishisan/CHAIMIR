// 入驻申请页(平台侧栏,/platform-admin/applications)。
//
// 学校在公共入驻页提交申请(匿名接口),平台在这里审核。通过即在一个事务里创建租户、
// 开通首个学校管理员并签发一次性激活码 —— 后果重且不可撤销,故核对资料与填写开通信息
// 都在详情深页完成,列表页只做「谁在等、谁已处理」的分诊。
//
// 后端 GET /platform/applications 回全量列表(不分页),故筛选与统计在本页内完成。

import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Inbox, Search } from 'lucide-react'
import { ApplicationStatus, type TenantApplication } from '@chaimir/api-client'
import {
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
  SegmentedControl,
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
  schoolTypeLabel,
} from '../../../../utils/labels/identity'
import { applicationStatusTone } from '../../statusPresentation'

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

  // 指标带必须是全量口径,故单独取一份不带状态筛选的清单:
  // 复用上面这份会让「待审核」在筛「已开通」时变成 0(规范 §6.5)。
  const allApplications = useAsyncResource(() => api.identity.getApplications(), [])
  const stats = useMemo(() => {
    const all = allApplications.data ?? []
    return {
      pending: all.filter((item) => item.status === ApplicationStatus.PENDING).length,
      approved: all.filter((item) => item.status === ApplicationStatus.APPROVED).length,
      rejected: all.filter((item) => item.status === ApplicationStatus.REJECTED).length,
    }
  }, [allApplications.data])

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
        kicker={<Breadcrumb items={[{ label: '租户' }]} />}
        title="入驻申请"
        description="学校提交的开通申请。审核通过会创建学校并开通首个管理员账号,请先核对联系人身份。"
        icon={Inbox}
      />

      {/* 指标降为内联摘要(§6.5.3 第 ① 族):本页主体是申请记录 */}
      <MetricStrip
        label="申请积压摘要"
        className="mb-5"
        items={[
          {
            label: '待审核',
            value: stats.pending,
            hint: stats.pending > 0 ? '需要你处理' : '暂时没有积压',
          },
          { label: '已开通', value: stats.approved, hint: '不受下方筛选影响' },
          { label: '已驳回', value: stats.rejected, hint: '学校可重新提交' },
        ]}
      />

      <Callout tone="info" className="mb-4">
        申请里的联系方式由学校自行填写,开通前请通过其他渠道核实身份 —— 通过后会立刻创建可登录的管理员账号。
      </Callout>

      {/* 筛选井与数据表同处一块抬起片(§6.5.2) */}
      <DataPanel
        label="申请记录"
        filter={
          <FilterBar label="申请记录筛选">
            <FilterField label="审核状态" group>
              <SegmentedControl
                aria-label="按审核状态筛选"
                size="sm"
                options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={statusFilter}
                onValueChange={setStatusFilter}
              />
            </FilterField>
            <FilterField label="学校名称或联系人" htmlFor="applications-keyword">
              <Input
                id="applications-keyword"
                leftIcon={Search}
                value={keyword}
                placeholder="输入关键词"
                onChange={(event) => setKeyword(event.target.value)}
              />
            </FilterField>
          </FilterBar>
        }
      >
          <ResourceState
            resource={applications}
            emptyIcon={Inbox}
            emptyTitle={statusFilter ? '这个状态下没有申请' : '还没有入驻申请'}
            emptyDescription={
              statusFilter ? '换个状态看看。' : '学校在公共入驻页提交申请后会出现在这里。'
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {() => (
              <Table
                columns={columns}
                data={visible}
                rowKey={(item) => item.application_id}
                elevated={false}
                empty={<span className="text-sm text-ink-sub">没有匹配的申请,换个关键词看看。</span>}
                // <md 换行卡(§6.4.1 规则 3):校名一行、联系人与提交时间一行
                mobileCard={(item) => ({
                  title: item.school_name,
                  meta: `${item.contact_name} · ${formatDateTime(item.submitted_at)}`,
                  badge: (
                    <StatusIndicator
                      tone={applicationStatusTone(item.status)}
                      label={applicationStatusLabel(item.status)}
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
