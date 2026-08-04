// 实验小组卡(实验详情页右栏)。
// 小组信息走 GET /experiment/groups/{id} —— 学生没有小组发现接口,
// 组号唯一来源是学生实验投影的 my_group_id(见 M7 接口设计 §3.1)。

import { Users } from 'lucide-react'
import type { ExperimentGroup } from '@chaimir/api-client'
import { Badge, Card, CardBody, CardHeader, Skeleton } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'

export interface ExperimentGroupCardProps {
  groupId: string
}

/**
 * ExperimentGroupCard 展示本人所在小组与成员分工。
 */
export function ExperimentGroupCard({ groupId }: ExperimentGroupCardProps) {
  const group = useAsyncResource(() => api.experiment.getGroup(groupId), [groupId], () => false)

  return (
    <Card>
      <CardHeader title="我的小组" description="小组共享同一套实验环境与实验进度。" />
      <CardBody>
        <ResourceState
          resource={group}
          emptyIcon={Users}
          emptyTitle="暂无小组信息"
          emptyDescription="老师完成分组后会显示成员与分工。"
          skeleton={<Skeleton variant="line" lines={3} />}
        >
          {(data) => <GroupMembers group={data} />}
        </ResourceState>
      </CardBody>
    </Card>
  )
}

/**
 * GroupMembers 列出小组名称与成员分工。
 * 成员用学号无法从本接口取得(只回 student_id),故按分工呈现 —— 内部编号不进界面。
 */
function GroupMembers({ group }: { group: ExperimentGroup }) {
  return (
    <div className="flex flex-col gap-3">
      <div className="text-base font-medium text-ink">{group.name}</div>
      <div className="flex flex-col gap-2">
        {group.members.map((member, index) => (
          <div key={member.id} className="flex items-center justify-between gap-2">
            <span className="text-sm text-ink-sub">成员 {index + 1}</span>
            <Badge tone="neutral">{member.role || '未分工'}</Badge>
          </div>
        ))}
      </div>
      <p className="text-xs text-ink-faint">共 {group.members.length} 名成员</p>
    </div>
  )
}
