// 教师实验列表页：读取后端实验编排，并执行校验、发布、下架和批改入口跳转。

import React, { useState } from 'react'
import type { Experiment } from '@chaimir/api-client'
import { ExperimentStatus } from '@chaimir/api-client'
import { Button, Table } from '@chaimir/ui'
import { FileCheck, LayoutTemplate, Send, SlidersHorizontal } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../experiment.module.css'
import { experimentStatusLabel } from '../../../../../utils/index'

const TeacherExperimentsPage: React.FC = () => {
  const navigate = useNavigate()
  const [message, setMessage] = useState('')
  const resource = useAsyncResource(() => api.experiment.getExperiments({ page: 1, size: 20 }), [])

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
    return <LoadingState title="正在读取实验编排" description="系统正在同步你创建的实验。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  const rows = resource.data?.list ?? []

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 实验实训编排</div>

      <div className={styles.header}>
        <h1 className={styles.title}>
          <LayoutTemplate className={styles.titleIcon} size={28} />
          实验实训编排
        </h1>
        <Button icon={<SlidersHorizontal size={16} />} onClick={() => navigate('/teacher/experiments/orchestration')}>
          新建实验编排
        </Button>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <Table<Experiment>
        rows={rows}
        rowKey="id"
        ariaLabel="教师实验编排"
        emptyTitle="暂无实验编排"
        emptyDescription="新建实验编排后会显示在这里。"
        columns={[
          { key: 'name', title: '实验名称', dataIndex: 'name', priority: 'primary' },
          { key: 'course', title: '课程编号', render: (row) => row.course_id || '未关联', priority: 'secondary' },
          { key: 'envs', title: '环境', render: (row) => `${row.components.envs.length} 个` },
          { key: 'checkpoints', title: '检查点', render: (row) => `${row.components.checkpoints.length} 个` },
          { key: 'status', title: '状态', render: (row) => experimentStatusLabel(row.status) },
          {
            key: 'actions',
            title: '操作',
            render: (row) => (
              <div className={styles.actions}>
                <Button size="sm" variant="outline" icon={<SlidersHorizontal size={15} />} onClick={() => navigate(`/teacher/experiments/orchestration?id=${row.id}`)}>编排</Button>
                <Button size="sm" variant="outline" icon={<FileCheck size={15} />} onClick={() => navigate(`/teacher/experiments/${row.id}/grading`)}>批改</Button>
                <Button size="sm" icon={<Send size={15} />} onClick={() => run(() => api.experiment.validateExperiment(row.id).then((result) => result.ok ? api.experiment.publishExperiment(row.id) : Promise.reject(new Error(result.issues.map((issue) => issue.message).join('；') || '实验配置未通过校验。'))), '实验已发布。')}>发布</Button>
                {row.status === ExperimentStatus.PUBLISHED && (
                  <Button size="sm" variant="ghost" onClick={() => run(() => api.experiment.unpublishExperiment(row.id), '实验已下架。')}>下架</Button>
                )}
              </div>
            ),
          },
        ]}
      />
    </div>
  )
}


export default TeacherExperimentsPage
