// 学生实验详情页：从后端实验定义展示指导、阶段和检查点，并创建实验实例。

import React, { useState } from 'react'
import type { StudentExperiment } from '@chaimir/api-client'
import { Button } from '@chaimir/ui'
import { FileText, Play } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../experiment.module.css'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const ExperimentDetailPage: React.FC = () => {
  const { id } = useParams()
  const navigate = useNavigate()
  const [starting, setStarting] = useState(false)
  const [message, setMessage] = useState('')
  const resource = useAsyncResource(
    async () => {
      const response = await api.experiment.getPublishedExperiments({ page: 1, size: 100 })
      return response.list.find((item) => item.id === id) ?? null
    },
    [id],
    (value) => value === null
  )

  const startExperiment = async (experiment: StudentExperiment) => {
    setStarting(true)
    setMessage('')
    try {
      const instance = await api.experiment.createInstance(experiment.id, {})
      navigate(`/student/experiments/${experiment.id}/workspace?instanceId=${instance.instance_id}`)
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法创建实验实例，请稍后重试。'))
    } finally {
      setStarting(false)
    }
  }

  if (resource.status === 'loading') {
    return <LoadingState title="正在读取实验详情" description="系统正在同步实验说明和检查点配置。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  if (!resource.data) {
    return <EmptyState title="未找到实验" description="该实验可能已下架或你没有访问权限。" />
  }

  const experiment = resource.data

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>学生端 / 实验实训 / 实验详情</div>

      <div className={styles.header}>
        <h1 className={styles.title}>
          <FileText className={styles.titleIcon} size={28} />
          {experiment.name}
        </h1>
        <Button icon={<Play size={16} />} loading={starting} onClick={() => startExperiment(experiment)}>
          创建实例并进入
        </Button>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <div className={styles.panel}>
        <div className={styles.stats}>
          <div className={styles.stat}>
            <span className={styles.statLabel}>阶段</span>
            <span className={styles.statValue}>{experiment.components.stages.length}</span>
          </div>
          <div className={styles.stat}>
            <span className={styles.statLabel}>检查点</span>
            <span className={styles.statValue}>{experiment.components.checkpoints.length}</span>
          </div>
          <div className={styles.stat}>
            <span className={styles.statLabel}>实验环境</span>
            <span className={styles.statValue}>{experiment.components.envs.length}</span>
          </div>
          <div className={styles.stat}>
            <span className={styles.statLabel}>报告要求</span>
            <span className={styles.statValue}>{experiment.require_report ? '需要提交' : '无需提交'}</span>
          </div>
        </div>
      </div>

      <div className={styles.layout}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>实验说明</h2>
          <p className={styles.muted}>{experiment.description || '教师尚未填写实验说明。'}</p>
          <h2 className={styles.sectionTitle}>阶段安排</h2>
          <ul className={styles.list}>
            {experiment.components.stages.map((stage) => (
              <li className={styles.listItem} key={stage.stage}>
                <strong>{stage.stage}. {stage.title}</strong>
                <p className={styles.muted}>{stage.description || '该阶段没有补充说明。'}</p>
                <div className={styles.itemMeta}>
                  <span>环境 {stage.components.envs?.length ?? 0} 个</span>
                  <span>仿真 {stage.components.sims?.length ?? 0} 个</span>
                </div>
              </li>
            ))}
          </ul>
        </section>
        <aside className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>检查点</h2>
          <ul className={styles.list}>
            {experiment.components.checkpoints.map((checkpoint) => (
              <li className={styles.listItem} key={checkpoint.id}>
                <strong>{checkpoint.id}</strong>
                <div className={styles.itemMeta}>
                  <span>{checkpoint.score} 分</span>
                </div>
              </li>
            ))}
          </ul>
        </aside>
      </div>
    </div>
  )
}

export default ExperimentDetailPage
