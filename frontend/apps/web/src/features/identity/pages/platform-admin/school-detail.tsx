// 学校详情页(平台深页,/platform-admin/schools/:tenantId)。
//
// 一所学校的全部平台侧信息:基本信息、服务状态、以及沙箱配额(内页区块,对齐清单 §3.4)。
// 配额按租户存储,平台身份必须显式带 tenant_id(后端 quotaScopeTenantID),
// 故配额只在这里出现 —— 运行时详情没有租户上下文,在那里再放一遍等于让管理员先选一次学校。
//
// 学校自己的展示配置(校名、校徽、模块开关、登录方式)由该校管理员在校管端维护,
// 平台端只读:PATCH /tenant/config 走的是「当前租户」语义,平台账号没有当前租户。

import { useCallback, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import {
  Building,
  Cpu,
  Gauge,
  HardDrive,
  MemoryStick,
  Save,
  Timer,
} from 'lucide-react'
import { type SandboxQuota, type Tenant } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  FormField,
  Input,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  Stat,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDate, formatDateTime } from '../../../../utils/formatters'
import {
  authModeLabel,
  deployModeLabel,
  tenantStatusLabel,
  tenantStatusTone,
} from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/**
 * PlatformSchoolDetailPage 呈现单所学校的平台侧信息与沙箱配额。
 */
export default function PlatformSchoolDetailPage() {
  const { tenantId = '' } = useParams<{ tenantId: string }>()
  const navigate = useNavigate()

  const tenant = useAsyncResource(() => api.identity.getTenant(tenantId), [tenantId], () => false)

  return (
    <PageScaffold>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '租户' },
              { label: '学校管理', href: '/platform-admin/schools' },
              { label: tenant.data ? tenant.data.display_name || tenant.data.name : '学校详情' },
            ]}
          />
        }
        title={tenant.data ? tenant.data.display_name || tenant.data.name : '学校详情'}
        description="这所学校的开通信息与沙箱资源配额。展示配置由该校管理员自行维护,平台端只读。"
        icon={Building}
        actions={
          <Button variant="outline" onClick={() => navigate('/platform-admin/schools')}>
            返回学校列表
          </Button>
        }
      />

      <ResourceState
        resource={tenant}
        emptyIcon={Building}
        emptyTitle="没有找到这所学校"
        emptyDescription="学校可能已被移除,回列表重新选择。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="block" />
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {(data) => <SchoolOverview tenant={data} />}
      </ResourceState>

      <QuotaSection tenantId={tenantId} />
    </PageScaffold>
  )
}

/**
 * SchoolOverview 渲染学校的开通信息与当前配置快照。
 */
function SchoolOverview({ tenant }: { tenant: Tenant }) {
  return (
    <>
      <PageSection title="服务状态" description="状态与到期时间在学校列表页调整。">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat
            label="当前状态"
            value={tenantStatusLabel(tenant.status)}
            icon={Building}
            hint={tenant.status === 1 ? '师生可正常登录' : '师生登录已被拒绝'}
          />
          <Stat
            label="服务到期"
            value={tenant.expire_at ? formatDate(tenant.expire_at) : '未设置'}
            icon={Timer}
            hint={tenant.expire_at ? '到期后自动停止服务' : '不限期'}
          />
          <Stat label="部署形态" value={deployModeLabel(tenant.deploy_mode)} icon={HardDrive} />
        </div>
      </PageSection>

      <PageSection title="开通信息" description="学校短名在开通时确定,不能修改。">
        <div className="flex flex-col gap-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0">
              <h3 className="text-base font-semibold text-ink">{tenant.name}</h3>
              <p className="mt-0.5 text-sm text-ink-sub">
                {tenant.display_name ? `对外显示为「${tenant.display_name}」` : '未设置对外显示名'}
              </p>
            </div>
            <StatusIndicator
              tone={tenantStatusTone(tenant.status)}
              label={tenantStatusLabel(tenant.status)}
            />
          </div>

          <DescriptionList
            columns={2}
            items={[
              { term: '学校短名', description: tenant.code, mono: true },
              { term: '登录方式', description: authModeLabel(tenant.auth_mode) },
              {
                term: '激活码开通',
                description: tenant.enable_activation_code ? '已启用' : '已关闭',
              },
              { term: '开通时间', description: formatDateTime(tenant.created_at), mono: true },
              { term: '最近更新', description: formatDateTime(tenant.updated_at), mono: true },
            ]}
          />

          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm text-ink-sub">已开启的模块:</span>
            <TenantModuleBadges featureFlags={tenant.feature_flags} />
          </div>
        </div>
      </PageSection>
    </>
  )
}

/** 模块开关在租户配置里的键,与校管端「租户配置」页写入的键一致。 */
const MODULES_KEY = 'modules'

/** 模块名称登记表:未登记的键不显示,避免把内部标识抛到界面上。 */
const MODULE_LABELS: Record<string, string> = {
  teaching: '教学',
  experiment: '实验',
  contest: '竞赛',
  sim: '仿真',
  grade: '成绩',
}

/**
 * TenantModuleBadges 展示该校已开启的功能模块。
 * feature_flags 是开放 JSONB,只呈现已登记的模块键;学校没配过就显示默认口径。
 */
function TenantModuleBadges({ featureFlags }: { featureFlags: Record<string, unknown> }) {
  const raw = featureFlags[MODULES_KEY]
  const modules = Array.isArray(raw) ? raw.filter((item): item is string => typeof item === 'string') : []
  const known = modules.filter((code) => MODULE_LABELS[code] !== undefined)

  if (known.length === 0) {
    return <span className="text-sm text-ink-sub">按平台默认(全部模块可用)</span>
  }
  return (
    <span className="flex flex-wrap gap-1">
      {known.map((code) => (
        <Badge key={code} tone="jade">
          {MODULE_LABELS[code]}
        </Badge>
      ))}
    </span>
  )
}

/**
 * QuotaSection 读取并调整这所学校的沙箱配额。
 * 后端 validateQuota 要求并发/CPU/内存/空闲超时/最长存活都为正数,
 * 保活与快照保留可以为 0(表示不允许),故前端按同一口径校验,不让保存到后端才报错。
 */
function QuotaSection({ tenantId }: { tenantId: string }) {
  const quota = useAsyncResource(
    () => api.sandbox.getQuota({ tenant_id: tenantId }),
    [tenantId],
    () => false,
  )

  return (
    <PageSection
      title="实验环境资源配额"
      description="这所学校能同时开的实验环境数量与单校资源上限。调低不会影响已在运行的环境,下一次创建时生效。"
    >
      <ResourceState
        resource={quota}
        emptyIcon={Gauge}
        emptyTitle="还没有配额记录"
        emptyDescription="学校首次创建沙箱时会按平台默认配额建档,也可以在这里先设定。"
        skeleton={<Skeleton variant="line" lines={4} />}
      >
        {(data) => <QuotaForm quota={data} tenantId={tenantId} onSaved={quota.reload} />}
      </ResourceState>
    </PageSection>
  )
}

/** 配额字段登记:名称、说明与是否允许为 0,与后端 validateQuota 一致。 */
const QUOTA_FIELDS = [
  {
    key: 'max_concurrent_sandbox' as const,
    label: '同时运行的沙箱数',
    helper: '全校同一时刻最多能开这么多个实验或答题环境',
    allowZero: false,
  },
  {
    key: 'max_cpu' as const,
    label: 'CPU 核数上限',
    helper: '全校沙箱合计可占用的核数',
    allowZero: false,
  },
  {
    key: 'max_memory_mb' as const,
    label: '内存上限(MB)',
    helper: '全校沙箱合计可占用的内存',
    allowZero: false,
  },
  {
    key: 'idle_timeout_min' as const,
    label: '无操作回收时长(分钟)',
    helper: '学生离开这么久后自动回收环境,释放资源',
    allowZero: false,
  },
  {
    key: 'max_lifetime_min' as const,
    label: '单次最长存活(分钟)',
    helper: '一个环境从创建起最多存活这么久',
    allowZero: false,
  },
  {
    key: 'max_keepalive_min' as const,
    label: '可申请的保活时长(分钟)',
    helper: '填 0 表示不允许学生申请延长',
    allowZero: true,
  },
  {
    key: 'max_snapshot_retention_min' as const,
    label: '快照保留时长(分钟)',
    helper: '填 0 表示不保留快照,环境回收即清空',
    allowZero: true,
  },
]

type QuotaFieldKey = (typeof QUOTA_FIELDS)[number]['key']

interface QuotaFormProps {
  quota: SandboxQuota
  tenantId: string
  onSaved: () => void
}

/**
 * QuotaForm 编辑七项配额并提交。
 */
function QuotaForm({ quota, tenantId, onSaved }: QuotaFormProps) {
  const [values, setValues] = useState<Record<QuotaFieldKey, string>>(() => ({
    max_concurrent_sandbox: String(quota.max_concurrent_sandbox),
    max_cpu: String(quota.max_cpu),
    max_memory_mb: String(quota.max_memory_mb),
    idle_timeout_min: String(quota.idle_timeout_min),
    max_lifetime_min: String(quota.max_lifetime_min),
    max_keepalive_min: String(quota.max_keepalive_min),
    max_snapshot_retention_min: String(quota.max_snapshot_retention_min),
  }))
  const [errors, setErrors] = useState<Partial<Record<QuotaFieldKey, string>>>({})
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const nextErrors: Partial<Record<QuotaFieldKey, string>> = {}
      const numbers = {} as Record<QuotaFieldKey, number>
      for (const field of QUOTA_FIELDS) {
        const parsed = Number(values[field.key])
        if (!Number.isInteger(parsed) || parsed < 0 || (!field.allowZero && parsed === 0)) {
          nextErrors[field.key] = field.allowZero ? '请填 0 或更大的整数' : '请填大于 0 的整数'
          continue
        }
        numbers[field.key] = parsed
      }
      setErrors(nextErrors)
      if (Object.keys(nextErrors).length > 0) {
        setFormError('有几项填得不对,按提示改一下再保存。')
        return
      }

      setFormError(undefined)
      setWorking(true)
      try {
        await api.sandbox.updateQuota({ tenant_id: tenantId, ...numbers })
        toast.success('配额已保存')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [onSaved, tenantId, values],
  )

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Stat
          label="当前活跃实验环境"
          value={quota.active_sandbox_count ?? 0}
          icon={Cpu}
          hint={`并发上限 ${quota.max_concurrent_sandbox}`}
        />
        <Stat label="CPU 上限" value={`${quota.max_cpu} 核`} icon={Cpu} />
        <Stat label="内存上限" value={`${quota.max_memory_mb} MB`} icon={MemoryStick} />
      </div>

      <Card>
        <CardHeader
          title="调整配额"
          description="改动只影响之后创建的环境,不会中断正在运行的实验。"
        />
        <CardBody>
          <form className="flex flex-col gap-4" onSubmit={submit} noValidate>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {QUOTA_FIELDS.map((field) => (
                <FormField
                  key={field.key}
                  label={field.label}
                  htmlFor={`quota-${field.key}`}
                  required
                  helper={field.helper}
                  error={errors[field.key]}
                >
                  <Input
                    id={`quota-${field.key}`}
                    type="number"
                    min={field.allowZero ? 0 : 1}
                    value={values[field.key]}
                    invalid={Boolean(errors[field.key])}
                    onChange={(event) =>
                      setValues((current) => ({ ...current, [field.key]: event.target.value }))
                    }
                  />
                </FormField>
              ))}
            </div>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}

            <div className="flex items-center gap-2">
              <Button type="submit" variant="seal" leftIcon={Save} loading={working}>
                保存配额
              </Button>
              <span className="text-sm text-ink-sub">
                超出配额时学生创建环境会被拒绝,并提示稍后再试。
              </span>
            </div>
          </form>
        </CardBody>
      </Card>
    </div>
  )
}
