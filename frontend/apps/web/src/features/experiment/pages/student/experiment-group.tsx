// 实验小组卡(实验详情页头下方)。设计 §7.2:组名、成员、我的角色。
// 小组信息走 GET /experiment/groups/{id} —— 学生没有小组发现接口,
// 组号唯一来源是学生实验投影的 my_group_id(见 M7 接口设计 §3.1)。
// 「我」由服务端会话里的账号编号认出,不接受客户端传参。

import { Users } from 'lucide-react'
import type { ExperimentGroup } from '@chaimir/api-client'
import { Badge, Skeleton } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { useSession } from '../../../../components/RoleGuard'
import { useAsyncResource } from '../../../../hooks'

export interface ExperimentGroupCardProps {
  groupId: string
}

/**
 * ExperimentGroupCard 展示本人所在小组:组名、成员与分工,本人带「我」标记。
 */
export function ExperimentGroupCard({ groupId }: ExperimentGroupCardProps) {
  const group = useAsyncResource(() => api.experiment.getGroup(groupId), [groupId], () => false)

  return (
    <section aria-labelledby="experiment-group-title" className="rounded-lg bg-surface p-5 shadow-xs">
      <h2 id="experiment-group-title" className="flex items-center gap-2 text-sm font-medium text-ink-sub">
        <Users aria-hidden="true" className="size-4" />
        我的小组
      </h2>
      {group.status === 'loading' ? (
        <div className="mt-3 flex flex-col gap-2" aria-busy="true">
          <Skeleton variant="title" width="third" />
          <Skeleton variant="line" lines={2} />
        </div>
      ) : group.status === 'error' || group.data === null ? (
        <ResourceError
          message={group.error?.message ?? '小组信息暂时读取不到'}
          traceId={group.error?.traceId}
          onRetry={group.reload}
        />
      ) : (
        <GroupMembers group={group.data} />
      )}
    </section>
  )
}

/** GroupMembers 小组名与成员:姓名、学号、分工;本人排在第一位并带「我」 */
function GroupMembers({ group }: { group: ExperimentGroup }) {
  const { me } = useSession()
  const members = [...group.members].sort(
    (a, b) => Number(b.student_id === me.account.id) - Number(a.student_id === me.account.id),
  )
  return (
    <div className="mt-2">
      <p className="text-lg font-semibold text-ink">{group.name}</p>
      <ul className="mt-3 flex flex-wrap gap-x-6 gap-y-3">
        {members.map((member) => {
          const mine = member.student_id === me.account.id
          return (
            <li key={member.id} className="flex items-center gap-2.5">
              <span
                aria-hidden="true"
                className="grid size-8 shrink-0 place-items-center rounded-full bg-primary-soft text-sm font-medium text-primary"
              >
                {member.student_name.slice(0, 1)}
              </span>
              <span className="min-w-0">
                <span className="flex items-center gap-1.5 text-sm font-medium text-ink">
                  {member.student_name}
                  {mine ? <Badge tone="jade">我</Badge> : null}
                </span>
                <span className="block text-xs text-ink-sub">
                  {[member.student_no, member.role || '未分工'].filter(Boolean).join(' · ')}
                </span>
              </span>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
