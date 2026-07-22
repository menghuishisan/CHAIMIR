// 学生竞赛回放页：读取对局列表并按后端回放引用展示可追溯信息。

import React, { useState } from 'react'
import type { BattleMatch } from '@chaimir/api-client'
import { Button, Input, Table } from '@chaimir/ui'
import { Clapperboard, Search } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { formatDateTime } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const StudentContestReplayPage: React.FC = () => {
  const { id } = useParams()
  const [matchId, setMatchId] = useState('')
  const [replayRef, setReplayRef] = useState('')
  const [message, setMessage] = useState('')
  const resource = useAsyncResource(
    async () => {
      if (!id) throw new Error('缺少竞赛编号，无法读取对局。')
      return api.contest.listBattleMatches(id, { page: 1, size: 20 })
    },
    [id]
  )

  const loadReplay = async (target = matchId) => {
    if (!target.trim()) return
    setMessage('')
    try {
      const result = await api.contest.getBattleReplay(target.trim())
      setReplayRef(result.replay_ref)
      setMatchId(result.match_id)
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法读取回放。'))
    }
  }

  if (resource.status === 'loading') {
    return <LoadingState title="正在读取对局回放" description="系统正在同步当前队伍的对局记录。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>学生端 / 竞赛中心 / 对局回放</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <Clapperboard className={styles.titleIcon} size={28} />
          对局回放
        </h1>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <div className={styles.split}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>对局列表</h2>
          <Table<BattleMatch>
            rows={resource.data?.list ?? []}
            rowKey="id"
            ariaLabel="对局列表"
            emptyTitle="暂无对局"
            emptyDescription="对抗赛产生对局后会显示在这里。"
            columns={[
              { key: 'problem', title: '题目', dataIndex: 'problem_id', priority: 'primary' },
              { key: 'status', title: '状态', dataIndex: 'status' },
              { key: 'matched', title: '匹配时间', render: (row) => formatDateTime(row.matched_at) },
              { key: 'action', title: '操作', render: (row) => <Button size="sm" variant="outline" onClick={() => loadReplay(row.id)}>读取回放</Button> },
            ]}
          />
        </section>

        <aside className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>回放引用</h2>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="match-id">对局编号</label>
            <Input id="match-id" value={matchId} onChange={(event) => setMatchId(event.target.value)} fullWidth />
          </div>
          <Button icon={<Search size={16} />} onClick={() => loadReplay()}>读取回放</Button>
          {replayRef ? <p className={styles.muted}>回放记录已准备：{replayRef}</p> : <p className={styles.muted}>从对局列表中选择一场比赛读取回放。</p>}
        </aside>
      </div>
    </div>
  )
}

export default StudentContestReplayPage
