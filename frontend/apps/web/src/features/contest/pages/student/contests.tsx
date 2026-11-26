// 学生赛事按参赛状态跨页排序；赛事与本人参赛记录必须一起读完，不能把读取失败当作未报名。

import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router'
import { Swords, Users } from 'lucide-react'
import { ContestStatus, PAGINATION_MAX_SIZE, TeamMode, type Contest } from '@chaimir/api-client'
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
import { CONTEST_STATUS_OPTIONS, contestModeLabel, contestStatusLabel, teamModeLabel } from '../../../../utils/labels/contest'
import { sortStudentContests } from '../../contestOrder'
import { contestTimeHint } from '../../contestTimeline'
import { contestStatusTone } from '../../statusPresentation'

/** 前端分页的每页卡片数 */
const PAGE_SIZE = 12

const STATUS_OPTIONS = [
  ...CONTEST_STATUS_OPTIONS.filter((option) => option.value !== String(ContestStatus.DRAFT)),
  { value: 'all', label: '全部' },
]

/** 地址参数:status 是分段,q 是搜索词 */
const CONTESTS_QUERY = {
  status: { defaultValue: 'all', allowed: STATUS_OPTIONS.map((option) => option.value) },
  q: { defaultValue: '' },
} as const

/**
 * StudentContestsPage 列出可参与赛事并标注本人参赛状态。
 */
export default function StudentContestsPage() {
  const [query, setQuery] = useUrlState(CONTESTS_QUERY)
  const [searchParams, setSearchParams] = useSearchParams()
  const page = readPage(searchParams, 'page')

  const view = useAsyncResource(
    async () => {
      const [contests, records] = await Promise.all([
        collectAllPages(
          (params) => api.contest.getStudentContests({ ...params, keyword: query.q || undefined }),
          PAGINATION_MAX_SIZE,
        ),
        api.contest.getMyContestRecords(),
      ])
      return { contests, joined: new Set(records.map((record) => record.contest_id)) }
    },
    [query.q],
    () => false,
  )
  const joined = view.data?.joined

  const counts = useMemo(() => {
    const result: Record<string, number> = { all: 0 }
    for (const contest of view.data?.contests ?? []) {
      result[contest.status] = (result[contest.status] ?? 0) + 1
      result.all += 1
    }
    return result
  }, [view.data])

  const visible = useMemo(() => {
    if (!view.data) return []
    const list = view.data.contests.filter(
      (contest) => query.status === 'all' || String(contest.status) === query.status,
    )
    return sortStudentContests(list, view.data.joined)
  }, [view.data, query.status])
  const currentPage = Math.min(page, Math.max(1, Math.ceil(visible.length / PAGE_SIZE)))
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

  const loaded = view.status === 'success' || view.status === 'empty'
  const filtered = query.status !== 'all' || query.q !== ''

  return (
    <PageScaffold>
      <PageHeader
        title="竞赛参赛"
        meta={loaded && joined && joined.size > 0 ? [{ label: '我已报名', value: `${joined.size} 场` }] : undefined}
      />

      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按赛事状态筛选"
          options={STATUS_OPTIONS.map((option) => ({
            ...option,
            count: loaded ? counts[option.value] ?? 0 : undefined,
          }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
          className="max-w-full overflow-x-auto"
        />
        <SearchBox label="搜索赛事" placeholder="搜索赛事名称" value={query.q} onSearch={(value) => setQuery({ q: value })} />
      </div>

      {view.status === 'error' ? (
        <ResourceError
          message={view.error?.message ?? '赛事暂时读取不到'}
          traceId={view.error?.traceId}
          onRetry={view.reload}
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
          <ul className="card-grid gap-4">
            {pageItems.map((contest) => (
              <li key={contest.id}>
                <ContestCard contest={contest} joined={joined?.has(contest.id) ?? false} />
              </li>
            ))}
          </ul>
          <div className="mt-6">
            <Pagination page={currentPage} pageSize={PAGE_SIZE} total={visible.length} onPageChange={setPage} />
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
