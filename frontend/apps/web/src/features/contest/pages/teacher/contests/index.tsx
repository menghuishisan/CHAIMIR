// 教师竞赛管理页：读取竞赛列表并执行发布、开始、封榜、结束和归档。

import React, { useState } from 'react'
import type { Contest } from '@chaimir/api-client'
import { ContestStatus } from '@chaimir/api-client'
import { Button, Table } from '@chaimir/ui'
import { Archive, Pause, Play, Settings, Square, Trophy } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { formatDateTime, contestStatusLabel } from '../../../../../utils/index'

const TeacherContestsPage: React.FC = () => {
  const navigate = useNavigate()
  const [message, setMessage] = useState('')
  const resource = useAsyncResource(() => api.contest.getContests({ page: 1, size: 30 }), [])

  const run = async (operation: () => Promise<unknown>, success: string) => {
    setMessage('')
    try {
      await operation()
      setMessage(success)
      resource.reload()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '操作没有完成，请稍后重试。')
    }
  }

  if (resource.status === 'loading') {
    return <LoadingState title="正在读取竞赛" description="系统正在同步你管理的竞赛。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 竞赛管理</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <Trophy className={styles.titleIcon} size={28} />
          竞赛管理
        </h1>
        <Button icon={<Settings size={16} />} onClick={() => navigate('/teacher/contests/config')}>新建竞赛</Button>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <Table<Contest>
        rows={resource.data?.list ?? []}
        rowKey="id"
        ariaLabel="教师竞赛列表"
        emptyTitle="暂无竞赛"
        emptyDescription="创建竞赛后会显示在这里。"
        columns={[
          { key: 'name', title: '竞赛名称', dataIndex: 'name', priority: 'primary' },
          { key: 'time', title: '比赛时间', render: (row) => `${formatDateTime(row.start_at)} 至 ${formatDateTime(row.end_at)}`, priority: 'secondary' },
          { key: 'status', title: '状态', render: (row) => contestStatusLabel(row.status) },
          {
            key: 'actions',
            title: '操作',
            render: (row) => (
              <div className={styles.actions}>
                <Button size="sm" variant="outline" icon={<Settings size={15} />} onClick={() => navigate(`/teacher/contests/${row.id}/config`)}>配置</Button>
                <Button size="sm" variant="outline" onClick={() => navigate(`/teacher/contests/${row.id}/authoring`)}>题目</Button>
                <Button size="sm" icon={<Play size={15} />} onClick={() => run(() => row.status === ContestStatus.DRAFT ? api.contest.publishContest(row.id) : api.contest.startContest(row.id), row.status === ContestStatus.DRAFT ? '竞赛已发布。' : '竞赛已开始。')}>{row.status === ContestStatus.DRAFT ? '发布' : '开始'}</Button>
                <Button size="sm" variant="outline" icon={<Pause size={15} />} onClick={() => run(() => api.contest.freezeContest(row.id), '竞赛已进入封榜期。')}>封榜</Button>
                <Button size="sm" variant="outline" icon={<Square size={15} />} onClick={() => run(() => api.contest.endContest(row.id), '竞赛已结束。')}>结束</Button>
                <Button size="sm" variant="ghost" icon={<Archive size={15} />} onClick={() => run(() => api.contest.archiveContest(row.id), '竞赛已归档。')}>归档</Button>
              </div>
            ),
          },
        ]}
      />
    </div>
  )
}



export default TeacherContestsPage
