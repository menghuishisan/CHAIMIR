// 竞赛参赛页(学生侧栏,/student/contests)。设计 §7.2:
// 页头 + 分段「报名中 / 进行中 / 已结束 / 全部」+ 搜索;卡片网格,卡片显示赛事名、赛制(解题 / 对抗)、
// 个人或组队、状态与时间提示(「报名还剩 2 天」「距结束 3 小时」、已结束写日期)、「我已报名」;
// 整卡链向赛事详情。我已报名且进行中的赛事排在最前。分段与搜索写进地址。
//
// 列表走学生专用路由 GET /contest/student/contests(后端只回非草稿赛事)。一个学生能看到的赛事
// 通常不到二十场,而「进行中」「已结束」各包含两个状态(接口的状态筛选只接受单个状态),
// 又要把「我已报名且进行中」排在最前 —— 故按每页最大条数取完(只带搜索词),在前端分段、
// 排序、分页。「我是否已报名」由 GET /contest/my/contest-records 判定(学生侧唯一能取到本人
// 队伍的接口,对齐清单 §3.1)。

import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router'
import { Swords, Users } from 'lucide-react'
import { PAGINATION_MAX_SIZE, TeamMode, type Contest } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Card,
  Icon,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { collectAllPages } from '../../../../utils/allPages'
import { readPage } from '../../../../utils/urlQuery'
import { contestModeLabel, contestStatusLabel, teamModeLabel } from '../../../../utils/labels/contest'
import { contestSegmentOf, sortStudentContests, type ContestSegment } from '../../contestOrder'
import { contestTimeHint } from '../../contestTimeline'
import { contestStatusTone } from '../../statusPresentation'

/** 前端分页的每页卡片数 */
const PAGE_SIZE = 12

const SEGMENTS: { value: 'all' | ContestSegment; label: string }[] = [
  { value: 'signup', label: '报名中' },
  { value: 'running', label: '进行中' },
  { value: 'ended', label: '已结束' },
  { value: 'all', label: '全部' },
]

/** 地址参数:status 是分段,q 是搜索词 */
const CONTESTS_QUERY = {
  status: { defaultValue: 'all', allowed: ['signup', 'running', 'ended', 'all'] },
  q: { defaultValue: '' },
} as const

/**
 * StudentContestsPage 列出可参与赛事并标注本人参赛状态。
 */
export default function StudentContestsPage() {
  const [query, setQuery] = useUrlState(CONTESTS_QUERY)
  const [searchParams, setSearchParams] = useSearchParams()
  const page = readPage(searchParams, 'page')

  const contests = useAsyncResource(
    () =>
      collectAllPages(
        (params) => api.contest.getStudentContests({ ...params, keyword: query.q || undefined }),
        PAGINATION_MAX_SIZE,
      ),
    [query.q],
    () => false,
  )
  const records = useAsyncResource(() => api.contest.getMyContestRecords(), [], () => false)
  const joined = useMemo(
    () => new Set((records.data ?? []).map((record) => record.contest_id)),
    [records.data],
  )

  const counts = useMemo(() => {
    const result = { signup: 0, running: 0, ended: 0, all: 0 }
    for (const contest of contests.data ?? []) {
      const segment = contestSegmentOf(contest.status)
      if (!segment) continue
      result[segment] += 1
      result.all += 1
    }
    return result
  }, [contests.data])

  const visible = useMemo(() => {
    const list = (contests.data ?? []).filter(
      (contest) => query.status === 'all' || contestSegmentOf(contest.status) === query.status,
    )
    return sortStudentContests(list, joined)
  }, [contests.data, joined, query.status])
  const pageItems = visible.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

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

  return (
    <PageScaffold>
      <PageHeader
        title="竞赛参赛"
        meta={records.status === 'success' && joined.size > 0 ? [{ label: '我已报名', value: `${joined.size} 场` }] : undefined}
      />

      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按赛事状态筛选"
          options={SEGMENTS.map((segment) => ({
            value: segment.value,
            label: segment.label,
            count: loaded ? counts[segment.value] : undefined,
          }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
        />
        <SearchBox label="搜索赛事" placeholder="搜索赛事名称" value={query.q} onSearch={(value) => setQuery({ q: value })} />
      </div>

      {contests.status === 'error' ? (
        <ResourceError
          message={contests.error?.message ?? '赛事暂时读取不到'}
          traceId={contests.error?.traceId}
          onRetry={contests.reload}
        />
      ) : !loaded ? (
        <ul className="card-grid gap-4" aria-busy="true" aria-label="正在读取赛事">
          {Array.from({ length: 3 }, (_, index) => (
            <li key={index} className="flex flex-col gap-3 rounded-lg bg-surface p-5 shadow-xs">
              <Skeleton variant="title" width="half" />
              <Skeleton variant="line" lines={2} />
            </li>
          ))}
        </ul>
      ) : pageItems.length === 0 ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Swords} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">{filtered ? '这里暂时没有赛事' : '还没有可以参加的赛事'}</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {filtered ? '切换到「全部」或换个关键词看看。' : '老师发布赛事后会显示在这里,报名开放时就能报名。'}
          </p>
          {filtered ? (
            <Button variant="outline" className="mt-5" onClick={() => setQuery({ status: 'all', q: '' })}>
              看全部赛事
            </Button>
          ) : null}
        </div>
      ) : (
        <>
          <ul className="card-grid gap-4" aria-busy={contests.status === 'loading' || undefined}>
            {pageItems.map((contest) => (
              <li key={contest.id}>
                <ContestCard contest={contest} joined={joined.has(contest.id)} />
              </li>
            ))}
          </ul>
          <div className="mt-6">
            <Pagination page={page} pageSize={PAGE_SIZE} total={visible.length} onPageChange={setPage} />
          </div>
        </>
      )}
    </PageScaffold>
  )
}

/** ContestCard 一场赛事:赛事名(整卡链接)、状态、赛制 · 参赛形式、时间提示、我已报名 */
function ContestCard({ contest, joined }: { contest: Contest; joined: boolean }) {
  const hint = contestTimeHint(contest)
  return (
    <Card interactive className="flex h-full flex-col gap-2 p-5">
      <div className="flex items-start justify-between gap-3">
        <Link to={`/student/contests/${contest.id}`} className="card-link line-clamp-2 text-lg font-bold text-ink">
          {contest.name}
        </Link>
        <StatusIndicator tone={contestStatusTone(contest.status)} label={contestStatusLabel(contest.status)} />
      </div>
      <p className="flex items-center gap-1.5 text-sm text-ink-sub">
        {contest.team_mode === TeamMode.GROUP ? <Icon icon={Users} size="xs" /> : null}
        {contestModeLabel(contest.mode)} · {teamModeLabel(contest.team_mode)}
      </p>
      <div className="mt-auto flex flex-wrap items-center justify-between gap-2 pt-3">
        <span className="text-sm font-medium tabular-nums text-ink">{hint ?? ''}</span>
        {joined ? <Badge tone="jade">我已报名</Badge> : null}
      </div>
    </Card>
  )
}
