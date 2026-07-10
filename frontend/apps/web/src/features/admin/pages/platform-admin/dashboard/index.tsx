// PlatformDashboardPage 展示平台管理员的大盘指标，数据来自 admin 后端模块。

import React from 'react'
import type { Dashboard } from '@chaimir/api-client'
import { Button } from '@chaimir/ui'
import { BarChart3, LayoutDashboard, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../dashboard.module.css'
import { formatDateTime, formatNumber } from '../../../../../utils/index'



/**
 * StatCard 渲染平台大盘指标。
 */
const StatCard: React.FC<{ label: string; value?: number; danger?: boolean }> = ({
  label,
  value,
  danger = false,
}) => (
  <div className={styles.statCard}>
    <div className={styles.statLabel}>{label}</div>
    <div className={`${styles.statValue} ${danger ? styles.statDanger : ''}`}>
      {formatNumber(value)}
    </div>
  </div>
)

/**
 * PlatformDashboardPage 读取平台管理概览。
 */
const PlatformDashboardPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource<Dashboard>(() => api.admin.getPlatformDashboard(), [])
  const dashboard = resource.data

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <LayoutDashboard className={styles.icon} size={28} />
          平台宏观大盘
        </h1>
        <div className={styles.headerActions}>
          <Button variant="outline" icon={<BarChart3 size={16} />} onClick={() => navigate('/platform-admin/dashboard/statistics')}>查看统计</Button>
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
        </div>
      </div>

      {resource.status === 'loading' && <LoadingState title="正在获取平台概览" />}
      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'empty' && (
        <EmptyState title="暂无概览" description="当前平台还没有可展示的管理数据。" />
      )}
      {resource.status === 'success' && dashboard && (
        <>
          <div className={styles.updatedAt}>统计时间 {formatDateTime(dashboard.generated_at)}</div>
          <div className={styles.statsGrid}>
            <StatCard label="入驻租户" value={dashboard.tenant_count} />
            <StatCard label="活跃账号" value={dashboard.active_account_count} />
            <StatCard label="运行中沙箱" value={dashboard.active_sandbox_count} />
            <StatCard label="待处理申请" value={dashboard.pending_apply_count} danger />
          </div>

          <section className={styles.panel}>
            <h2 className={styles.panelTitle}>平台业务规模</h2>
            <div className={styles.metricsList}>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>账号总数</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.account_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>教师账号</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.teacher_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>学生账号</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.student_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>课程总数</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.course_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>实验总数</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.experiment_count)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>竞赛总数</span>
                <span className={styles.metricValue}>{formatNumber(dashboard.contest_count)}</span>
              </div>
            </div>
          </section>
        </>
      )}
    </div>
  )
}

export default PlatformDashboardPage
