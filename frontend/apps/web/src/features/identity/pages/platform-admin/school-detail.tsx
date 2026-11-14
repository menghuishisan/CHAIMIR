// 学校详情(平台深页,/platform-admin/schools/:tenantId)。设计 §7.6:
// 页头为校名 + 状态 + 一行属性(短码 · 部署形态 · 服务到期 · 开通时间),右侧「调整状态」(与列表页同一弹窗);
// 左栏「开通信息」只读列表;右栏「实验环境资源配额」:当前运行数 / 上限的使用比例条,下方配额表单,
// 有改动时才出现「保存配额」,并说明「只影响之后创建的环境」。
// 学校自己的展示配置(显示名、校徽、模块、登录方式)由该校管理员维护,平台端只读。

import { useMemo, useState } from 'react'
import { useParams } from 'react-router'
import { Building, Save } from 'lucide-react'
import { API_ERROR_CODES, ApiError, type SandboxQuota, type Tenant } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  cn,
  DescriptionList,
  FormField,
  Input,
  PageHeader,
  PageScaffold,
  Skeleton,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { locationLabel, useAsyncResource, usePageLocation } from '../../../../hooks'
import { formatDate, formatDateTime } from '../../../../utils/formatters'
import {
  authModeLabel,
  deployModeLabel,
  schoolTypeLabel,
  TENANT_MODULE_LABELS,
  tenantStatusLabel,
} from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { TenantExpiryText } from '../../components/TenantExpiryText'
import { TenantStatusModal } from '../../components/TenantStatusModal'
import { tenantStatusTone } from '../../statusPresentation'
import { readTenantModules } from '../../tenantModules'

/** PlatformSchoolDetailPage 一所学校:开通信息与实验环境资源配额 */
export default function PlatformSchoolDetailPage() {
  const { tenantId = '' } = useParams<{ tenantId: string }>()
  const [statusOpen, setStatusOpen] = useState(false)

  const tenant = useAsyncResource(
    async () => {
      try {
        return await api.identity.getTenant(tenantId)
      } catch (error) {
        // 学校不存在时给「没有找到」,不当作读取失败
        if (error instanceof ApiError && error.code === API_ERROR_CODES.RESOURCE_NOT_FOUND)
          return null
        throw error
      }
    },
    [tenantId],
    (value) => value === null
  )

  usePageLocation([
    {
      label: locationLabel(
        tenant,
        (data) => (data ? data.display_name || data.name : ''),
        '学校详情'
      ),
    },
  ])

  return (
    <PageScaffold>
      <ResourceState
        resource={tenant}
        emptyIcon={Building}
        emptyTitle="没有找到这所学校"
        emptyDescription="学校可能已被移除,回学校管理重新选择。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="title" width="half" />
            <Skeleton variant="block" />
          </div>
        }
      >
        {(data) =>
          data ? (
            <>
              <PageHeader
                title={data.display_name || data.name}
                status={
                  <StatusIndicator
                    tone={tenantStatusTone(data.status)}
                    label={tenantStatusLabel(data.status)}
                  />
                }
                meta={[
                  { value: <span className="font-mono">{data.code}</span> },
                  { value: deployModeLabel(data.deploy_mode) },
                  { label: '服务到期', value: <TenantExpiryText expireAt={data.expire_at} /> },
                  { label: '开通于', value: formatDate(data.created_at) },
                ]}
                actions={
                  <Button variant="outline" onClick={() => setStatusOpen(true)}>
                    调整状态
                  </Button>
                }
              />
              <div className="grid gap-6 lg:grid-cols-2">
                <ProvisionCard tenant={data} />
                <QuotaCard key={data.id} tenantId={data.id} />
              </div>
              {statusOpen ? (
                <TenantStatusModal
                  tenant={data}
                  onClose={() => setStatusOpen(false)}
                  onSaved={() => {
                    setStatusOpen(false)
                    tenant.reload()
                  }}
                />
              ) : null}
            </>
          ) : null
        }
      </ResourceState>
    </PageScaffold>
  )
}

/** ProvisionCard 开通信息:只读;展示配置由该校管理员在校管端维护 */
function ProvisionCard({ tenant }: { tenant: Tenant }) {
  const modules = readTenantModules(tenant.feature_flags)
  return (
    <Card>
      <CardHeader
        title="开通信息"
        description="学校的显示名、校徽、模块与登录方式由该校管理员维护,这里只读。"
      />
      <CardBody>
        <DescriptionList
          items={[
            { term: '学校全称', description: tenant.name },
            { term: '对外显示名', description: tenant.display_name || '未设置,显示学校全称' },
            { term: '学校短码', description: tenant.code, mono: true },
            { term: '机构类型', description: schoolTypeLabel(tenant.type) },
            { term: '部署形态', description: deployModeLabel(tenant.deploy_mode) },
            { term: '登录方式', description: authModeLabel(tenant.auth_mode) },
            {
              term: '激活码开通账号',
              description: tenant.enable_activation_code ? '已开启' : '已关闭',
            },
            {
              term: '已开启的模块',
              description:
                modules.length === 0 ? (
                  '没有开启业务模块'
                ) : (
                  <span className="flex flex-wrap gap-1">
                    {modules.map((code) => (
                      <Badge key={code} tone="jade">
                        {TENANT_MODULE_LABELS[code]}
                      </Badge>
                    ))}
                  </span>
                ),
            },
            { term: '开通时间', description: formatDateTime(tenant.created_at), mono: true },
            { term: '最近更新', description: formatDateTime(tenant.updated_at), mono: true },
          ]}
        />
      </CardBody>
    </Card>
  )
}

/** 配额字段:名称、说明与是否允许为 0,与后端 validateQuota 一致 */
const QUOTA_FIELDS = [
  {
    key: 'max_concurrent_sandbox',
    label: '同时运行的环境数',
    helper: '全校同一时刻最多能开的实验或答题环境',
    allowZero: false,
  },
  { key: 'max_cpu', label: 'CPU 核数上限', helper: '全校环境合计可占用的核数', allowZero: false },
  {
    key: 'max_memory_mb',
    label: '内存上限(MB)',
    helper: '全校环境合计可占用的内存',
    allowZero: false,
  },
  {
    key: 'idle_timeout_min',
    label: '无操作回收(分钟)',
    helper: '学生离开这么久后自动回收环境',
    allowZero: false,
  },
  {
    key: 'max_lifetime_min',
    label: '单次最长存活(分钟)',
    helper: '一个环境从创建起最多存活这么久',
    allowZero: false,
  },
  {
    key: 'max_keepalive_min',
    label: '可申请保活(分钟)',
    helper: '填 0 表示不允许延长',
    allowZero: true,
  },
  {
    key: 'max_snapshot_retention_min',
    label: '快照保留(分钟)',
    helper: '填 0 表示回收即清空,不保留快照',
    allowZero: true,
  },
] as const

type QuotaKey = (typeof QUOTA_FIELDS)[number]['key']

/** quotaDraft 把配额换成表单的文本值 */
function quotaDraft(quota: SandboxQuota): Record<QuotaKey, string> {
  return Object.fromEntries(
    QUOTA_FIELDS.map((field) => [field.key, String(quota[field.key])])
  ) as Record<QuotaKey, string>
}

/** QuotaCard 读取这所学校的配额;平台身份读配额必须带学校编号 */
function QuotaCard({ tenantId }: { tenantId: string }) {
  const quota = useAsyncResource(
    () => api.sandbox.getQuota({ tenant_id: tenantId }),
    [tenantId],
    () => false
  )
  return (
    <Card>
      <CardHeader
        title="实验环境资源配额"
        description="改动只影响之后创建的环境,正在运行的不受影响。"
      />
      <CardBody>
        <ResourceState
          resource={quota}
          emptyIcon={Building}
          emptyTitle="还没有配额记录"
          emptyDescription="学校第一次创建环境时会按平台默认配额建档。"
          skeleton={<Skeleton variant="line" lines={5} />}
        >
          {(data) => (
            <QuotaForm
              key={JSON.stringify(data)}
              quota={data}
              tenantId={tenantId}
              onSaved={quota.reload}
            />
          )}
        </ResourceState>
      </CardBody>
    </Card>
  )
}

interface QuotaFormProps {
  quota: SandboxQuota
  tenantId: string
  onSaved: () => void
}

/** QuotaForm 使用比例条 + 七项配额;有改动时才出现保存 */
function QuotaForm({ quota, tenantId, onSaved }: QuotaFormProps) {
  const initial = useMemo(() => quotaDraft(quota), [quota])
  const [values, setValues] = useState(initial)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const errors = useMemo(() => {
    const next: Partial<Record<QuotaKey, string>> = {}
    for (const field of QUOTA_FIELDS) {
      const raw = values[field.key].trim()
      const parsed = Number(raw)
      if (
        raw === '' ||
        !Number.isInteger(parsed) ||
        parsed < 0 ||
        (!field.allowZero && parsed === 0)
      ) {
        next[field.key] = field.allowZero ? '请填 0 或更大的整数' : '请填大于 0 的整数'
      }
    }
    return next
  }, [values])
  const dirty = QUOTA_FIELDS.some((field) => values[field.key].trim() !== initial[field.key])
  const valid = Object.keys(errors).length === 0

  const active = quota.active_sandbox_count ?? 0
  const limit = quota.max_concurrent_sandbox
  const share = limit > 0 ? Math.min(100, Math.round((active / limit) * 100)) : 0

  /** save 提交七项配额 */
  const save = async () => {
    if (!valid) return
    setFormError(undefined)
    setWorking(true)
    try {
      await api.sandbox.updateQuota({
        tenant_id: tenantId,
        ...(Object.fromEntries(
          QUOTA_FIELDS.map((field) => [field.key, Number(values[field.key])])
        ) as Record<QuotaKey, number>),
      })
      toast.success('配额已保存,之后创建的环境按新配额执行')
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '配额没有保存成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-1.5">
        <p className="flex items-baseline justify-between text-sm text-ink-sub">
          <span>正在运行的环境</span>
          <span className="tabular-nums">
            <span className="text-lg font-semibold text-ink">{active}</span> / {limit}
          </span>
        </p>
        <div
          role="progressbar"
          aria-label="实验环境并发使用比例"
          aria-valuenow={share}
          aria-valuemin={0}
          aria-valuemax={100}
          className="h-2 overflow-hidden rounded-full bg-surface-sunken"
        >
          {/* 动态宽度:使用比例来自接口数据,属于运行时计算值 */}
          <span
            className={cn('block h-full', share >= 85 ? 'bg-warning' : 'bg-primary')}
            style={{ width: `${share}%` }}
          />
        </div>
        <p className="text-xs text-ink-sub">
          已用 {share}%{share >= 85 ? ',接近上限,学生新开环境可能会被拒绝' : ''}
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        {QUOTA_FIELDS.map((field) => (
          <FormField
            key={field.key}
            label={field.label}
            htmlFor={`quota-${field.key}`}
            required
            helper={field.helper}
            error={dirty ? errors[field.key] : undefined}
          >
            <Input
              id={`quota-${field.key}`}
              type="number"
              inputMode="numeric"
              min={field.allowZero ? 0 : 1}
              value={values[field.key]}
              invalid={dirty && Boolean(errors[field.key])}
              onChange={(event) =>
                setValues((current) => ({ ...current, [field.key]: event.target.value }))
              }
            />
          </FormField>
        ))}
      </div>

      {formError ? <Callout tone="danger">{formError}</Callout> : null}

      {dirty ? (
        <div className="flex flex-wrap items-center gap-3">
          <Button
            variant="primary"
            leftIcon={Save}
            loading={working}
            disabled={!valid}
            onClick={() => void save()}
          >
            保存配额
          </Button>
          <Button variant="ghost" disabled={working} onClick={() => setValues(initial)}>
            放弃修改
          </Button>
          <span className="text-sm text-ink-sub">只影响之后创建的环境。</span>
        </div>
      ) : null}
    </div>
  )
}
