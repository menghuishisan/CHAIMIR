// AdminDashboardPage 展示学校管理员的大盘指标，数据来自 admin 后端模块。

import React from 'react'
import type { Dashboard } from '@chaimir/api-client'
import { Button, ResourceState } from '@chaimir/ui'
import { LayoutDashboard, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../dashboard.module.css'
import { formatDateTime, formatNumber } from '../../../../../utils/index'
import { DashboardStatCard } from '../../../components/DashboardStatCard'

/**
 * AdminDashboardPage 读取本校管理概览。
 */
const AdminDashboardPage: React.FC = () => {
  const resource = useAsyncResource<Dashboard>(() => api.admin.getSchoolDashboard(), [])
  const dashboard = resource.data

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <LayoutDashboard className={styles.icon} size={28} />
          本校数字资源概览
        </h1>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取本校概览" />}
      {resource.status === 'error' && (
        <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'empty' && (
        <ResourceState status="empty" title="暂无概览" description="当前学校还没有可展示的管理数据。" />
      )}
      {resource.status === 'success' && dashboard && (
        <>
          <div className={styles.updatedAt}>统计时间 {formatDateTime(dashboard.generated_at)}</div>
          <div className={styles.statsGrid}>
            <DashboardStatCard label="学生账号" value={dashboard.student_count} />
            <DashboardStatCard label="教师账号" value={dashboard.teacher_count} />
            <DashboardStatCard label="活跃课程" value={dashboard.active_course_count} />
            <DashboardStatCard label="待处理申请" value={dashboard.pending_apply_count} danger />
          </div>

          <section className={styles.panel}>
            <h2 className={styles.panelTitle}>教学与实验资源</h2>
            <div className={styles.metricsList}>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>课程总数</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.course_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>实验总数</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.experiment_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>运行中实验实例</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.active_instance_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>运行中沙箱</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.active_sandbox_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>竞赛总数</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.contest_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>进行中竞赛</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.active_contest_count)}</span>
              </div>
            </div>
          </section>
        </>
      )}
    </div>
  )
}

export default AdminDashboardPage
