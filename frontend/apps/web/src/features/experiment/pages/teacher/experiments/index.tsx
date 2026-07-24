// 教师实验列表页：读取后端实验编排，并执行校验、发布、下架和批改入口跳转。

import React from 'react'
import type { Experiment } from '@chaimir/api-client'
import { ExperimentStatus } from '@chaimir/api-client'
import { Button, Table, ResourceState } from '@chaimir/ui'
import { FileCheck, LayoutTemplate, Send, SlidersHorizontal } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useActionFeedback, useAsyncResource } from '../../../../../hooks'
import styles from '../../experiment.module.css'
import { experimentStatusLabel } from '../../../../../utils/index'

const TeacherExperimentsPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(async () => {
    const [items, courses] = await Promise.all([
      api.experiment.getExperiments({ page: 1, size: 20 }),
      api.teaching.getCourses({ role: 'teacher', page: 1, size: 100 }),
    ])
    return { items, courseNames: new Map(courses.list.map((course) => [course.id, course.name])) }
  }, [])
  const { error, message, pendingAction, runAction } = useActionFeedback(resource.reload, '操作没有完成，请稍后重试。')

  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在读取实验编排" description="系统正在同步你创建的实验。" />
  }

  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  }

  const rows = resource.data?.items.list ?? []

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
      {error && <p className={styles.message} role="alert">{error}</p>}
      {message && <p className={styles.message} role="status">{message}</p>}

      <Table<Experiment>
        rows={rows}
        rowKey="id"
        ariaLabel="教师实验编排"
        emptyTitle="暂无实验编排"
        emptyDescription="新建实验编排后会显示在这里。"
        columns={[
          { key: 'name', title: '实验名称', dataIndex: 'name', priority: 'primary' },
          { key: 'course', title: '所属课程', render: (row) => row.course_id ? resource.data?.courseNames.get(row.course_id) || '课程不可用' : '独立实验', priority: 'secondary' },
          { key: 'envs', title: '环境', render: (row) => `${row.components.envs.length} 个` },
          { key: 'checkpoints', title: '检查点', render: (row) => `${row.components.checkpoints.length} 个` },
          { key: 'status', title: '状态', render: (row) => experimentStatusLabel(row.status) },
          {
            key: 'actions',
            title: '操作',
            render: (row) => (
              <div className={styles.actions}>
                <Button size="sm" variant="outline" icon={<SlidersHorizontal size={15} />} onClick={() => navigate(`/teacher/experiments/orchestration?id=${row.id}`)}>编排</Button>
          <Button size="sm" variant="outline" icon={<FileCheck size={15} />} onClick={() => navigate(`/teacher/experiments/${row.id}/grading`)}>查看提交</Button>
                <Button size="sm" icon={<Send size={15} />} loading={pendingAction === `${row.id}-publish`} disabled={Boolean(pendingAction)} onClick={() => runAction(`${row.id}-publish`, () => api.experiment.validateExperiment(row.id).then((result) => result.ok ? api.experiment.publishExperiment(row.id) : Promise.reject(new Error(result.issues.map((issue) => issue.message).join('；') || '实验配置未通过校验。'))), '实验已发布。')}>发布</Button>
                {row.status === ExperimentStatus.PUBLISHED && (
                  <Button size="sm" variant="ghost" loading={pendingAction === `${row.id}-unpublish`} disabled={Boolean(pendingAction)} onClick={() => runAction(`${row.id}-unpublish`, () => api.experiment.unpublishExperiment(row.id), '实验已下架。')}>下架</Button>
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
