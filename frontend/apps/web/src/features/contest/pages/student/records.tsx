// 竞赛战绩页(学生侧栏,/student/records)。
// 数据来自 GET /contest/my/contest-records —— 学生侧唯一的历史战绩来源。
// 后端已过滤取消资格的记录(SQL 内 cheat_record.action=3 的队伍不出现),前端不做二次过滤。
// 「我的战绩」是本页的深页别名(对齐清单 §3.1),不新增侧栏项:同一份数据同一个页面。

import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { ListOrdered, Medal, Swords, Trophy } from 'lucide-react'
import { ContestStatus, type ContestRecord } from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  PageHeader,
  PageScaffold,
  PageSection,
  Stat,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatScore } from '../../../../utils/formatters'
import { contestStatusLabel, contestStatusTone } from '../../../../utils/labels/contest'

/** 已完赛状态:名次已定,可计入最佳成绩统计。 */
const SETTLED_STATUSES: ReadonlySet<ContestStatus> = new Set([
  ContestStatus.ENDED,
  ContestStatus.ARCHIVED,
])

/**
 * StudentRecordsPage 列出历史参赛记录与名次。
 */
export default function StudentRecordsPage() {
  const navigate = useNavigate()
  const records = useAsyncResource(() => api.contest.getMyContestRecords(), [])

  const stats = useMemo(() => {
    const list = records.data ?? []
    const settled = list.filter((item) => SETTLED_STATUSES.has(item.contest_status))
    const ranked = settled.filter((item) => item.rank > 0)
    const bestRank = ranked.length > 0 ? Math.min(...ranked.map((item) => item.rank)) : undefined
    const totalScore = list.reduce((sum, item) => sum + item.score, 0)
    return { count: list.length, settledCount: settled.length, bestRank, totalScore }
  }, [records.data])

  const columns: TableColumn<ContestRecord>[] = [
    {
      key: 'contest_name',
      header: '赛事',
      render: (record) => <span className="font-medium text-ink">{record.contest_name}</span>,
    },
    {
      key: 'contest_status',
      header: '赛事状态',
      render: (record) => (
        <StatusIndicator
          tone={contestStatusTone(record.contest_status)}
          label={contestStatusLabel(record.contest_status)}
        />
      ),
    },
    {
      key: 'score',
      header: '得分',
      align: 'right',
      mono: true,
      render: (record) => formatScore(record.score),
    },
    {
      key: 'rank',
      header: '名次',
      align: 'right',
      mono: true,
      // rank 为 0 表示尚未上榜(SQL 内 COALESCE(l.rank, 0)),不显示「第 0 名」
      render: (record) => (record.rank > 0 ? `第 ${record.rank} 名` : '未上榜'),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (record) => (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => navigate(`/student/contests/${record.contest_id}`)}
        >
          查看赛事
        </Button>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '学习区' }, { label: '竞赛战绩' }]} />}
        title="竞赛战绩"
        description="这里是你参加过的全部赛事、得分与名次。"
        icon={Trophy}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="参赛场次" value={stats.count} icon={Swords} />
          <Stat
            label="最好名次"
            value={stats.bestRank ? `第 ${stats.bestRank} 名` : '—'}
            icon={Medal}
            hint={`已完赛 ${stats.settledCount} 场`}
          />
          <Stat label="累计得分" value={formatScore(stats.totalScore)} icon={ListOrdered} />
        </div>
      </PageSection>

      <PageSection title="参赛记录" description="按赛事结束时间从新到旧排列。">
        <ResourceState
          resource={records}
          emptyIcon={Trophy}
          emptyTitle="还没有参赛记录"
          emptyDescription="报名并参加赛事后,得分与名次会显示在这里。"
          emptyAction={
            <Button variant="primary" onClick={() => navigate('/student/contests')}>
              去看看有哪些赛事
            </Button>
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(data) => (
            <Table columns={columns} data={data} rowKey={(item) => `${item.contest_id}-${item.team_id}`} />
          )}
        </ResourceState>
      </PageSection>
    </PageScaffold>
  )
}
