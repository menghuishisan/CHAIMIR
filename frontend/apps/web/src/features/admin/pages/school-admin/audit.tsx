// 本校审计复用服务端操作人信息；详情中的学生按需读取。

import { useMemo, useState } from 'react'
import {
  API_ERROR_CODES,
  ApiError,
  AuditActorRole,
  type Account,
  type AuditLogEntry,
} from '@chaimir/api-client'
import { api } from '../../../../app/api'
import { useAsyncResource, type AsyncResourceState } from '../../../../hooks'
import { AccountPicker } from '../../../identity/components/AccountPicker'
import { AccountNameText } from '../../../identity/components/AccountNameText'
import { auditActorRoleLabel, auditTargetTypeLabel } from '../../../../utils/labels/admin'
import { auditDetailText, type AuditDetailLookups } from '../../auditDetail'
import { AuditLogPage, type AuditTargetOption } from '../../components/AuditLogPage'

/** 本校审计里会出现的对象类型(学校端能产生的操作) */
const TARGET_TYPES = [
  'identity.account',
  'identity.class',
  'identity.major',
  'identity.department',
  'identity.import_batch',
  'identity.tenant',
  'identity.sso_config',
  'identity.auth_session',
  'teaching.course',
  'teaching.lesson',
  'teaching.assignment',
  'teaching.submission',
  'teaching.grade',
  'experiment',
  'experiment_report',
  'contest',
  'contest_team',
  'content.item',
  'content.paper',
  'grade.review',
  'grade.appeal',
  'grade.transcript',
  'judge_task',
  'sandbox',
  'notify.announcement',
  'alert_rule',
] as const

const TARGET_OPTIONS: readonly AuditTargetOption[] = TARGET_TYPES.map((value) => ({
  value,
  label: auditTargetTypeLabel(value),
}))

/** 系统自动产生的记录没有操作人(编号为 0) */
function isSystem(entry: AuditLogEntry): boolean {
  return entry.actor_role === AuditActorRole.SYSTEM || entry.actor_id === '0'
}

/** actorLine 操作人写成一行文字(抽屉与窄屏行卡用):系统、平台管理员、本校账号各有写法 */
function actorLine(entry: AuditLogEntry): string {
  if (isSystem(entry)) return '系统'
  if (entry.actor_role === AuditActorRole.PLATFORM_ADMIN)
    return auditActorRoleLabel(entry.actor_role)
  if (!entry.actor_name) return '已移除的账号'
  return entry.actor_no ? `${entry.actor_name}(${entry.actor_no})` : entry.actor_name
}

/** studentText 详情里那位学生的一行文字:读取中、读不到、已移除各有说法 */
function studentText(student: AsyncResourceState<Account | null>): string {
  if (student.status === 'error') {
    return student.error instanceof ApiError &&
      student.error.code === API_ERROR_CODES.RESOURCE_NOT_FOUND
      ? '已移除的账号'
      : '姓名暂时读不到'
  }
  if (student.status === 'loading' || !student.data) return '正在读取…'
  return student.data.no ? `${student.data.name}(${student.data.no})` : student.data.name
}

/** SchoolAdminAuditPage 本校审计:按人、动作、对象、时间筛选 */
export default function SchoolAdminAuditPage() {
  // 详情里记录的学生:打开那一条时按编号读一次
  const [detailStudentId, setDetailStudentId] = useState<string>()
  const detailStudent = useAsyncResource<Account | null>(
    () => (detailStudentId ? api.identity.getAccount(detailStudentId) : Promise.resolve(null)),
    [detailStudentId],
    () => false
  )

  const detailLookups = useMemo<AuditDetailLookups>(
    () => ({
      student_id: (value) =>
        typeof value === 'string' && value === detailStudentId
          ? studentText(detailStudent)
          : undefined,
    }),
    [detailStudent, detailStudentId]
  )

  return (
    <AuditLogPage
      title="审计日志"
      description="本校的敏感操作记录,只能查询与导出,不能修改或删除。"
      targetOptions={TARGET_OPTIONS}
      renderActor={(entry) =>
        isSystem(entry) || entry.actor_role === AuditActorRole.PLATFORM_ADMIN ? (
          <span className="text-ink-sub">{actorLine(entry)}</span>
        ) : (
          <span className="min-w-0">
            <AccountNameText name={entry.actor_name} no={entry.actor_no} compact />
            <span className="block text-xs text-ink-sub">
              {auditActorRoleLabel(entry.actor_role)}
            </span>
          </span>
        )
      }
      actorText={actorLine}
      detailLookups={detailLookups}
      onDetailOpen={(entry) => setDetailStudentId(auditDetailText(entry.detail, 'student_id'))}
      actorFilter={{
        render: (value, onChange) => (
          <AccountPicker
            value={value}
            onChange={onChange}
            allLabel="全部操作人"
            label="按操作人筛选"
          />
        ),
      }}
      taskPath="/school-admin/tasks"
    />
  )
}
