// 教师竞赛管理页：读取竞赛列表并执行发布、开始、封榜、结束和归档。

import React, { useCallback, useState } from 'react'
import type { Contest, ResultSnapshot } from '@chaimir/api-client'
import { ContestStatus } from '@chaimir/api-client'
import { Button, Modal, Table, useConfirm, ResourceState } from '@chaimir/ui'
import { Archive, Eye, Pause, Play, Settings, Square, Trophy } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useActionFeedback, useAsyncResource } from '../../../../../hooks'
import styles from '../../contest.module.css'
import { formatDateTime, contestStatusLabel } from '../../../../../utils/index'

const TeacherContestsPage: React.FC = () => {
  const confirm = useConfirm()
  const navigate = useNavigate()
  const resource = useAsyncResource(() => api.contest.getContests({ page: 1, size: 30 }), [])
  const { error, message, pendingAction, runAction } = useActionFeedback(resource.reload, '操作没有完成，请稍后重试。')
  const [snapshot, setSnapshot] = useState<ResultSnapshot | null>(null)
  const [snapshotContest, setSnapshotContest] = useState<Contest | null>(null)
  const [snapshotLoading, setSnapshotLoading] = useState(false)
  const [snapshotError, setSnapshotError] = useState<unknown>(null)

  /** runLifecycle 在改变赛事公开性或生命周期前要求教师明确确认。 */
  const runLifecycle = async (key: string, title: string, description: string, operation: () => Promise<unknown>, success: string, danger = false) => {
    const confirmed = await confirm({ title, description, confirmLabel: '确认继续', confirmVariant: danger ? 'danger' : 'primary' })
    if (confirmed) await runAction(key, operation, success)
  }

  /** openSnapshot 读取已归档竞赛的权威最终榜单。 */
  const openSnapshot = useCallback(async (contest: Contest) => {
    setSnapshotContest(contest)
    setSnapshot(null)
    setSnapshotError(null)
    setSnapshotLoading(true)
    try {
      setSnapshot(await api.contest.getResultSnapshot(contest.id))
    } catch (loadError) {
      setSnapshotError(loadError)
    } finally {
      setSnapshotLoading(false)
    }
  }, [])

  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在读取竞赛" description="系统正在同步你管理的竞赛。" />
  }

  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 竞赛管理</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <Trophy className={styles.titleIcon} size={28} />
          竞赛管理
        </h1>
        <Button icon={<Settings size={16} />} onClick={() => navigate('/teacher/contests/config')}>新建竞赛</Button>
      </div>
      {error && <p className={styles.message} role="alert">{error}</p>}
      {message && <p className={styles.message} role="status">{message}</p>}

      <Table<Contest>
        rows={resource.data?.list ?? []}
        rowKey="id"
        ariaLabel="教师竞赛列表"
        emptyTitle="暂无竞赛"
        emptyDescription="创建竞赛后会显示在这里。"
        columns={[
          { key: 'name', title: '竞赛名称', dataIndex: 'name', priority: 'primary' },
          { key: 'time', title: '比赛时间', render: (row) => `${formatDateTime(row.start_at)} 至 ${formatDateTime(row.end_at)}`, priority: 'secondary' },
          { key: 'status', title: '状态', render: (row) => contestStatusLabel(row.status) },
          {
            key: 'actions',
            title: '操作',
            render: (row) => (
              <div className={styles.actions}>
                <Button size="sm" variant="outline" icon={<Settings size={15} />} onClick={() => navigate(`/teacher/contests/${row.id}/config`)}>配置</Button>
                <Button size="sm" variant="outline" onClick={() => navigate(`/teacher/contests/${row.id}/authoring`)}>题目</Button>
                {row.status === ContestStatus.DRAFT && <Button size="sm" icon={<Play size={15} />} loading={pendingAction === `${row.id}-publish`} disabled={Boolean(pendingAction)} onClick={() => runLifecycle(`${row.id}-publish`, '发布竞赛', `发布“${row.name}”后学生即可报名。`, () => api.contest.publishContest(row.id), '竞赛已发布。')}>发布</Button>}
                {row.status === ContestStatus.SIGNUP && <Button size="sm" icon={<Play size={15} />} loading={pendingAction === `${row.id}-start`} disabled={Boolean(pendingAction)} onClick={() => runLifecycle(`${row.id}-start`, '开始竞赛', `开始“${row.name}”后参赛队伍即可提交。`, () => api.contest.startContest(row.id), '竞赛已开始。')}>开始</Button>}
                {row.status === ContestStatus.RUNNING && <Button size="sm" variant="outline" icon={<Pause size={15} />} loading={pendingAction === `${row.id}-freeze`} disabled={Boolean(pendingAction)} onClick={() => runLifecycle(`${row.id}-freeze`, '进入封榜期', '封榜后学生将看不到实时排名变化。', () => api.contest.freezeContest(row.id), '竞赛已进入封榜期。')}>封榜</Button>}
                {(row.status === ContestStatus.RUNNING || row.status === ContestStatus.FROZEN) && <Button size="sm" variant="outline" icon={<Square size={15} />} loading={pendingAction === `${row.id}-end`} disabled={Boolean(pendingAction)} onClick={() => runLifecycle(`${row.id}-end`, '结束竞赛', '结束后将停止接收新提交和新对局。', () => api.contest.endContest(row.id), '竞赛已结束。', true)}>结束</Button>}
                {row.status === ContestStatus.ENDED && <Button size="sm" variant="ghost" icon={<Archive size={15} />} loading={pendingAction === `${row.id}-archive`} disabled={Boolean(pendingAction)} onClick={() => runLifecycle(`${row.id}-archive`, '归档竞赛', '归档会生成最终榜单并回收竞赛关联环境。', () => api.contest.archiveContest(row.id), '竞赛已归档。', true)}>归档</Button>}
                {row.status === ContestStatus.ARCHIVED && <Button size="sm" variant="outline" icon={<Eye size={15} />} onClick={() => void openSnapshot(row)}>最终榜单</Button>}
              </div>
            ),
          },
        ]}
      />
      <Modal open={snapshotContest !== null} title={snapshotContest ? `${snapshotContest.name} · 最终榜单` : '最终榜单'} size="lg" onClose={() => setSnapshotContest(null)}>
        {snapshotLoading && <ResourceState status="loading" title="正在读取最终榜单" />}
        {snapshotError !== null && <ResourceState status="error" error={snapshotError} onRetry={() => snapshotContest && void openSnapshot(snapshotContest)} />}
        {snapshot && (
          <Table
            rows={snapshot.final_ranking}
            rowKey="team_id"
            ariaLabel="竞赛最终榜单"
            emptyTitle="暂无最终排名"
            emptyDescription="这场竞赛归档时没有可计入排名的队伍。"
            columns={[
              { key: 'rank', title: '名次', dataIndex: 'rank', priority: 'primary' },
              { key: 'team', title: '队伍', dataIndex: 'team_name' },
              { key: 'score', title: '得分', dataIndex: 'score' },
              { key: 'solved', title: '通过题数', dataIndex: 'solved_count' },
              { key: 'updated', title: '最后更新', render: (row) => formatDateTime(row.updated_at), priority: 'secondary' },
            ]}
          />
        )}
      </Modal>
    </div>
  )
}



export default TeacherContestsPage
