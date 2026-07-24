// 学生竞赛回放页：读取对局列表并按后端回放引用展示可追溯信息。

import React, { useState } from 'react'
import type { BattleMatch, BattleReplay } from '@chaimir/api-client'
import { Button, Table, ResourceState } from '@chaimir/ui'
import { CheckCircle, Clapperboard, XCircle } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { usePendingAction } from '../../../../../hooks'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { battleMatchStatusLabel, battleResultLabel, formatDateTime } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const StudentContestReplayPage: React.FC = () => {
  const { id } = useParams()
  const [replay, setReplay] = useState<BattleReplay>()
  const [message, setMessage] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()
  const resource = useAsyncResource(
    async () => {
      if (!id) throw new Error('缺少竞赛编号，无法读取对局。')
      return api.contest.listBattleMatches(id, { page: 1, size: 20 })
    },
    [id]
  )

  const loadReplay = async (target: string) => {
    setMessage('')
    try {
      const result = await api.contest.getBattleReplay(target.trim())
      setReplay(result)
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法读取回放。'))
    }
  }

  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在读取对局回放" description="系统正在同步当前队伍的对局记录。" />
  }

  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
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
              { key: 'problem', title: '题目', render: () => '对抗题目', priority: 'primary' },
              { key: 'status', title: '状态', render: (row) => battleMatchStatusLabel(row.status) },
              { key: 'matched', title: '匹配时间', render: (row) => formatDateTime(row.matched_at) },
              { key: 'action', title: '操作', render: (row) => row.replay_available ? <Button size="sm" variant="outline" loading={pendingAction === `replay-${row.id}`} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction(`replay-${row.id}`, () => loadReplay(row.id))}>查看回放</Button> : <span className={styles.muted}>尚未生成</span> },
            ]}
          />
        </section>

        <aside className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>{replay?.problem_title || '回放时间轴'}</h2>
          {replay ? (
            <>
              <p className={styles.muted}>{battleResultLabel(replay.result)} · {formatDateTime(replay.finished_at)}</p>
              <ol className={styles.list}>
                {replay.steps.map((step) => (
                  <li className={styles.listItem} key={step.seq}>
                    <strong>{step.passed ? <CheckCircle size={16} /> : <XCircle size={16} />} 第 {step.seq} 步 · {step.title}</strong>
                    {(step.source || step.target) && <p className={styles.muted}>{[step.source, step.target].filter(Boolean).join(' → ')}</p>}
                    {step.actual && <p>{step.actual}</p>}
                    {step.hint && <p className={styles.muted}>{step.hint}</p>}
                  </li>
                ))}
              </ol>
            </>
          ) : <p className={styles.muted}>从对局列表中选择一场已完成的比赛查看真实步骤。</p>}
        </aside>
      </div>
    </div>
  )
}

export default StudentContestReplayPage
