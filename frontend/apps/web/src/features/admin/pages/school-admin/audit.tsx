// 学校审计页只负责本校账号解析、筛选选项和操作者展示。
// 查询、导出、分页与结果骨架复用 admin 领域的 AuditLogPage。

import { useMemo } from 'react'
import { PAGINATION_MAX_SIZE, type Account, type AuditLogEntry } from '@chaimir/api-client'
import type { SelectOption, TableColumn } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useAsyncResource } from '../../../../hooks'
import { auditActorRoleLabel } from '../../../../utils/labels/admin'
import { AuditLogPage } from '../../components/AuditLogPage'

/** 本校审计可筛选的对象类型。 */
const TARGET_TYPE_OPTIONS = [
  { value: '', label: '全部对象' },
  { value: 'account', label: '账号' },
  { value: 'course', label: '课程' },
  { value: 'assignment', label: '作业' },
  { value: 'experiment', label: '实验' },
  { value: 'contest', label: '竞赛' },
  { value: 'grade.review', label: '成绩审核' },
  { value: 'grade.appeal', label: '成绩申诉' },
  { value: 'content.item', label: '题目' },
  { value: 'session', label: '登录会话' },
] as const

/** 本校审计常用动作。 */
const ACTION_OPTIONS = [
  { value: '', label: '全部动作' },
  { value: 'auth.login', label: '登录' },
  { value: 'account.create', label: '创建账号' },
  { value: 'account.disable', label: '停用账号' },
  { value: 'account.reset_password', label: '重置账号密码' },
  { value: 'account.grant_admin', label: '授予管理员' },
  { value: 'account.import', label: '批量导入账号' },
  { value: 'tenant.config.update', label: '修改学校配置' },
  { value: 'grade.review.approve', label: '通过成绩审核' },
  { value: 'grade.review.unlock', label: '解锁成绩审核' },
  { value: 'grade.appeal.accept', label: '受理成绩申诉' },
] as const

/** SchoolAdminAuditPage 提供本校管理身份可见的审计范围配置。 */
export default function SchoolAdminAuditPage() {
  const accounts = useAsyncResource(
    () => api.identity.getAccounts({ page: 1, size: PAGINATION_MAX_SIZE }),
    [],
    () => false,
  )

  const accountById = useMemo(
    () => new Map((accounts.data?.list ?? []).map((account: Account) => [account.id, account])),
    [accounts.data],
  )

  const actorOptions = useMemo<SelectOption[]>(
    () => [
      { value: '', label: '全部操作者' },
      ...(accounts.data?.list ?? []).map((account: Account) => ({
        value: account.id,
        label: account.no ? `${account.name} · ${account.no}` : account.name,
      })),
    ],
    [accounts.data],
  )

  const subjectColumn = useMemo<TableColumn<AuditLogEntry>>(
    () => ({
      key: 'actor_id',
      header: '操作者',
      render: (entry) => {
        const account = accountById.get(entry.actor_id)
        return (
          <div className="min-w-0">
            <div className="truncate text-ink">{account ? account.name : '系统或已离校人员'}</div>
            <div className="truncate text-xs text-ink-sub">{auditActorRoleLabel(entry.actor_role)}</div>
          </div>
        )
      },
    }),
    [accountById],
  )

  return (
    <AuditLogPage
      idPrefix="audit"
      breadcrumbItems={[{ label: '系统配置' }, { label: '审计日志' }]}
      title="审计日志"
      description="本校范围内的敏感操作记录。只能查询与导出,不能修改或删除。"
      actionOptions={ACTION_OPTIONS}
      targetTypeOptions={TARGET_TYPE_OPTIONS}
      subjectColumn={subjectColumn}
      actorOptions={actorOptions}
      taskPath="/school-admin/tasks"
    />
  )
}
