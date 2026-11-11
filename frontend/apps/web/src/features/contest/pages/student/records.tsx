// 竞赛战绩页(学生侧栏,/student/records)。设计 §7.2:
// 页头下一行摘要(参赛场次、最好名次、累计得分 —— 这三个数就是这一页要看的结论);
// 下方每场一行(赛事名、状态、名次、得分、解题数、最后解题时间),整行链向赛事详情。
// 没有战绩时给「去看看有哪些赛事」。
//
// 数据来自 GET /contest/my/contest-records —— 学生侧唯一的历史战绩来源。
// 后端已过滤取消资格的记录(cheat_record.action=3 的队伍不出现),前端不做二次过滤。

import { useMemo } from 'react'
import { Link, useNavigate } from 'react-router'
import { Trophy } from 'lucide-react'
import { ContestStatus, type ContestRecord } from '@chaimir/api-client'
import {
  buttonVariants,
  Icon,
  MetricStrip,
  PageHeader,
  PageScaffold,
  Skeleton,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence, formatScore } from '../../../../utils/formatters'
import { contestStatusLabel } from '../../../../utils/labels/contest'
import { contestStatusTone } from '../../statusPresentation'

/** 已完赛状态:名次已定,可计入最好名次 */
const SETTLED_STATUSES: ReadonlySet<ContestStatus> = new Set([ContestStatus.ENDED, ContestStatus.ARCHIVED])

/**
 * StudentRecordsPage 列出历史参赛记录与名次。
 */
export default function StudentRecordsPage() {
  const navigate = useNavigate()
  const records = useAsyncResource(() => api.contest.getMyContestRecords(), [])

  const stats = useMemo(() => {
    const list = records.data ?? []
    const ranked = list.filter((item) => SETTLED_STATUSES.has(item.contest_status) && item.rank > 0)
    return {
      count: list.length,
      bestRank: ranked.length > 0 ? Math.min(...ranked.map((item) => item.rank)) : undefined,
      totalScore: list.reduce((sum, item) => sum + item.score, 0),
    }
  }, [records.data])

  // 进行中的在前,其余按最后解题时间新的在前
  const sorted = useMemo(
    () =>
      [...(records.data ?? [])].sort((a, b) => {
        const live = Number(!SETTLED_STATUSES.has(b.contest_status)) - Number(!SETTLED_STATUSES.has(a.contest_status))
        if (live !== 0) return live
        return (b.last_solve_at ?? '').localeCompare(a.last_solve_at ?? '')
      }),
    [records.data],
  )

  const columns: TableColumn<ContestRecord>[] = [
    {
      key: 'contest_name',
      header: '赛事',
      render: (record) => <span className="font-medium text-ink">{record.contest_name}</span>,
    },
    {
      key: 'contest_status',
      header: '状态',
      render: (record) => (
        <StatusIndicator tone={contestStatusTone(record.contest_status)} label={contestStatusLabel(record.contest_status)} />
      ),
    },
    {
      key: 'rank',
      header: '名次',
      align: 'right',
      render: (record) => (
        <span className="font-semibold tabular-nums text-ink">{record.rank > 0 ? `第 ${record.rank} 名` : '—'}</span>
      ),
    },
    { key: 'score', header: '得分', align: 'right', mono: true, render: (record) => formatScore(record.score) },
    { key: 'solved_count', header: '解题数', align: 'right', mono: true },
    {
      key: 'last_solve_at',
      header: '最后解题',
      render: (record) =>
        record.last_solve_at ? (
          <time dateTime={record.last_solve_at} title={formatDateTime(record.last_solve_at)} className="text-ink-sub">
            {formatRelativeOccurrence(record.last_solve_at)}
          </time>
        ) : (
          <span className="text-ink-sub">还没有解出题</span>
        ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader title="竞赛战绩" />

      {records.status === 'error' ? (
        <ResourceError
          message={records.error?.message ?? '战绩暂时读取不到'}
          traceId={records.error?.traceId}
          onRetry={records.reload}
        />
      ) : records.status === 'loading' ? (
        <div aria-busy="true" className="flex flex-col gap-4">
          <Skeleton variant="stat" />
          <Skeleton variant="block" />
        </div>
      ) : records.status === 'empty' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Trophy} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">还没有参赛记录</p>
          <p className="mt-1.5 text-sm text-ink-sub">报名参加赛事后,每场的名次和得分都会记在这里。</p>
          <Link to="/student/contests" className={`${buttonVariants({ variant: 'primary' })} mt-5`}>
            去看看有哪些赛事
          </Link>
        </div>
      ) : (
        <>
          <MetricStrip
            label="战绩摘要"
            className="mb-6"
            items={[
              { label: '参赛场次', value: stats.count },
              {
                label: '最好名次',
                value: stats.bestRank !== undefined ? `第 ${stats.bestRank} 名` : '—',
                hint: stats.bestRank !== undefined ? '只算已结束的赛事' : '还没有已结束的赛事',
              },
              { label: '累计得分', value: formatScore(stats.totalScore) },
            ]}
          />
          <Table
            columns={columns}
            data={sorted}
            rowKey={(record) => record.contest_id}
            getRowHref={(record) => `/student/contests/${record.contest_id}`}
            onNavigate={(href) => navigate(href)}
            mobileCard={(record) => ({
              title: record.contest_name,
              meta: `${record.rank > 0 ? `第 ${record.rank} 名 · ` : ''}${formatScore(record.score)} 分 · 解出 ${record.solved_count} 题`,
              badge: (
                <StatusIndicator tone={contestStatusTone(record.contest_status)} label={contestStatusLabel(record.contest_status)} />
              ),
            })}
          />
        </>
      )}
    </PageScaffold>
  )
}
