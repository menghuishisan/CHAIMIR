// 平台审计页只负责全平台范围的学校解析、筛选选项和范围说明。
// 查询、导出、分页与结果骨架复用 admin 领域的 AuditLogPage。

import { useMemo } from 'react'
import { PAGINATION_MAX_SIZE, type AuditLogEntry, type Tenant } from '@chaimir/api-client'
import { Badge, type TableColumn } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useAsyncResource } from '../../../../hooks'
import { auditActorRoleLabel } from '../../../../utils/labels/admin'
import { AuditLogPage } from '../../components/AuditLogPage'

/** 平台审计可筛选的对象类型。 */
const TARGET_TYPE_OPTIONS = [
  { value: '', label: '全部对象' },
  { value: 'tenant', label: '学校' },
  { value: 'account', label: '账号' },
  { value: 'config', label: '系统配置' },
  { value: 'alert', label: '告警' },
  { value: 'sim.package', label: '仿真场景包' },
  { value: 'session', label: '登录会话' },
] as const

/** 平台审计常用动作。 */
const ACTION_OPTIONS = [
  { value: '', label: '全部动作' },
  { value: 'auth.login', label: '登录' },
  { value: 'platform.application.approve', label: '通过入驻申请' },
  { value: 'platform.application.reject', label: '驳回入驻申请' },
  { value: 'platform.tenant.update', label: '修改学校信息' },
  { value: 'admin.config.update', label: '修改系统配置' },
  { value: 'admin.config.rollback', label: '回滚系统配置' },
  { value: 'admin.alert.handle', label: '处理告警' },
  { value: 'sim.review.approve', label: '通过仿真包审核' },
  { value: 'sim.review.reject', label: '退回仿真包' },
] as const

/** PlatformAuditPage 提供平台身份可见的审计范围配置。 */
export default function PlatformAuditPage() {
  const tenants = useAsyncResource(
    () => api.identity.getTenants({ page: 1, size: PAGINATION_MAX_SIZE }),
    [],
    () => false,
  )

  const tenantNameById = useMemo(
    () =>
      new Map(
        (tenants.data?.list ?? []).map((tenant: Tenant) => [
          tenant.id,
          tenant.display_name || tenant.name,
        ]),
      ),
    [tenants.data],
  )

  const subjectColumn = useMemo<TableColumn<AuditLogEntry>>(
    () => ({
      key: 'tenant_id',
      header: '学校',
      render: (entry) => (
        <div className="min-w-0">
          {entry.tenant_id ? (
            <span className="text-ink">{tenantNameById.get(entry.tenant_id) ?? '已移除的学校'}</span>
          ) : (
            <Badge tone="cinnabar">平台</Badge>
          )}
          <div className="truncate text-xs text-ink-sub">{auditActorRoleLabel(entry.actor_role)}</div>
        </div>
      ),
    }),
    [tenantNameById],
  )

  return (
    <AuditLogPage
      idPrefix="platform-audit"
      breadcrumbItems={[{ label: '底层资源' }, { label: '平台审计' }]}
      title="平台审计"
      description="全平台的敏感操作记录,含各学校内部的操作。只能查询与导出,不能修改或删除。"
      actionOptions={ACTION_OPTIONS}
      targetTypeOptions={TARGET_TYPE_OPTIONS}
      subjectColumn={subjectColumn}
      taskPath="/platform-admin/tasks"
      scopeNote={
        <>
          这里按身份呈现操作者(平台管理员、学校管理员、教师、学生)。要看具体是谁,
          请在那所学校的管理端审计页按人筛选 —— 跨学校的账号姓名不在平台侧解析。
        </>
      }
    />
  )
}
