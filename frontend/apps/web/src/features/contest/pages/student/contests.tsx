// 竞赛参赛页(学生侧栏,/student/contests)。
// 列表走学生专用路由 GET /contest/student/contests(后端只回非草稿赛事)。
// 「我是否已报名」由 GET /contest/my/contest-records 判定 —— 它是学生侧唯一能取到本人
// team_id 的接口(对齐清单 §3.1),没有独立的「我的报名」接口。

import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Swords } from 'lucide-react'
import { ContestStatus, type Contest, type ContestRecord } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  DataPanel,
  FilterBar,
  FilterField,
  MetricStrip,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatDateTime, formatRelativeDeadline } from '../../../../utils/formatters'
import { contestStatusLabel } from '../../../../utils/labels/contest'
import { ContestIdentityCell, ContestScheduleCell } from '../../components/ContestTableCells'
import { contestStatusTone } from '../../statusPresentation'

/** 报名进行中的状态:只有此状态下报名入口有效(后端 validateSignupWindow 同口径)。 */
const SIGNUP_OPEN_STATUS = ContestStatus.SIGNUP

/** 状态筛选项:值为空串表示不过滤。学生看不到草稿赛事,故不列草稿。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(ContestStatus.SIGNUP), label: '报名中' },
  { value: String(ContestStatus.RUNNING), label: '进行中' },
  { value: String(ContestStatus.ENDED), label: '已结束' },
] as const

/**
 * StudentContestsPage 列出可参与赛事并标注本人参赛状态。
 */
export default function StudentContestsPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<string>('')

  const contests = usePagedResource<Contest>(
    (params) =>
      api.contest.getStudentContests({
        status: statusFilter ? (Number(statusFilter) as ContestStatus) : undefined,
        ...params,
      }),
    [statusFilter],
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
      render: (contest) => <ContestIdentityCell contest={contest} />,
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
      render: (contest) => <ContestScheduleCell contest={contest} />,
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
        kicker={<Breadcrumb items={[{ label: '学习区' }]} />}
        title="竞赛参赛"
        description="报名后进入赛事详情可以看到赛题、榜单和你的队伍。"
        icon={Swords}
      />

      {/* 指标降为内联摘要(§6.5.3 第 ① 族):本页主体是赛事列表,不是三个数字 */}
      <MetricStrip
        label="赛事总量摘要"
        className="mb-5"
        items={[
          { label: '可参与赛事', value: totalCount ?? '—', hint: '不受下方筛选影响' },
          { label: '报名中', value: signupOpenCount ?? '—', hint: '报名窗口尚未关闭' },
          { label: '进行中', value: runningCount ?? '—', hint: '正在进行的比赛' },
          { label: '我已报名', value: joinedContestIds.size, hint: '按我的战绩判定' },
        ]}
      />

      {/* 筛选井、数据表、分页同处一块抬起片(§6.5.2) */}
      <DataPanel
        label="赛事列表"
        filter={
          <FilterBar label="赛事筛选">
            <FilterField label="赛事状态" group>
              <SegmentedControl
                aria-label="按赛事状态筛选"
                size="sm"
                options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={statusFilter}
                onValueChange={setStatusFilter}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={contests.page}
            pageSize={contests.pageSize}
            total={contests.total}
            onPageChange={contests.setPage}
          />
        }
      >
        <ResourceState
          resource={contests}
          emptyIcon={Swords}
          emptyTitle={statusFilter ? '这个状态下没有赛事' : '暂无可参与的赛事'}
          emptyDescription={
            statusFilter
              ? '换个状态看看,或查看全部赛事。'
              : '老师发布赛事后会显示在这里,报名窗口开放时即可报名。'
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(page) => (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(item) => item.id}
              elevated={false}
              onRowClick={(item) => navigate(`/student/contests/${item.id}`)}
              // <md 换行卡(§6.4.1 规则 3):赛事名一行、比赛时间一行,状态在右
              mobileCard={(item) => ({
                title: item.name,
                meta: `比赛 ${formatDateTime(item.start_at)} 起 · 报名截止 ${formatDateTime(item.signup_end)}`,
                badge: (
                  <div className="flex flex-wrap items-center gap-1.5">
                    <StatusIndicator
                      tone={contestStatusTone(item.status)}
                      label={contestStatusLabel(item.status)}
                    />
                    {joinedContestIds.has(item.id) ? <Badge tone="jade">已报名</Badge> : null}
                  </div>
                ),
              })}
            />
          )}
        </ResourceState>
      </DataPanel>
    </PageScaffold>
  )
}
