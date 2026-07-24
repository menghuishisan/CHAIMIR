// DashboardStatCard 统一展示平台与学校管理看板的单项数字指标。

import React from 'react'
import { formatNumber } from '../../../utils'
import styles from '../pages/dashboard.module.css'

/** DashboardStatCard 渲染带可选警示语义的看板指标。 */
export function DashboardStatCard({ label, value, danger = false }: { label: string; value?: number; danger?: boolean }): React.ReactElement {
  return (
    <div className={styles.statCard}>
      <div className={styles.statLabel}>{label}</div>
      <div className={`${styles.statValue} ${danger ? styles.statDanger : ''}`}>{formatNumber(value)}</div>
    </div>
  )
}
