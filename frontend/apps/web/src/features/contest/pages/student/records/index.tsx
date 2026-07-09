// 学生竞赛战绩页：读取当前账号的后端竞赛记录。

import React from 'react'
import type { ContestRecord } from '@chaimir/api-client'
import { Table } from '@chaimir/ui'
import { Medal } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { contestStatusLabel } from '../../../../../utils/index'

const StudentRecordsPage: React.FC = () => {
  const resource = useAsyncResource(() => api.contest.getMyContestRecords(), [])

  if (resource.status === 'loading') {
    return <LoadingState title="正在读取竞赛战绩" description="系统正在同步你的竞赛记录。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>学生端 / 竞赛战绩</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <Medal className={styles.titleIcon} size={28} />
          竞赛战绩
        </h1>
      </div>
      <Table<ContestRecord>
        rows={resource.data ?? []}
        rowKey={(row) => `${row.contest_id}-${row.team_id}`}
        ariaLabel="竞赛战绩"
        emptyTitle="暂无战绩"
        emptyDescription="参加竞赛并产生榜单后会显示在这里。"
        columns={[
          { key: 'contest', title: '竞赛', dataIndex: 'contest_name', priority: 'primary' },
          { key: 'score', title: '得分', dataIndex: 'score' },
          { key: 'rank', title: '排名', render: (row) => `第 ${row.rank} 名` },
          { key: 'status', title: '状态', render: (row) => contestStatusLabel(row.contest_status, '未完成') },
        ]}
      />
    </div>
  )
}


export default StudentRecordsPage
