// 竞赛战绩页(学生侧栏,/student/records)。
// 数据来自 GET /contest/my/contest-records —— 学生侧唯一的历史战绩来源。
// 后端已过滤取消资格的记录(SQL 内 cheat_record.action=3 的队伍不出现),前端不做二次过滤。
// 「我的战绩」是本页的深页别名(对齐清单 §3.1),不新增侧栏项:同一份数据同一个页面。

import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { Trophy } from 'lucide-react'
import { ContestStatus, type ContestRecord } from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  DataPanel,
  MetricStrip,
  PageHeader,
  PageScaffold,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatScore } from '../../../../utils/formatters'
import { contestStatusLabel } from '../../../../utils/labels/contest'
import { contestStatusTone } from '../../statusPresentation'

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
        kicker={<Breadcrumb items={[{ label: '学习区' }]} />}
        title="竞赛战绩"
        description="这里是你参加过的全部赛事、得分与名次。"
        icon={Trophy}
      />

      {/*
        指标降为内联摘要(§6.5.3 第 ① 族)。这四项由本人全部战绩算出而不是页面切片:
        GET /contest/my/contest-records 不分页、一次回齐,故客户端聚合就是全量口径(§6.5.4)。
      */}
      <MetricStrip
        label="战绩摘要"
        className="mb-5"
        items={[
          { label: '参赛场次', value: stats.count, hint: '含进行中的赛事' },
          {
            label: '最好名次',
            value: stats.bestRank ? `第 ${stats.bestRank} 名` : '—',
            hint: `已完赛 ${stats.settledCount} 场`,
          },
          { label: '累计得分', value: formatScore(stats.totalScore), hint: '全部赛事求和' },
        ]}
      />

      {/* 数据表独占一块抬起片(§6.5.2)。本页不分页:接口一次回齐全部战绩,
          也没有筛选项 —— 场次数量本就不多,再排一条筛选井是多余的一层。 */}
      <DataPanel label="参赛记录">
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
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(data) => (
            <Table
              columns={columns}
              data={data}
              rowKey={(item) => `${item.contest_id}-${item.team_id}`}
              elevated={false}
              onRowClick={(item) => navigate(`/student/contests/${item.contest_id}`)}
              // <md 换行卡(§6.4.1 规则 3):赛事名一行、得分与名次一行,赛事状态在右
              mobileCard={(item) => ({
                title: item.contest_name,
                meta: `得分 ${formatScore(item.score)} · ${item.rank > 0 ? `第 ${item.rank} 名` : '未上榜'}`,
                badge: (
                  <StatusIndicator
                    tone={contestStatusTone(item.contest_status)}
                    label={contestStatusLabel(item.contest_status)}
                  />
                ),
              })}
            />
          )}
        </ResourceState>
      </DataPanel>
    </PageScaffold>
  )
}
