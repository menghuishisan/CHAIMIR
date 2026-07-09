// 学生实验列表页：读取后端已发布实验，并进入详情或沉浸式实验工作台。

import React from 'react'
import type { Experiment } from '@chaimir/api-client'
import { ExperimentStatus } from '@chaimir/api-client'
import { Button, Table } from '@chaimir/ui'
import { ExternalLink, FileText, FlaskConical } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../experiment.module.css'

const ExperimentsPage: React.FC = () => {
  const navigate = useNavigate()
  const experiments = useAsyncResource(
    () => api.experiment.getExperiments({ status: ExperimentStatus.PUBLISHED, page: 1, size: 20 }),
    []
  )

  if (experiments.status === 'loading') {
    return <LoadingState title="正在读取实验" description="系统正在同步可进入的实验列表。" />
  }

  if (experiments.status === 'error') {
    return <ErrorState error={experiments.error} onRetry={experiments.reload} />
  }

  const rows = experiments.data?.list ?? []

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>学生端 / 实验实训</div>

      <div className={styles.header}>
        <h1 className={styles.title}>
          <FlaskConical className={styles.titleIcon} size={28} />
          实验实训
        </h1>
      </div>

      <Table<Experiment>
        rows={rows}
        rowKey="id"
        ariaLabel="可进入实验"
        emptyTitle="暂无可进入的实验"
        emptyDescription="教师发布实验后会显示在这里。"
        columns={[
          { key: 'name', title: '实验名称', dataIndex: 'name', priority: 'primary' },
          { key: 'course', title: '课程编号', render: (row) => row.course_id || '未关联', priority: 'secondary' },
          { key: 'envs', title: '环境', render: (row) => `${row.components.envs.length} 个环境`, priority: 'secondary' },
          { key: 'checkpoints', title: '检查点', render: (row) => `${row.components.checkpoints.length} 个` },
          {
            key: 'actions',
            title: '操作',
            render: (row) => (
              <div className={styles.actions}>
                <Button size="sm" icon={<ExternalLink size={15} />} onClick={() => navigate(`/student/experiments/${row.id}/workspace`)}>
                  进入工作台
                </Button>
                <Button size="sm" variant="outline" icon={<FileText size={15} />} onClick={() => navigate(`/student/experiments/${row.id}`)}>
                  查看详情
                </Button>
              </div>
            ),
          },
        ]}
      />
    </div>
  )
}

export default ExperimentsPage
