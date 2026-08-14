// 学校管理页(平台侧栏首屏,/platform-admin/schools)。
//
// 平台端的第一个功能页(FE-5:登录直达,不做仪表盘落地页)。一所学校就是一个租户,
// 这里只做「有哪些学校、各自什么状态」的总览与状态调整;学校自己的配置、配额与
// 使用情况在详情深页 —— 列表页塞详情会让常用的状态巡检变慢。
//
// 状态三档由后端 UpdateTenantStatusByPlatform 限定(正常/停用/到期),
// 停用与到期的后果不同,故各自说明再确认。到期时间只在这条接口上可改。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Building, CalendarClock, CircleSlash, Search, ShieldCheck } from 'lucide-react'
import { DeployMode, TenantStatus, type Tenant } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DescriptionList,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Select,
  Stat,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { formatDate, formatDateTime } from '../../../../utils/formatters'
import {
  TENANT_STATUSES,
  deployModeLabel,
  tenantStatusLabel,
  tenantStatusTone,
} from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

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
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [target, setTarget] = useState<Tenant>()

  const tenants = usePagedResource<Tenant>((params) => api.identity.getTenants(params), [])

  // 后端租户列表不带关键词与状态参数,故筛选在本页对当前页做:
  // 平台租户量级远小于账号,分页够用;不自造后端没有的查询参数。
  const visible = useMemo(() => {
    const list = tenants.data ? tenants.data.list : []
    const text = keyword.trim().toLowerCase()
    return list.filter((tenant) => {
      if (statusFilter && tenant.status !== Number(statusFilter)) return false
      if (text === '') return true
      return (
        tenant.name.toLowerCase().includes(text) ||
        tenant.code.toLowerCase().includes(text) ||
        (tenant.display_name ?? '').toLowerCase().includes(text)
      )
    })
  }, [keyword, statusFilter, tenants.data])

  const stats = useMemo(() => {
    const list = tenants.data ? tenants.data.list : []
    return {
      active: list.filter((item) => item.status === TenantStatus.ACTIVE).length,
      saas: list.filter((item) => item.deploy_mode === DeployMode.SAAS).length,
      expiring: list.filter((item) => item.expire_at !== undefined).length,
    }
  }, [tenants.data])

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
        kicker={<Breadcrumb items={[{ label: '租户' }, { label: '学校管理' }]} />}
        title="学校管理"
        description="平台上已开通的学校。停用与到期会立即影响该校师生登录,调整前请确认。"
        icon={Building}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="学校总数" value={tenants.total} icon={Building} />
          <Stat label="正常使用" value={stats.active} icon={ShieldCheck} />
          <Stat label="平台托管" value={stats.saas} icon={Building} hint="其余为学校自建" />
          <Stat label="已设到期" value={stats.expiring} icon={CalendarClock} hint="需要关注续期" />
        </div>
      </PageSection>

      <PageSection
        title="学校列表"
        description={`共 ${tenants.total} 所学校。按名称或短名搜索,按状态筛选。`}
        actions={
          <div className="flex flex-wrap items-end gap-2">
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
            <Input
            aria-label="按学校名称或短名搜索"
              leftIcon={Search}
              value={keyword}
              placeholder="搜索学校"
              onChange={(event) => setKeyword(event.target.value)}
            />
          </div>
        }
      >
        <ResourceState
          resource={tenants}
          emptyIcon={Building}
          emptyTitle="还没有开通学校"
          emptyDescription="通过入驻申请审核后,学校会出现在这里。"
          emptyAction={
            <Button variant="primary" onClick={() => navigate('/platform-admin/applications')}>
              去入驻申请
            </Button>
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <div className="flex flex-col gap-4">
              <Table
                columns={columns}
                data={visible}
                rowKey={(item) => item.id}
                empty={
                  <span className="text-sm text-ink-sub">
                    这一页没有匹配的学校,换个条件或翻页看看。
                  </span>
                }
              />
              <Pagination
                page={tenants.page}
                pageSize={tenants.pageSize}
                total={page.total}
                onPageChange={tenants.setPage}
              />
            </div>
          )}
        </ResourceState>
      </PageSection>

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
