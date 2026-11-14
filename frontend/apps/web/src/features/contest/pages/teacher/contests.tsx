// 赛事组织页(教师侧栏,/teacher/contests)。设计 §7.4:
// 页头 +「新建赛事」,次要入口「漏洞题工坊」;分段按状态(进行中 / 报名中 / 未发布 / 已结束 / 全部)与搜索都写进地址;
// 表格(名称、赛制、状态、当前阶段的关键时间及倒计时),整行链向赛事详情;行内只给「下一步」一个按钮
// (草稿→发布、报名中→开赛、进行中→封榜或结束、已结束→归档),确认弹窗写清会发生什么。
//
// 列表接口只接受单个状态,而「进行中」「已结束」各对应两个状态,且排序要跨页(进行中的最前);
// 一所学校的赛事通常几场到十几场,故按页取完后在浏览器里分段、排序、分页(与学生端竞赛列表同一做法)。
// 列表接口返回本校全部赛事,详情与状态流转却只对组织者和学校管理员开放,所以只列自己能管的赛事。

import { useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { Bug, Plus, Trophy } from 'lucide-react'
import { ContestStatus, PAGINATION_MAX_SIZE, type Contest } from '@chaimir/api-client'
import {
  Button,
  buttonVariants,
  Icon,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  StatusIndicator,
  Table,
  cn,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useSession } from '../../../../components/RoleGuard/session'
import { ResourceError } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { collectAllPages } from '../../../../utils/allPages'
import { formatDateTime } from '../../../../utils/formatters'
import { contestModeLabel, contestStatusLabel, teamModeLabel } from '../../../../utils/labels/contest'
import { readPage } from '../../../../utils/urlQuery'
import { ContestFormModal } from '../../components/ContestFormModal'
import { ContestNextStepButton } from '../../components/ContestNextStep'
import { sortTeacherContests, teacherContestSegmentOf, type TeacherContestSegment } from '../../contestOrder'
import { contestTimeHint, contestTimeline } from '../../contestTimeline'
import { canManageContest } from '../../rules'
import { contestStatusTone } from '../../statusPresentation'

/** 前端分页的每页行数(与后端默认页大小一致) */
const PAGE_SIZE = 20

const SEGMENTS: { value: 'all' | TeacherContestSegment; label: string }[] = [
  { value: 'running', label: '进行中' },
  { value: 'signup', label: '报名中' },
  { value: 'draft', label: '未发布' },
  { value: 'ended', label: '已结束' },
  { value: 'all', label: '全部' },
]

/** 地址参数:status 是分段,q 是搜索词;页码在 page */
const CONTESTS_QUERY = {
  status: { defaultValue: 'all', allowed: ['running', 'signup', 'draft', 'ended', 'all'] },
  q: { defaultValue: '' },
} as const

/**
 * TeacherContestsPage 列出自己能管理的赛事,行内推进赛程;新建赛事后直接进入详情编排赛题。
 */
export default function TeacherContestsPage() {
  const navigate = useNavigate()
  const { me } = useSession()
  const [query, setQuery] = useUrlState(CONTESTS_QUERY)
  const [searchParams, setSearchParams] = useSearchParams()
  const page = readPage(searchParams, 'page')
  const [createOpen, setCreateOpen] = useState(false)

  const contests = useAsyncResource(
    () =>
      collectAllPages(
        (params) => api.contest.getContests({ keyword: query.q || undefined, ...params }),
        PAGINATION_MAX_SIZE,
      ),
    [query.q],
    () => false,
  )
  // 只列自己能管理的(组织者本人或学校管理员),其余赛事点进去也只会被拒绝
  const manageable = useMemo(
    () => (contests.data ?? []).filter((contest) => canManageContest(contest, me.account)),
    [contests.data, me.account],
  )

  const counts = useMemo(() => {
    const result = { running: 0, signup: 0, draft: 0, ended: 0, all: 0 }
    for (const contest of manageable) {
      const segment = teacherContestSegmentOf(contest.status)
      if (!segment) continue
      result[segment] += 1
      result.all += 1
    }
    return result
  }, [manageable])

  const visible = useMemo(
    () =>
      sortTeacherContests(
        manageable.filter(
          (contest) => query.status === 'all' || teacherContestSegmentOf(contest.status) === query.status,
        ),
      ),
    [manageable, query.status],
  )
  // 手改地址或数据变少后页码可能越界:显示最后一页,不显示空列表
  const lastPage = Math.max(1, Math.ceil(visible.length / PAGE_SIZE))
  const currentPage = Math.min(page, lastPage)
  const pageItems = visible.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)

  /** setPage 把页码写进地址(替换历史记录,第 1 页不写) */
  const setPage = (next: number) =>
    setSearchParams(
      (previous) => {
        const params = new URLSearchParams(previous)
        if (next === 1) params.delete('page')
        else params.set('page', String(next))
        return params
      },
      { replace: true },
    )

  const loaded = contests.status === 'success' || contests.status === 'empty'
  const filtered = query.status !== 'all' || query.q !== ''

  const columns: TableColumn<Contest>[] = [
    {
      key: 'name',
      header: '赛事',
      primary: true,
      render: (contest) => <span className="line-clamp-2 font-medium text-ink">{contest.name}</span>,
    },
    {
      key: 'mode',
      header: '赛制',
      render: (contest) => (
        <span className="whitespace-nowrap text-ink-sub">
          {contestModeLabel(contest.mode)} · {teamModeLabel(contest.team_mode)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (contest) => (
        <StatusIndicator tone={contestStatusTone(contest.status)} label={contestStatusLabel(contest.status)} />
      ),
    },
    {
      key: 'time',
      header: '关键时间',
      render: (contest) => <ContestKeyTime contest={contest} />,
    },
    {
      key: 'next',
      header: '下一步',
      align: 'right',
      render: (contest) => <ContestNextStepButton contest={contest} placement="row" onDone={contests.reload} />,
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        title="赛事组织"
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Link to="/teacher/vuln-workshop" className={buttonVariants({ variant: 'outline' })}>
              <Icon icon={Bug} size="sm" />
              漏洞题工坊
            </Link>
            <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
              新建赛事
            </Button>
          </div>
        }
      />

      <div className="mb-5 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <SegmentedControl
          aria-label="按赛事状态筛选"
          options={SEGMENTS.map((segment) => ({
            value: segment.value,
            label: segment.label,
            count: loaded ? counts[segment.value] : undefined,
          }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
          className="max-w-full overflow-x-auto"
        />
        <SearchBox label="搜索赛事" placeholder="搜索赛事名称" value={query.q} onSearch={(value) => setQuery({ q: value })} />
      </div>

      {contests.status === 'error' ? (
        <ResourceError
          message={contests.error?.message ?? '赛事暂时读取不到'}
          traceId={contests.error?.traceId}
          onRetry={contests.reload}
        />
      ) : loaded && pageItems.length === 0 ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Trophy} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">{filtered ? '没有符合条件的赛事' : '还没有赛事'}</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {filtered ? '换个状态或关键词试试。' : '新建赛事后编排赛题,发布即可开放报名。'}
          </p>
          {filtered ? (
            <Button variant="outline" className="mt-5" onClick={() => setQuery({ status: 'all', q: '' })}>
              看全部赛事
            </Button>
          ) : (
            <Button variant="primary" leftIcon={Plus} className="mt-5" onClick={() => setCreateOpen(true)}>
              新建赛事
            </Button>
          )}
        </div>
      ) : (
        <>
          <Table
            columns={columns}
            data={pageItems}
            rowKey={(contest) => contest.id}
            loading={!loaded}
            getRowHref={(contest) => `/teacher/contests/${contest.id}`}
            onNavigate={(href) => navigate(href)}
            // <md 换行卡(§6.4.1 规则 3):赛事名一行、赛制与关键时间一行,状态在右;下一步在详情页做
            mobileCard={(contest) => ({
              title: contest.name,
              meta: `${contestModeLabel(contest.mode)} · ${contestTimeHint(contest) ?? formatDateTime(contest.start_at)}`,
              badge: (
                <StatusIndicator tone={contestStatusTone(contest.status)} label={contestStatusLabel(contest.status)} />
              ),
            })}
          />
          {loaded ? (
            <div className="mt-4">
              <Pagination page={currentPage} pageSize={PAGE_SIZE} total={visible.length} onPageChange={setPage} />
            </div>
          ) : null}
        </>
      )}

      {createOpen ? (
        <ContestFormModal
          onClose={() => setCreateOpen(false)}
          onSaved={(saved) => {
            setCreateOpen(false)
            navigate(`/teacher/contests/${saved.id}`)
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

/**
 * ContestKeyTime 当前阶段的关键时间:上行是倒计时或结果(「报名还剩 2 天」「距结束 3 小时」「3 天内须发布」),
 * 下行是下一个赛程节点的完整时刻(草稿的发布期限就是报名截止,所以草稿显示报名截止);
 * 已结束的只留上行。报名截止已过的草稿用警示色。
 */
function ContestKeyTime({ contest }: { contest: Contest }) {
  const hint = contestTimeHint(contest)
  const nodes = contestTimeline(contest)
  const next =
    contest.status === ContestStatus.DRAFT
      ? nodes.find((node) => node.key === 'signup_end')
      : nodes.find((node) => node.state === 'upcoming')
  const overdueDraft = hint === '报名截止时间已过'
  return (
    <span className="flex flex-col">
      <span className={cn('whitespace-nowrap font-medium tabular-nums', overdueDraft ? 'text-warning' : 'text-ink')}>
        {hint ?? '—'}
      </span>
      {next && !overdueDraft ? (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {next.label} {formatDateTime(next.at)}
        </span>
      ) : null}
    </span>
  )
}
