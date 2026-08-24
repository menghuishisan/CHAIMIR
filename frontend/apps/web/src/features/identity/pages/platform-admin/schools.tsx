// 学校管理页(平台侧栏首屏,/platform-admin/schools)。
//
// 平台端的第一个功能页(FE-5:登录直达,不做仪表盘落地页)。一所学校就是一个租户,
// 这里只做「有哪些学校、各自什么状态」的总览与状态调整;学校自己的配置、配额与
// 使用情况在详情深页 —— 列表页塞详情会让常用的状态巡检变慢。
//
// 状态三档由后端 UpdateTenantStatusByPlatform 限定(正常/停用/到期),
// 停用与到期的后果不同,故各自说明再确认。到期时间只在这条接口上可改。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import { Building, CircleSlash, Search, ShieldCheck } from 'lucide-react'
import { TenantStatus, type Tenant } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  DescriptionList,
  FilterBar,
  FilterField,
  FormField,
  Input,
  MetricStrip,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Select,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatDate, formatDateTime } from '../../../../utils/formatters'
import {
  deployModeLabel,
  tenantStatusLabel,
} from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { TENANT_STATUSES } from '../../options'
import { tenantStatusTone } from '../../statusPresentation'

/** 状态调整的后果说明:三档各不相同,确认框据此给出不同文案。 */
const STATUS_COPY: Record<TenantStatus, { title: string; description: string; danger?: boolean }> = {
  [TenantStatus.ACTIVE]: {
    title: '恢复正常使用',
    description: '恢复后这所学校的师生可以照常登录与使用。已停用期间产生的数据不受影响。',
  },
  [TenantStatus.DISABLED]: {
    title: '停用这所学校',
    description:
      '停用后全校师生立即无法登录,进行中的实验与竞赛环境会被回收。数据保留,恢复后可继续使用。',
    danger: true,
  },
  [TenantStatus.EXPIRED]: {
    title: '标记为已到期',
    description:
      '到期与停用的效果相同,区别在于原因记录为服务期结束。续期时把状态改回正常并延长到期时间。',
    danger: true,
  },
}

/**
 * PlatformSchoolsPage 列出平台上的学校并承载状态调整。
 */
export default function PlatformSchoolsPage() {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [searchTerm, setSearchTerm] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [target, setTarget] = useState<Tenant>()

  // 状态与关键词都交给服务端:对当前页做二次筛选会让「共 N 所」与看到的行数、
  // 以及翻页范围三者互相矛盾(筛掉的行仍占着页码)。
  const tenants = usePagedResource<Tenant>(
    (params) =>
      api.identity.getTenants({
        status: statusFilter ? (Number(statusFilter) as TenantStatus) : undefined,
        keyword: searchTerm || undefined,
        ...params,
      }),
    [statusFilter, searchTerm],
  )

  // 指标带取全量口径,不随下方筛选变化:它回答「平台整体有多少学校、多少在正常使用」。
  const totalCount = useResourceTotal((params) => api.identity.getTenants(params), [])
  const activeCount = useResourceTotal(
    (params) => api.identity.getTenants({ status: TenantStatus.ACTIVE, ...params }),
    [],
  )
  const disabledCount = useResourceTotal(
    (params) => api.identity.getTenants({ status: TenantStatus.DISABLED, ...params }),
    [],
  )
  const expiredCount = useResourceTotal(
    (params) => api.identity.getTenants({ status: TenantStatus.EXPIRED, ...params }),
    [],
  )

  const columns: TableColumn<Tenant>[] = [
    {
      key: 'name',
      header: '学校',
      render: (tenant) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{tenant.display_name || tenant.name}</div>
          <div className="truncate font-mono text-xs text-ink-sub">{tenant.code}</div>
        </div>
      ),
    },
    {
      key: 'deploy_mode',
      header: '部署形态',
      render: (tenant) => <Badge tone="neutral">{deployModeLabel(tenant.deploy_mode)}</Badge>,
    },
    {
      key: 'status',
      header: '状态',
      render: (tenant) => (
        <StatusIndicator tone={tenantStatusTone(tenant.status)} label={tenantStatusLabel(tenant.status)} />
      ),
    },
    {
      key: 'expire_at',
      header: '服务到期',
      render: (tenant) =>
        tenant.expire_at ? (
          <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
            {formatDate(tenant.expire_at)}
          </span>
        ) : (
          <span className="text-sm text-ink-sub">未设置</span>
        ),
    },
    {
      key: 'created_at',
      header: '开通时间',
      render: (tenant) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDate(tenant.created_at)}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (tenant) => (
        <div className="flex items-center justify-end gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigate(`/platform-admin/schools/${tenant.id}`)}
          >
            详情
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setTarget(tenant)}>
            调整状态
          </Button>
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '租户' }]} />}
        title="学校管理"
        description="平台上已开通的学校。停用与到期会立即影响该校师生登录,调整前请确认。"
        icon={Building}
      />

      {/* 指标降为内联摘要(§6.5.3 第 ① 族):本页主体是学校列表 */}
      <MetricStrip
        label="学校总量摘要"
        className="mb-5"
        items={[
          { label: '学校总数', value: totalCount ?? '—', hint: '不受下方筛选影响' },
          { label: '正常使用', value: activeCount ?? '—', hint: '师生可正常登录' },
          { label: '已停用', value: disabledCount ?? '—', hint: '该校师生无法登录' },
          { label: '已到期', value: expiredCount ?? '—', hint: '需要关注续期' },
        ]}
      />

      {/* 筛选井、数据表、分页同处一块抬起片(§6.5.2) */}
      <DataPanel
        label="学校列表"
        filter={
          <FilterBar label="学校筛选" onSubmit={() => setSearchTerm(keyword.trim())} submitLabel="搜索">
            <FilterField label="学校状态" group>
              <SegmentedControl
                aria-label="按学校状态筛选"
                size="sm"
                options={[
                  { value: '', label: '全部' },
                  ...TENANT_STATUSES.map((status) => ({
                    value: String(status),
                    label: tenantStatusLabel(status),
                  })),
                ]}
                value={statusFilter}
                onValueChange={setStatusFilter}
              />
            </FilterField>
            <FilterField label="学校名称或短名" htmlFor="schools-keyword">
              <Input
                id="schools-keyword"
                leftIcon={Search}
                value={keyword}
                placeholder="输入关键词"
                onChange={(event) => setKeyword(event.target.value)}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={tenants.page}
            pageSize={tenants.pageSize}
            total={tenants.total}
            onPageChange={tenants.setPage}
          />
        }
      >
          <ResourceState
            resource={tenants}
            emptyIcon={Building}
            emptyTitle={statusFilter || searchTerm ? '没有匹配的学校' : '还没有开通学校'}
            emptyDescription={
              statusFilter || searchTerm
                ? '换个状态或关键词再试,也可以清空条件看全部学校。'
                : '通过入驻申请审核后,学校会出现在这里。'
            }
            emptyAction={
              statusFilter || searchTerm ? undefined : (
                <Button variant="primary" onClick={() => navigate('/platform-admin/applications')}>
                  去入驻申请
                </Button>
              )
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):校名一行、短码与到期一行,状态在右
                mobileCard={(item) => ({
                  title: item.name,
                  meta: `${item.code} · ${item.expire_at ? `到期 ${formatDate(item.expire_at)}` : '长期有效'}`,
                  badge: <StatusIndicator tone={tenantStatusTone(item.status)} label={tenantStatusLabel(item.status)} />,
                })}
              />
            )}
          </ResourceState>
      </DataPanel>

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

interface TenantStatusModalProps {
  tenant: Tenant
  onClose: () => void
  onSaved: () => void
}

/**
 * TenantStatusModal 调整学校状态与服务到期时间。
 * 到期时间随状态一起提交(后端 UpdateTenantStatusRequest 的两个字段),
 * 因此续期就是「状态改回正常 + 延长到期」一次完成。
 */
function TenantStatusModal({ tenant, onClose, onSaved }: TenantStatusModalProps) {
  const [status, setStatus] = useState(String(tenant.status))
  const [expireAt, setExpireAt] = useState(tenant.expire_at ? tenant.expire_at.slice(0, 10) : '')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const statusValue = Number(status) as TenantStatus
  const copy = STATUS_COPY[statusValue]

  const submit = useCallback(async () => {
    setFormError(undefined)
    setWorking(true)
    try {
      await api.identity.updateTenant(tenant.id, {
        status: statusValue,
        // 日期控件给的是本地日期,统一转成当天结束时刻的时间戳再提交
        expire_at: expireAt ? new Date(`${expireAt}T23:59:59`).toISOString() : undefined,
      })
      toast.success('学校状态已更新')
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '更新没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [expireAt, onSaved, statusValue, tenant.id])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{copy.title}</ModalTitle>
          <ModalDescription>{copy.description}</ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            items={[
              { term: '学校', description: tenant.display_name || tenant.name },
              { term: '学校短名', description: tenant.code, mono: true },
              { term: '当前状态', description: tenantStatusLabel(tenant.status) },
              { term: '最近更新', description: formatDateTime(tenant.updated_at), mono: true },
            ]}
          />

          <FormField label="调整为" htmlFor="tenant-status" required>
            <Select
              id="tenant-status"
              options={TENANT_STATUSES.map((item) => ({
                value: String(item),
                label: tenantStatusLabel(item),
              }))}
              value={status}
              onValueChange={setStatus}
            />
          </FormField>

          <FormField
            label="服务到期日"
            htmlFor="tenant-expire"
            helper="留空表示不设到期。到期后师生无法登录,数据保留"
          >
            <Input
              id="tenant-expire"
              type="date"
              value={expireAt}
              onChange={(event) => setExpireAt(event.target.value)}
            />
          </FormField>

          {statusValue !== TenantStatus.ACTIVE ? (
            <Callout tone="warning" title="会立即生效">
              保存后该校师生将无法登录,正在进行的实验与仿真会被停止。
            </Callout>
          ) : null}

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button
            variant={copy.danger ? 'danger' : 'seal'}
            leftIcon={copy.danger ? CircleSlash : ShieldCheck}
            loading={working}
            onClick={() => void submit()}
          >
            确认调整
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
