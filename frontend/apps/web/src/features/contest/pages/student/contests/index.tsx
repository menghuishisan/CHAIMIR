// 学生竞赛列表页：读取后端竞赛列表并提供报名、详情和参赛入口。

import React from 'react'
import type { Contest } from '@chaimir/api-client'
import { Button, Table } from '@chaimir/ui'
import { Eye, Swords, Trophy, UserPlus } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { formatDateTime, contestStatusLabel } from '../../../../../utils/index'

const StudentContestsPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(() => api.contest.getContests({ page: 1, size: 20 }), [])

  if (resource.status === 'loading') {
    return <LoadingState title="正在读取竞赛" description="系统正在同步可报名和可参赛的竞赛。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>学生端 / 竞赛中心</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <Trophy className={styles.titleIcon} size={28} />
          竞赛中心
        </h1>
      </div>

      <Table<Contest>
        rows={resource.data?.list ?? []}
        rowKey="id"
        ariaLabel="竞赛列表"
        emptyTitle="暂无竞赛"
        emptyDescription="竞赛发布后会显示在这里。"
        columns={[
          { key: 'name', title: '竞赛名称', dataIndex: 'name', priority: 'primary' },
          { key: 'signup', title: '报名时间', render: (row) => `${formatDateTime(row.signup_start)} 至 ${formatDateTime(row.signup_end)}`, priority: 'secondary' },
          { key: 'match', title: '比赛时间', render: (row) => `${formatDateTime(row.start_at)} 至 ${formatDateTime(row.end_at)}` },
          { key: 'status', title: '状态', render: (row) => contestStatusLabel(row.status, '未发布') },
          {
            key: 'actions',
            title: '操作',
            render: (row) => (
              <div className={styles.actions}>
                <Button size="sm" variant="outline" icon={<Eye size={15} />} onClick={() => navigate(`/student/contests/${row.id}`)}>详情</Button>
                <Button size="sm" variant="outline" icon={<UserPlus size={15} />} onClick={() => navigate(`/student/contests/${row.id}/apply`)}>报名</Button>
                <Button size="sm" icon={<Swords size={15} />} onClick={() => navigate(`/student/contests/${row.id}/workspace`)}>进入</Button>
              </div>
            ),
          },
        ]}
      />
    </div>
  )
}



export default StudentContestsPage
