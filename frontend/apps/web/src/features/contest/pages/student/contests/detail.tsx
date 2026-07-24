// 学生竞赛详情页：展示后端竞赛规则、题目和排行榜。

import React, { useCallback, useMemo } from 'react'
import type { Contest, ContestProblem, LadderRank } from '@chaimir/api-client'
import { Button, Table, ResourceState } from '@chaimir/ui'
import { FileText, Play, UserPlus } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource, useTicketedWebSocket } from '../../../../../hooks'
import styles from '../../contest.module.css'
import { formatDateTime } from '../../../../../utils/index'

interface DetailData {
  contest: Contest | null
  problems: ContestProblem[]
  ladder: LadderRank[]
  leaderboardTopic?: string
}

const StudentContestDetailPage: React.FC = () => {
  const { id } = useParams()
  const navigate = useNavigate()
  const resource = useAsyncResource<DetailData>(
    async () => {
      if (!id) throw new Error('缺少竞赛编号，无法读取竞赛详情。')
      const [contest, problems, ladder, me] = await Promise.all([
        api.contest.getStudentContest(id),
        api.contest.getProblems(id),
        api.contest.getLadder(id, { page: 1, size: 10 }),
        api.identity.getMe(),
      ])
      if (!me.account.tenant_id) {
        throw new Error('当前账号未关联学校，暂时无法订阅竞赛排行榜。')
      }
      return {
        contest,
        problems,
        ladder: ladder.list,
        leaderboardTopic: api.contest.getLeaderboardTopic(me.account.tenant_id, id),
      }
    },
    [id],
    (value) => !value.contest
  )
  const subscription = useMemo(() => resource.data?.leaderboardTopic ? { action: 'subscribe', topics: [resource.data.leaderboardTopic] } : undefined, [resource.data?.leaderboardTopic])
  const handleLeaderboard = useCallback(() => resource.reload(), [resource])
  const realtime = useTicketedWebSocket({ url: subscription ? api.eventWebSocketUrl() : null, subscribeMessage: subscription, onMessage: handleLeaderboard })

  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在读取竞赛详情" description="系统正在同步赛程、题目和排行榜。" />
  }

  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  }

  if (!resource.data?.contest) {
    return <ResourceState status="empty" title="未找到竞赛" description="该竞赛可能已下架或你没有访问权限。" />
  }

  const { contest, problems, ladder } = resource.data

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>学生端 / 竞赛中心 / 竞赛详情</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <FileText className={styles.titleIcon} size={28} />
          {contest.name}
        </h1>
        <div className={styles.actions}>
          <Button variant="outline" icon={<UserPlus size={16} />} onClick={() => navigate(`/student/contests/${contest.id}/apply`)}>报名</Button>
          <Button icon={<Play size={16} />} onClick={() => navigate(`/student/contests/${contest.id}/workspace`)}>进入赛场</Button>
        </div>
      </div>

      <div className={styles.panel}>
        <div className={styles.stats}>
          <div className={styles.stat}><span className={styles.statLabel}>题目数</span><span className={styles.statValue}>{problems.length}</span></div>
          <div className={styles.stat}><span className={styles.statLabel}>封榜时长</span><span className={styles.statValue}>{contest.freeze_minutes} 分钟</span></div>
          <div className={styles.stat}><span className={styles.statLabel}>比赛开始</span><span className={styles.statValue}>{formatDateTime(contest.start_at)}</span></div>
          <div className={styles.stat}><span className={styles.statLabel}>排行榜同步</span><span className={styles.statValue}>{realtime.status === 'open' ? '实时' : '连接中'}</span></div>
        </div>
      </div>

      <div className={styles.split}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>竞赛题目</h2>
          <Table<ContestProblem>
            rows={problems}
            rowKey="id"
            ariaLabel="竞赛题目"
            emptyTitle="暂无题目"
            emptyDescription="教师配置题目后会显示在这里。"
            columns={[
              { key: 'seq', title: '序号', dataIndex: 'seq' },
              { key: 'item', title: '题目', render: (row) => row.title, priority: 'primary' },
              { key: 'version', title: '版本', dataIndex: 'item_version' },
              { key: 'score', title: '分值', dataIndex: 'score' },
            ]}
          />
        </section>
        <aside className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>排行榜</h2>
          <ul className={styles.list}>
            {ladder.map((rank) => (
              <li className={styles.listItem} key={rank.team_id}>
                <strong>第 {rank.rank} 名 · {rank.team_name}</strong>
                <p className={styles.muted}>{rank.score} 分，解出 {rank.solved_count} 题</p>
              </li>
            ))}
          </ul>
        </aside>
      </div>
    </div>
  )
}

export default StudentContestDetailPage
