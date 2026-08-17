// 竞赛参赛页(学生侧栏,/student/contests)。
// 列表走学生专用路由 GET /contest/student/contests(后端只回非草稿赛事)。
// 「我是否已报名」由 GET /contest/my/contest-records 判定 —— 它是学生侧唯一能取到本人
// team_id 的接口(对齐清单 §3.1),没有独立的「我的报名」接口。

import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { Swords, Trophy, UserCheck } from 'lucide-react'
import { ContestStatus, type Contest, type ContestRecord } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  Stat,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatDateTime, formatRelativeDeadline } from '../../../../utils/formatters'
import {
  contestModeLabel,
  contestStatusLabel,
  contestStatusTone,
  teamModeLabel,
} from '../../../../utils/labels/contest'

/** 报名进行中的状态:只有此状态下报名入口有效(后端 validateSignupWindow 同口径)。 */
const SIGNUP_OPEN_STATUS = ContestStatus.SIGNUP

/**
 * StudentContestsPage 列出可参与赛事并标注本人参赛状态。
 */
export default function StudentContestsPage() {
  const navigate = useNavigate()

  const contests = usePagedResource<Contest>(
    (params) => api.contest.getStudentContests(params),
    [],
  )
  // 战绩同时承担「已报名判定」:有记录说明已在某支队伍里
  const records = useAsyncResource(() => api.contest.getMyContestRecords(), [], () => false)

  const joinedContestIds = useMemo(
    () => new Set((records.data ?? []).map((record: ContestRecord) => record.contest_id)),
    [records.data],
  )

  // 指标带取服务端全量口径:第 1 页的 20 条里数出来的「报名中 3」在赛事更多时是错数
  const totalCount = useResourceTotal((params) => api.contest.getStudentContests(params), [])
  const signupOpenCount = useResourceTotal(
    (params) => api.contest.getStudentContests({ status: SIGNUP_OPEN_STATUS, ...params }),
    [],
  )
  const runningCount = useResourceTotal(
    (params) => api.contest.getStudentContests({ status: ContestStatus.RUNNING, ...params }),
    [],
  )

  const columns: TableColumn<Contest>[] = [
    {
      key: 'name',
      header: '赛事',
      render: (contest) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{contest.name}</div>
          <div className="truncate text-xs text-ink-sub">
            {contestModeLabel(contest.mode)} · {teamModeLabel(contest.team_mode)}
          </div>
        </div>
      ),
    },
    {
      key: 'signup',
      header: '报名时间',
      render: (contest) => (
        <div className="flex flex-col gap-1">
          <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
            {formatDateTime(contest.signup_start)} — {formatDateTime(contest.signup_end)}
          </span>
          {contest.status === SIGNUP_OPEN_STATUS ? (
            <Badge tone="info">{formatRelativeDeadline(contest.signup_end).text}截止报名</Badge>
          ) : null}
        </div>
      ),
    },
    {
      key: 'start_at',
      header: '比赛时间',
      render: (contest) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(contest.start_at)} — {formatDateTime(contest.end_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '赛事状态',
      render: (contest) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator tone={contestStatusTone(contest.status)} label={contestStatusLabel(contest.status)} />
          {joinedContestIds.has(contest.id) ? <Badge tone="jade">已报名</Badge> : null}
        </div>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (contest) => (
        <Button variant="ghost" size="sm" onClick={() => navigate(`/student/contests/${contest.id}`)}>
          {joinedContestIds.has(contest.id) ? '进入赛事' : '查看详情'}
        </Button>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '学习区' }, { label: '竞赛参赛' }]} />}
        title="竞赛参赛"
        description="报名后进入赛事详情可以看到赛题、榜单和你的队伍。"
        icon={Swords}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="可参与赛事" value={totalCount ?? '—'} icon={Swords} />
          <Stat label="报名中" value={signupOpenCount ?? '—'} icon={UserCheck} hint="报名窗口尚未关闭" />
          <Stat label="进行中" value={runningCount ?? '—'} icon={Trophy} />
        </div>
      </PageSection>

      <PageSection title="赛事列表" description={`共 ${contests.total} 场赛事`}>
        <ResourceState
          resource={contests}
          emptyIcon={Swords}
          emptyTitle="暂无可参与的赛事"
          emptyDescription="老师发布赛事后会显示在这里,报名窗口开放时即可报名。"
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <div className="flex flex-col gap-4">
              <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
              <Pagination
                page={contests.page}
                pageSize={contests.pageSize}
                total={contests.total}
                onPageChange={contests.setPage}
              />
            </div>
          )}
        </ResourceState>
      </PageSection>
    </PageScaffold>
  )
}
