// 学生竞赛画像页：基于后端战绩记录汇总个人竞赛表现。

import React, { useMemo } from 'react'
import { Medal, TrendingUp } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'

const StudentRecordProfilePage: React.FC = () => {
  const resource = useAsyncResource(() => api.contest.getMyContestRecords(), [])
  const summary = useMemo(() => {
    const records = resource.data ?? []
    const bestRank = records.length ? Math.min(...records.map((record) => record.rank || Number.MAX_SAFE_INTEGER)) : 0
    const totalScore = records.reduce((sum, record) => sum + record.score, 0)
    return { total: records.length, bestRank, totalScore }
  }, [resource.data])

  if (resource.status === 'loading') {
    return <LoadingState title="正在读取竞赛画像" description="系统正在汇总你的竞赛表现。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>学生端 / 竞赛战绩 / 个人画像</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <TrendingUp className={styles.titleIcon} size={28} />
          竞赛画像
        </h1>
      </div>
      <div className={styles.panel}>
        <div className={styles.stats}>
          <div className={styles.stat}><span className={styles.statLabel}>参赛次数</span><span className={styles.statValue}>{summary.total}</span></div>
          <div className={styles.stat}><span className={styles.statLabel}>最佳排名</span><span className={styles.statValue}>{summary.bestRank ? `第 ${summary.bestRank} 名` : '暂无'}</span></div>
          <div className={styles.stat}><span className={styles.statLabel}>累计得分</span><span className={styles.statValue}>{summary.totalScore}</span></div>
        </div>
      </div>
      <section className={`${styles.panel} ${styles.section}`}>
        <h2 className={styles.sectionTitle}><Medal size={18} /> 最近记录</h2>
        <ul className={styles.list}>
          {(resource.data ?? []).map((record) => (
            <li className={styles.listItem} key={`${record.contest_id}-${record.team_id}`}>
              <strong>{record.contest_name}</strong>
              <p className={styles.muted}>排名第 {record.rank}，得分 {record.score}</p>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}

export default StudentRecordProfilePage
