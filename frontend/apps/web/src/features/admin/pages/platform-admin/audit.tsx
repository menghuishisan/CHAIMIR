// 平台「平台审计」:全平台范围的审计查询(设计 §7.7),平台自己的操作与各校的操作都在这里。
// 每条记录标出所属学校(学校名按页取完解析,平台自己的操作标「平台」);平台管理员的操作带姓名(服务端解析),
// 各校的操作人只显示身份:平台不跨校读取学校人员姓名,核对具体是谁要到该校的审计日志按人筛选。
// 工具条可按学校筛选(只对平台身份生效,写进地址);不选为全平台,含平台自己的操作。

import { useMemo } from 'react'
import {
  PAGINATION_MAX_SIZE,
  TenantStatus,
  type AuditLogEntry,
  type Tenant,
} from '@chaimir/api-client'
import { Badge, Select, Skeleton } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useAsyncResource } from '../../../../hooks'
import { collectAllPages } from '../../../../utils/allPages'
import { tenantStatusLabel } from '../../../../utils/labels/identity'
import { auditActorRoleLabel, auditTargetTypeLabel } from '../../../../utils/labels/admin'
import type { AuditDetailLookups } from '../../auditDetail'
import { AUDIT_ALL, AuditLogPage, type AuditTargetOption } from '../../components/AuditLogPage'

/** 平台审计里常见的对象类型(平台端能产生的操作在前,各校的常见操作在后) */
const TARGET_TYPES = [
  'identity.tenant',
  'identity.tenant_application',
  'identity.platform_admin',
  'identity.platform_auth_session',
  'system_config',
  'alert_rule',
  'alert_event',
  'runtime',
  'runtime_image',
  'tool',
  'tenant_quota',
  'judger',
  'sim_package',
  'vuln_source',
  'identity.account',
  'identity.auth_session',
  'teaching.course',
  'contest',
  'content.item',
  'grade.review',
  'audit_log',
] as const

const TARGET_OPTIONS: readonly AuditTargetOption[] = TARGET_TYPES.map((value) => ({
  value,
  label: auditTargetTypeLabel(value),
}))

/** 学校状态的取值,用于把「更新学校状态」记录里的状态换成文字 */
const TENANT_STATUSES = new Set<number>(
  Object.values(TenantStatus).filter((value) => typeof value === 'number')
)

/** 详情里的附加项:平台端只解析学校状态;学生等人名在平台端读不到,不展示 */
const DETAIL_LOOKUPS: AuditDetailLookups = {
  status: (value) =>
    typeof value === 'number' && TENANT_STATUSES.has(value)
      ? tenantStatusLabel(value as TenantStatus)
      : undefined,
}

/** PlatformAuditPage 全平台审计:按动作、对象、时间筛选,每条标出所属学校 */
export default function PlatformAuditPage() {
  const tenants = useAsyncResource<Tenant[]>(
    () => collectAllPages((params) => api.identity.getTenants(params), PAGINATION_MAX_SIZE),
    [],
    () => false
  )
  const tenantName = useMemo(
    () =>
      new Map(
        (tenants.data ?? []).map((tenant) => [tenant.id, tenant.display_name || tenant.name])
      ),
    [tenants.data]
  )

  /** 学校筛选项:全平台 + 各校(按名称排序);学校名读不到时下拉不可用 */
  const schoolOptions = useMemo(
    () => [
      { value: AUDIT_ALL, label: '全平台' },
      ...[...tenantName.entries()]
        .sort((a, b) => a[1].localeCompare(b[1], 'zh-CN'))
        .map(([value, label]) => ({ value, label })),
    ],
    [tenantName],
  )

  /** schoolText 所属学校的文字;平台自己的操作为「平台」 */
  const schoolText = (entry: AuditLogEntry): string => {
    if (!entry.tenant_id) return '平台'
    if (tenants.status === 'loading') return '正在读取…'
    if (tenants.status === 'error') return '学校名暂时读不到'
    return tenantName.get(entry.tenant_id) ?? '已移除的学校'
  }

  return (
    <AuditLogPage
      title="平台审计"
      description="全平台的敏感操作记录,包括平台运营与各校的操作。只能查询与导出,不能修改或删除。"
      targetOptions={TARGET_OPTIONS}
      tenantColumn={{
        header: '所属',
        render: (entry) =>
          !entry.tenant_id ? (
            <Badge tone="cinnabar">平台</Badge>
          ) : tenants.status === 'loading' ? (
            <Skeleton variant="line" width="half" />
          ) : (
            <span className="text-sm text-ink">{schoolText(entry)}</span>
          ),
        text: schoolText,
      }}
      renderActor={(entry) =>
        entry.actor_name ? (
          <span className="min-w-0">
            <span className="block truncate text-sm font-medium text-ink">{entry.actor_name}</span>
            <span className="block text-xs text-ink-sub">{auditActorRoleLabel(entry.actor_role)}</span>
          </span>
        ) : (
          <span className="text-sm text-ink">{auditActorRoleLabel(entry.actor_role)}</span>
        )
      }
      actorText={(entry) =>
        entry.actor_name
          ? `${entry.actor_name}(${auditActorRoleLabel(entry.actor_role)})`
          : auditActorRoleLabel(entry.actor_role)
      }
      detailLookups={DETAIL_LOOKUPS}
      schoolFilter={{
        render: (value, onChange) => (
          <Select
            aria-label="按学校筛选"
            value={value || AUDIT_ALL}
            onValueChange={(next) => onChange(next === AUDIT_ALL ? '' : next)}
            disabled={tenants.status !== 'success'}
            options={schoolOptions}
            className="w-full"
          />
        ),
      }}
      taskPath="/platform-admin/tasks"
    />
  )
}
