// 平台审计按学校筛选；学校人员仅显示身份，不跨校读取姓名。

import { useMemo } from 'react'
import {
  PAGINATION_MAX_SIZE,
  type AuditLogEntry,
  type Tenant,
} from '@chaimir/api-client'
import { Badge, Select, Skeleton } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useAsyncResource } from '../../../../hooks'
import { collectAllPages } from '../../../../utils/allPages'
import { auditActorRoleLabel, auditTargetTypeLabel } from '../../../../utils/labels/admin'
import { AUDIT_ALL, AuditLogPage, type AuditTargetOption } from '../../components/AuditLogPage'

/** 平台审计里常见的对象类型(平台端能产生的操作在前,各校的常见操作在后) */
const TARGET_TYPES = [
  'identity.tenant',
  'identity.tenant_application',
  'identity.platform_admin',
  'identity.platform_auth_session',
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
