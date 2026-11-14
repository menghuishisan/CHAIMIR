// 学校管理(平台侧栏首页,/platform-admin/schools)。设计 §7.6:
// 平台管理员登录后的第一页:看已开通的学校、调整状态和到期时间。几十到几百所。
// 页头右侧次要入口「入驻申请」(有待审核时带数字);工具条为状态分段(带全量数字)与搜索,都写进地址;
// 表格列为学校名与短码、部署形态、登录方式、状态、服务到期(30 天内到期警示并写剩余天数),整行链向学校详情;
// 行内「调整状态」打开与详情页同一个弹窗,弹窗写明「会立即影响该校师生登录」。指标条去掉。

import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { Building, Inbox } from 'lucide-react'
import { ApplicationStatus, TenantStatus, type Tenant } from '@chaimir/api-client'
import {
  Button,
  buttonVariants,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import {
  useAsyncResource,
  usePagedResource,
  useUrlState,
} from '../../../../hooks'
import { segmentCounts } from '../../../../utils/facets'
import {
  authModeLabel,
  deployModeLabel,
  tenantStatusLabel,
} from '../../../../utils/labels/identity'
import { TenantExpiryText } from '../../components/TenantExpiryText'
import { TenantStatusModal } from '../../components/TenantStatusModal'
import { tenantStatusTone } from '../../statusPresentation'

/** 地址参数:status 状态分段(all 为全部),q 学校名或短码;页码在 page */
const SCHOOLS_QUERY = {
  status: { defaultValue: 'all', allowed: ['all', 'active', 'disabled', 'expired'] },
  q: { defaultValue: '' },
} as const

const STATUS_SEGMENTS = [
  { value: 'all', label: '全部' },
  { value: 'active', label: '正常', status: TenantStatus.ACTIVE },
  { value: 'disabled', label: '已停用', status: TenantStatus.DISABLED },
  { value: 'expired', label: '已到期', status: TenantStatus.EXPIRED },
] as const

/** PlatformSchoolsPage 学校列表:按状态与名称找学校,整行进入详情,行内调整状态 */
export default function PlatformSchoolsPage() {
  const navigate = useNavigate()
  const [query, setQuery] = useUrlState(SCHOOLS_QUERY)
  const [target, setTarget] = useState<Tenant>()
  const segment = STATUS_SEGMENTS.find((item) => item.value === query.status)
  const statusFilter = segment && 'status' in segment ? segment.status : undefined
  const keyword = query.q.trim()

  const tenants = usePagedResource<Tenant>(
    (params) =>
      api.identity.getTenants({ status: statusFilter, keyword: keyword || undefined, ...params }),
    [statusFilter, keyword],
    { urlPageKey: 'page' }
  )

  // 分段计数取列表接口的 facets:套用搜索,不套用状态筛选
  const facets = tenants.status === 'success' || tenants.status === 'empty' ? tenants.data?.facets : undefined
  const counts = segmentCounts(facets, 'status', STATUS_SEGMENTS, (item) => ('status' in item ? item.status : undefined))
  // 页头「共 N 所学校」是不随状态筛选的全部学校数(套用搜索),即状态一组之和
  const allCount = counts.all

  // 页头「入驻申请」上的待审核数(申请接口不分页,直接取待审核的清单数)
  const pendingApplications = useAsyncResource(
    () => api.identity.getApplications({ status: ApplicationStatus.PENDING }),
    [],
    () => false
  )
  const pendingCount = pendingApplications.data?.length ?? 0
  const filtered = query.status !== 'all' || keyword !== ''

  const columns: TableColumn<Tenant>[] = [
    {
      key: 'name',
      header: '学校',
      primary: true,
      render: (tenant) => (
        <span className="min-w-0">
          <span className="block truncate font-medium text-ink">
            {tenant.display_name || tenant.name}
          </span>
          <span className="block font-mono text-xs text-ink-sub">{tenant.code}</span>
        </span>
      ),
    },
    {
      key: 'deploy_mode',
      header: '部署形态',
      render: (tenant) => (
        <span className="text-sm text-ink">{deployModeLabel(tenant.deploy_mode)}</span>
      ),
    },
    {
      key: 'auth_mode',
      header: '登录方式',
      render: (tenant) => (
        <span className="text-sm text-ink">{authModeLabel(tenant.auth_mode)}</span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (tenant) => (
        <StatusIndicator
          tone={tenantStatusTone(tenant.status)}
          label={tenantStatusLabel(tenant.status)}
        />
      ),
    },
    {
      key: 'expire_at',
      header: '服务到期',
      render: (tenant) => <TenantExpiryText expireAt={tenant.expire_at} />,
    },
    {
      key: 'actions',
      header: <span className="sr-only">操作</span>,
      align: 'right',
      render: (tenant) => (
        <Button variant="ghost" size="sm" onClick={() => setTarget(tenant)}>
          调整状态
        </Button>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        title="学校管理"
        meta={allCount !== undefined ? [{ value: `共 ${allCount} 所学校` }] : undefined}
        actions={
          <Link
            to="/platform-admin/applications"
            className={buttonVariants({ variant: 'outline' })}
          >
            入驻申请
            {pendingCount > 0 ? (
              <span className="ml-1 rounded-full bg-warning-bg px-2 text-xs font-semibold tabular-nums text-warning">
                {pendingCount} 待审核
              </span>
            ) : null}
          </Link>
        }
      />

      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按学校状态筛选"
          options={STATUS_SEGMENTS.map((item) => ({
            value: item.value,
            label: item.label,
            count: counts[item.value],
          }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
          className="max-w-full self-start overflow-x-auto"
        />
        <SearchBox
          label="搜索学校"
          placeholder="搜索学校名或短码"
          value={query.q}
          onSearch={(value) => setQuery({ q: value })}
        />
      </div>

      <ResourceState
        resource={tenants}
        emptyIcon={filtered ? Building : Inbox}
        emptyTitle={filtered ? '没有符合条件的学校' : '还没有开通学校'}
        emptyDescription={
          filtered ? '换个状态或关键词再试。' : '入驻申请审核通过后,学校会出现在这里。'
        }
        emptyAction={
          filtered ? (
            <Button variant="outline" onClick={() => setQuery({ status: 'all', q: '' })}>
              清空筛选
            </Button>
          ) : (
            <Link
              to="/platform-admin/applications"
              className={buttonVariants({ variant: 'primary' })}
            >
              去看入驻申请
            </Link>
          )
        }
        skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
      >
        {(page) => (
          <>
            <Table
              columns={columns}
              data={page.list}
              rowKey={(tenant) => tenant.id}
              getRowHref={(tenant) => `/platform-admin/schools/${tenant.id}`}
              onNavigate={navigate}
              // <md 换行卡:校名一行;短码、部署形态与到期一行;状态在右,行尾「调整」
              mobileCard={(tenant) => ({
                title: tenant.display_name || tenant.name,
                meta: (
                  <>
                    <span className="font-mono">{tenant.code}</span> ·{' '}
                    {deployModeLabel(tenant.deploy_mode)} ·{' '}
                    <TenantExpiryText expireAt={tenant.expire_at} />
                  </>
                ),
                badge: (
                  <StatusIndicator
                    tone={tenantStatusTone(tenant.status)}
                    label={tenantStatusLabel(tenant.status)}
                  />
                ),
                action: (
                  <Button variant="ghost" size="sm" onClick={() => setTarget(tenant)}>
                    调整
                  </Button>
                ),
              })}
            />
            <div className="mt-4">
              <Pagination
                page={tenants.page}
                pageSize={tenants.pageSize}
                total={tenants.total}
                onPageChange={tenants.setPage}
              />
            </div>
          </>
        )}
      </ResourceState>

      {target ? (
        <TenantStatusModal
          tenant={target}
          onClose={() => setTarget(undefined)}
          onSaved={() => {
            setTarget(undefined)
            tenants.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}
