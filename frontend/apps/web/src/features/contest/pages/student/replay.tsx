// 学生对局回放：按服务端时间窗回溯战绩，授权取件后展示链上轨迹。

import { useState } from 'react'
import type { ReactNode } from 'react'
import { useParams } from 'react-router'
import { Clock, Download, History, LoaderCircle, Swords, TriangleAlert, Trophy } from 'lucide-react'
import {
  BattleResult,
  BattleRole,
  PAGINATION_MAX_SIZE,
  type BattleReplayArchive,
  type BattleReplayWindow,
  type BattleReplayWindowItem,
  type LadderRank,
  type PaginatedResponse,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  ChainProgress,
  Pagination,
  WorkbenchShell,
  WorkbenchTopbar,
  type WorkbenchShortcut,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { AppStatusScreen } from '../../../../components/AppStatusScreen'
import { ResourceError } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import type { AsyncResourceState } from '../../../../hooks/useAsyncResource'
import { useImmersive } from '../../../../layouts/immersive/context'
import { downloadAttachment } from '../../../../utils/downloadAttachment'
import { formatDateTime } from '../../../../utils/formatters'
import { battleResultLabel, battleRoleLabel } from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { readBattleReplayArchive } from '../../battleTrace'
import {
  AttackDefenseTopology,
  ChainLogStream,
  TraceAssertions,
  TraceLegend,
} from '../../components/BattleTracePanels'
import { CONTEST_LADDER_PREVIEW_SIZE } from '../../queryLimits'

/** StudentContestReplayPage 加载本队回放时间窗与当前榜单。 */
export default function StudentContestReplayPage() {
  const { contestId = '' } = useParams<{ contestId: string }>()
  const { exit } = useImmersive()

  const replay = usePagedResource<BattleReplayWindowItem, BattleReplayWindow>(
    (params) => api.contest.getBattleReplayWindow(contestId, params),
    [contestId],
    { pageSize: PAGINATION_MAX_SIZE }
  )
  const ladder = useAsyncResource(
    () =>
      api.contest.getLadder(contestId, {
        page: 1,
        size: CONTEST_LADDER_PREVIEW_SIZE,
      }),
    [contestId],
    () => false
  )

  if (replay.status === 'loading') {
    return (
      <AppStatusScreen icon={LoaderCircle} spinning title="正在取回对局记录" fullScreen={false} />
    )
  }

  if (replay.status === 'error') {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="对局记录暂时读不到"
        description="对局记录加载失败,请重新加载后再试。"
        traceId={replay.error?.traceId}
        fullScreen={false}
        actions={
          <>
            <Button variant="on-dark" onClick={replay.reload}>
              重新加载
            </Button>
            <Button variant="on-dark" onClick={exit}>
              返回竞赛详情
            </Button>
          </>
        }
      />
    )
  }

  if (replay.status === 'empty' || !replay.data) {
    return (
      <AppStatusScreen
        icon={Swords}
        title="还没有对局"
        description="提交参战物后平台会自动匹配对手,打完的对局会出现在这条时间轴上。"
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回竞赛详情
          </Button>
        }
      />
    )
  }

  return (
    <Rewinder
      key={replay.page}
      window={replay.data}
      ladder={<LadderSection resource={ladder} pending={replay.data.pending} />}
      page={replay.page}
      pageSize={replay.pageSize}
      onPageChange={replay.setPage}
    />
  )
}

interface RewinderProps {
  window: BattleReplayWindow
  ladder: ReactNode
  page: number
  pageSize: number
  onPageChange: (page: number) => void
}

/** Rewinder 按游标展示历史战绩和选定对局的轨迹。 */
function Rewinder({ window, ladder, page, pageSize, onPageChange }: RewinderProps) {
  const { title, exit } = useImmersive()

  const timeline = window.list
  const timelineStart = (page - 1) * pageSize

  // 游标为零时显示当前窗口之前的检查点。
  const [cursor, setCursor] = useState(timeline.length)

  const played = timeline.slice(0, cursor)
  const currentItem = cursor > 0 ? timeline[cursor - 1] : undefined
  const currentMatch = currentItem?.match
  const currentMatchId = currentMatch?.id

  const standing = recomputeStanding(played, window.checkpoint)
  const entry = currentItem?.entry

  const failedIndexes = timeline.flatMap((item, index) =>
    index < cursor && outcomeOf(item) === 'lose' ? [timelineStart + index] : [])
  const attackAnchors = new Set(timeline.flatMap((item, index) => attackLandedAt(item) ? [index] : []))

  const [trace, setTrace] = useState<TraceState>({ loading: false, step: 0 })
  const activeTrace = trace.matchId === currentMatchId ? trace : { loading: false, step: 0 }
  const step = activeTrace.step
  function setStep(next: number | ((current: number) => number)) {
    setTrace((previous) => {
      if (previous.matchId !== currentMatchId || !previous.archive) return previous
      return { ...previous, step: typeof next === 'function' ? next(previous.step) : next }
    })
  }

  async function loadTrace() {
    if (!currentMatchId) return
    const pending: TraceState = { loading: true, matchId: currentMatchId, step: 0 }
    setTrace(pending)
    try {
      const archive = await readBattleReplayArchive(currentMatchId)
      // 只完成本次请求；较晚发起的请求和游标不受迟到响应影响。
      setTrace((previous) => previous === pending
        ? { ...pending, loading: false, archive, step: archive.actions.length }
        : previous)
    } catch (error) {
      setTrace((previous) => previous === pending
        ? { ...pending, loading: false, error: userFacingErrorMessage(error, '这一局的轨迹没有读到,请稍后重试。') }
        : previous)
    }
  }

  const [topbarError, setTopbarError] = useState<{ matchId: string; message: string }>()
  const topbarDownload = useReplayDownload((message) => {
    if (currentMatchId) setTopbarError({ matchId: currentMatchId, message })
  })
  const downloadError = topbarError?.matchId === currentMatchId ? topbarError?.message : undefined

  const actionCount = activeTrace.archive?.actions.length
  const timelineShortcuts: WorkbenchShortcut[] = [
    {
      key: 'ArrowLeft',
      repeat: true,
      description: '回退一局',
      disabled: cursor === 0,
      run: () => setCursor((current) => Math.max(0, current - 1)),
    },
    {
      key: 'ArrowRight',
      repeat: true,
      description: '前进一局',
      disabled: cursor >= timeline.length,
      run: () => setCursor((current) => Math.min(timeline.length, current + 1)),
    },
    {
      key: 'ArrowLeft',
      shift: true,
      repeat: true,
      description: '逐帧回退一步',
      disabled: actionCount === undefined || step === 0,
      run: () => setStep((current) => Math.max(0, current - 1)),
    },
    {
      key: 'ArrowRight',
      shift: true,
      repeat: true,
      description: '逐帧前进一步',
      disabled: actionCount === undefined || step >= actionCount,
      run: () => setStep((current) => Math.min(actionCount ?? 0, current + 1)),
    },
  ]

  return (
    <WorkbenchShell
      workbench="replay"
      topbar={
        <WorkbenchTopbar
          onExit={exit}
          exitLabel="退出回放"
          shortcuts={timelineShortcuts}
          title={title}
          subtitle={
            '已完成 ' +
            window.total +
            ' 局' +
            (window.pending > 0 ? '，' + window.pending + ' 局处理中' : '')
          }
          progress={
            <ChainProgress
              onDark
              size="sm"
              label="时间轴位置"
              total={window.total}
              done={timelineStart + cursor}
              failedIndexes={failedIndexes}
            />
          }
          cta={
            <div className="flex flex-wrap items-center gap-2">
              {downloadError ? <span role="alert" className="text-xs text-on-dark-danger">{downloadError}</span> : null}
              <Button
                variant="primary"
                size="sm"
                leftIcon={Download}
                loading={topbarDownload.downloading}
                disabled={!currentMatch?.replay_available}
                title={currentMatch?.replay_available ? undefined : '这一局没有留下轨迹记录'}
                onClick={() => {
                  if (!currentMatchId) return
                  setTopbarError(undefined)
                  void topbarDownload.download(currentMatch.id)
                }}
              >
                下载回放归档
              </Button>
            </div>
          }
        />
      }
      left={
        <SituationPanel
          standing={standing}
          entry={entry}
          globalCursor={timelineStart + cursor}
          total={window.total}
          ladder={ladder}
        />
      }
      leftLabel="当时的战绩与榜单"
      stage={
        <TimelineStage
          timeline={timeline}
          cursor={cursor}
          attackAnchors={attackAnchors}
          onCursorChange={setCursor}
          currentMatch={currentMatch}
          archive={activeTrace.archive}
          step={step}
          onStepChange={setStep}
        />
      }
      right={
        <ChainLogStream
          archive={activeTrace.archive}
          step={step}
          loading={activeTrace.loading}
          error={activeTrace.error}
          available={currentMatch?.replay_available === true}
          onLoad={() => void loadTrace()}
        />
      }
      rightLabel="链上日志流"
      footer={
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-2">
          <div className="flex min-w-0 flex-col">
            <span className="flex flex-wrap items-center gap-2 font-mono text-xs tabular-nums text-on-dark-sub">
              <Clock aria-hidden="true" className="size-3.5" />
              <span>
                {currentMatch
                  ? `回溯到 ${formatDateTime(currentMatch.finished_at ?? '')}`
                  : timelineStart > 0 ? '回溯到当前窗口开始之前' : '回溯到开赛之前'}
              </span>
              <span>
                第 {timelineStart + cursor}/{window.total} 局
              </span>
            </span>
            <span className="mt-0.5 text-xs text-on-dark-faint">
              拖动时间轴即按那一刻之前的对局重算战绩,数字都来自对局记录本身。
            </span>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button
              variant="on-dark"
              size="sm"
              disabled={cursor === 0}
              onClick={() => setCursor((current) => Math.max(0, current - 1))}
            >
              回退一局
            </Button>
            <Button
              variant="on-dark"
              size="sm"
              disabled={cursor >= timeline.length}
              onClick={() => setCursor((current) => Math.min(timeline.length, current + 1))}
            >
              前进一局
            </Button>
            <Button
              variant="on-dark"
              size="sm"
              leftIcon={History}
              disabled={
                cursor >= timeline.length && page >= Math.max(1, Math.ceil(window.total / pageSize))
              }
              onClick={() => {
                const lastPage = Math.max(1, Math.ceil(window.total / pageSize))
                if (page !== lastPage) {
                  onPageChange(lastPage)
                  return
                }
                setCursor(timeline.length)
              }}
            >
              回到最新
            </Button>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <span className="text-xs text-on-dark-faint">历史窗口</span>
            <Pagination
              page={page}
              pageSize={pageSize}
              total={window.total}
              onPageChange={onPageChange}
            />
          </div>
        </div>
      }
    />
  )
}

type Outcome = 'win' | 'lose' | 'draw' | 'unknown'

interface TraceState {
  matchId?: string
  loading: boolean
  step: number
  archive?: BattleReplayArchive
  error?: string
}

/** attackLandedAt 标记攻方获胜或守方失利；角色未知时不标记。 */
function attackLandedAt(item: BattleReplayWindowItem): boolean {
  const role = item.entry.role
  if (role !== BattleRole.ATTACK && role !== BattleRole.DEFENSE) return false
  const outcome = outcomeOf(item)
  if (outcome === 'unknown' || outcome === 'draw') return false
  return role === BattleRole.ATTACK ? outcome === 'win' : outcome === 'lose'
}

interface Standing {
  win: number
  lose: number
  draw: number
  /** 当时的积分(取最后一局记录里的赛后评分);没打过则未知 */
  rating?: number
  /** 这段时间里的积分净变化 */
  ratingDelta: number
}

/** recomputeStanding 累加胜负和净变化，积分直接读取服务端赛后记录。 */
function recomputeStanding(
  played: BattleReplayWindowItem[],
  checkpoint: BattleReplayWindow['checkpoint']
): Standing {
  const standing: Standing = {
    win: checkpoint.wins,
    lose: checkpoint.losses,
    draw: checkpoint.draws,
    ratingDelta: checkpoint.rating_delta,
    rating: checkpoint.wins + checkpoint.losses + checkpoint.draws > 0 ? checkpoint.rating : undefined,
  }
  for (const item of played) {
    const outcome = outcomeOf(item)
    if (outcome === 'win') standing.win += 1
    else if (outcome === 'lose') standing.lose += 1
    else if (outcome === 'draw') standing.draw += 1

    const delta =
      item.my_side === 'a'
        ? (item.match.score_delta?.delta_a ?? 0)
        : (item.match.score_delta?.delta_b ?? 0)
    standing.ratingDelta += delta
    const after =
      item.my_side === 'a'
        ? item.match.score_delta?.rating_a_after
        : item.match.score_delta?.rating_b_after
    if (after !== undefined) standing.rating = after
  }
  return standing
}

/** outcomeOf 使用服务端确定的本队方向,不从当前页参战物猜测。 */
function outcomeOf(item: Pick<BattleReplayWindowItem, 'match' | 'my_side'>): Outcome {
  const match = item.match
  if (match.result === undefined) return 'unknown'
  if (match.result === BattleResult.DRAW) return 'draw'
  const won = match.result === (item.my_side === 'a' ? BattleResult.A_WIN : BattleResult.B_WIN)
  return won ? 'win' : 'lose'
}

interface SituationPanelProps {
  standing: Standing
  entry?: BattleReplayWindowItem['entry']
  globalCursor: number
  total: number
  ladder: ReactNode
}

/** SituationPanel 组合历史战绩、参战物与当前榜单。 */
function SituationPanel({
  standing,
  entry,
  globalCursor,
  total,
  ladder,
}: SituationPanelProps) {
  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-y-auto p-4">
      <StandingSection
        standing={standing}
        entry={entry}
        cursor={globalCursor}
        total={total}
      />
      {ladder}
    </div>
  )
}

interface StandingSectionProps {
  standing: Standing
  entry?: BattleReplayWindowItem['entry']
  cursor: number
  total: number
}

/** StandingSection 展示游标处的战绩与参战物。 */
function StandingSection({ standing, entry, cursor, total }: StandingSectionProps) {
  return (
    <>
      <section className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <History aria-hidden="true" className="size-4 text-accent" />
          <h2 className="text-sm font-medium text-on-dark">时空回溯器</h2>
        </div>
        <p className="text-xs text-on-dark-sub">
          这里显示的是「回溯到那一刻」的战绩,不是当前战绩。拖动中间的时间轴就能换时刻。
        </p>
      </section>

      <section className="flex flex-col gap-2">
        <h3 className="text-xs font-medium text-on-dark-sub">当时战绩</h3>
        {cursor === 0 ? (
          <p className="text-sm text-on-dark-sub">还没打过对局。</p>
        ) : (
          <ul className="flex flex-col gap-1">
            <li className="flex items-center justify-between gap-2 rounded-md border border-dark-line bg-dark-surface px-2 py-1.5">
              <span className="text-xs text-on-dark-sub">胜 / 负 / 平</span>
              <span className="font-mono text-xs tabular-nums text-on-dark">
                {standing.win} / {standing.lose} / {standing.draw}
              </span>
            </li>
            <li className="flex items-center justify-between gap-2 rounded-md border border-dark-line bg-dark-surface px-2 py-1.5">
              <span className="text-xs text-on-dark-sub">当时积分</span>
              <span className="font-mono text-xs tabular-nums text-on-dark">
                {standing.rating !== undefined ? standing.rating : '未记录'}
              </span>
            </li>
            <li className="flex items-center justify-between gap-2 rounded-md border border-dark-line bg-dark-surface px-2 py-1.5">
              <span className="text-xs text-on-dark-sub">这段时间净变化</span>
              <span className="font-mono text-xs tabular-nums text-on-dark">
                {standing.ratingDelta > 0 ? `+${standing.ratingDelta}` : standing.ratingDelta}
              </span>
            </li>
            <li className="flex items-center justify-between gap-2 rounded-md border border-dark-line bg-dark-surface px-2 py-1.5">
              <span className="text-xs text-on-dark-sub">已回溯</span>
              <span className="font-mono text-xs tabular-nums text-on-dark">
                {cursor} / {total} 局
              </span>
            </li>
          </ul>
        )}
      </section>

      <section className="flex flex-col gap-2">
        <h3 className="text-xs font-medium text-on-dark-sub">本局使用的参战物</h3>
        {entry ? (
          <div className="flex flex-col gap-1 rounded-md border border-dark-line bg-dark-surface p-2">
            <span className="text-sm text-on-dark">
              第 {entry.version_no} 版 · {battleRoleLabel(entry.role)}
            </span>
            <span className="font-mono text-xs text-on-dark-sub">
              {formatDateTime(entry.submitted_at)} 提交
            </span>
          </div>
        ) : (
          <p className="text-xs text-on-dark-sub">
            选择一局后可查看本队实际使用的版本。
          </p>
        )}
        <p className="text-xs text-on-dark-faint">
          每局保留实际使用的版本，重新提交不会改变历史对局。
        </p>
      </section>
    </>
  )
}

interface TimelineStageProps {
  timeline: BattleReplayWindowItem[]
  cursor: number
  /** 攻方得手的局次下标,时间轴上以攻击锚点标出 */
  attackAnchors: ReadonlySet<number>
  onCursorChange: (cursor: number) => void
  currentMatch?: BattleReplayWindowItem['match']
  /** 当前这一局的轨迹归档;未取件时不画拓扑 */
  archive?: BattleReplayArchive
  step: number
  onStepChange: (step: number) => void
}

/** TimelineStage 固定拓扑与回溯条，仅让对局详情独立滚动。 */
function TimelineStage({
  timeline,
  cursor,
  attackAnchors,
  onCursorChange,
  currentMatch,
  archive,
  step,
  onStepChange,
}: TimelineStageProps) {
  const currentItem = currentMatch
    ? timeline.find((item) => item.match.id === currentMatch.id)
    : undefined
  const mySide = currentItem?.my_side

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 p-4">
      {archive ? (
        <AttackDefenseTopology archive={archive} mySide={mySide} step={step} />
      ) : (
        <section className="flex shrink-0 flex-col gap-1" aria-label="攻防拓扑">
          <h2 className="text-sm font-medium text-on-dark">攻防拓扑</h2>
          <p className="text-xs text-on-dark-sub">
            {currentMatch
              ? '双方的攻防角色只写在这一局的轨迹归档里。在右侧读取轨迹后,这里会画出攻防关系。'
              : '选一局之后才有攻防关系可画。'}
          </p>
        </section>
      )}

      <section className="flex shrink-0 flex-col gap-3">
        <h2 className="text-sm font-medium text-on-dark">对局时间轴</h2>
        <input
          type="range"
          min={0}
          max={timeline.length}
          step={1}
          value={cursor}
          aria-label="回溯到第几局"
          onChange={(event) => onCursorChange(Number(event.target.value))}
          className="w-full accent-jade-500"
        />
        {/* 局次超多时自己滚,不挤压下方详情 */}
        <ol className="flex max-h-24 flex-wrap gap-1.5 overflow-y-auto">
          {timeline.map((item, index) => {
            const outcome = outcomeOf(item)
            const reached = index < cursor
            const anchored = attackAnchors.has(index)
            return (
              <li key={item.match.id}>
                <button
                  type="button"
                  onClick={() => onCursorChange(index + 1)}
                  aria-label={`回溯到第 ${item.sequence} 局${anchored ? ',这一局攻方得手' : ''}`}
                  className={
                    'hit-target relative rounded-md border px-2 py-1 font-mono text-xs tabular-nums focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2 ' +
                    outcomeClass(outcome, reached)
                  }
                >
                  {item.sequence}
                  {/* 攻击锚点:颜色之外用一个上标记号,色弱用户同样能定位 */}
                  {anchored ? (
                    <span
                      aria-hidden="true"
                      className="absolute -top-1 right-0 text-on-dark-danger"
                    >
                      ×
                    </span>
                  ) : null}
                </button>
              </li>
            )
          })}
        </ol>
        <div className="flex flex-wrap items-center gap-2 text-xs text-on-dark-sub">
          <Badge onDark tone="success">
            胜
          </Badge>
          <Badge onDark tone="danger">
            负
          </Badge>
          <Badge onDark tone="neutral">
            平或未判定
          </Badge>
          <span>已回溯到的局次为实心;带 × 的是攻方得手的局。</span>
        </div>
      </section>

      {archive ? (
        <section className="flex shrink-0 flex-col gap-2" aria-label="逐帧回溯">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-sm font-medium text-on-dark">逐帧回溯</h2>
            <span className="font-mono text-xs tabular-nums text-on-dark-sub">
              第 {step}/{archive.actions.length} 步
            </span>
          </div>
          <input
            type="range"
            min={0}
            max={archive.actions.length}
            step={1}
            value={step}
            aria-label="回放到第几步链上动作"
            onChange={(event) => onStepChange(Number(event.target.value))}
            className="w-full accent-jade-500"
          />
          <TraceLegend />
        </section>
      ) : null}

      <div className="min-h-0 flex-1 overflow-y-auto">
        {currentMatch ? (
          <div className="flex flex-col gap-3">
            <MatchDetail key={currentMatch.id} match={currentMatch} mySide={mySide} />
            {archive ? <TraceAssertions archive={archive} /> : null}
          </div>
        ) : (
          <p className="text-sm text-on-dark-sub">
            {timeline[0]?.sequence > 1 ? '时间轴在当前窗口开始之前。' : '时间轴在开赛之前。'}
            往右拖或点一个局次,就能看到那一局发生了什么。
          </p>
        )}
      </div>
    </div>
  )
}

/** useReplayDownload 每次签发取件授权，下载失败交给调用方显示。 */
function useReplayDownload(onError: (message: string) => void) {
  const [downloading, setDownloading] = useState(false)
  async function download(matchId: string) {
    setDownloading(true)
    try {
      const grant = await api.contest.issueBattleReplayDownloadGrant(matchId)
      const file = await api.storage.consumeGrant(grant.token)
      downloadAttachment(file)
      toast.success('回放归档已开始下载')
    } catch (error) {
      onError(userFacingErrorMessage(error, '回放下载没有完成,请稍后重试。'))
    } finally {
      setDownloading(false)
    }
  }
  return { downloading, download }
}

/** outcomeClass 按结果与是否已回溯给出区块样式;色彩之外用实心/描边双通道区分。 */
function outcomeClass(outcome: Outcome, reached: boolean): string {
  if (!reached) return 'border-dark-line text-on-dark-faint'
  if (outcome === 'win') return 'border-jade-500 bg-jade-500 text-substrate'
  if (outcome === 'lose') return 'border-danger bg-danger-bg text-on-dark'
  return 'border-line-strong bg-dark-elevated text-on-dark'
}

interface MatchDetailProps {
  match: BattleReplayWindowItem['match']
  mySide?: BattleReplayWindowItem['my_side']
}

/** MatchDetail 展示对局事实和归档下载结果。 */
function MatchDetail({ match, mySide }: MatchDetailProps) {
  const [checkError, setCheckError] = useState<string>()
  const { downloading, download } = useReplayDownload(setCheckError)

  const side = mySide
  const outcome = side ? outcomeOf({ match, my_side: side }) : 'unknown'

  const before =
    side === 'b'
      ? match.score_delta?.rating_b_before
      : match.score_delta?.rating_a_before
  const after =
    side === 'b'
      ? match.score_delta?.rating_b_after
      : match.score_delta?.rating_a_after
  const delta = side === 'b' ? (match.score_delta?.delta_b ?? 0) : (match.score_delta?.delta_a ?? 0)

  return (
    <section className="flex flex-col gap-3 rounded-md border border-dark-line bg-dark-surface p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-medium text-on-dark">这一局</h3>
        <div className="flex flex-wrap items-center gap-2">
          <Badge
            onDark
            tone={outcome === 'win' ? 'success' : outcome === 'lose' ? 'danger' : 'neutral'}
          >
            {outcome === 'unknown'
              ? '未判定'
              : outcome === 'draw'
                ? '打平'
                : outcome === 'win'
                  ? '本队获胜'
                  : '本队失利'}
          </Badge>
          {match.result !== undefined ? (
            <Badge onDark tone="neutral">
              {battleResultLabel(match.result)}
            </Badge>
          ) : null}
          {side ? (
            <Badge onDark tone="neutral">
              本队为 {side === 'a' ? '先手' : '后手'}
            </Badge>
          ) : null}
        </div>
      </div>

      <dl className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <TimelineFact term="匹配时间" value={formatDateTime(match.matched_at)} />
        <TimelineFact
          term="结束时间"
          value={match.finished_at ? formatDateTime(match.finished_at) : '未记录'}
        />
        <TimelineFact term="赛前积分" value={before !== undefined ? String(before) : '未记录'} />
        <TimelineFact
          term="赛后积分"
          value={after !== undefined ? `${after}(${delta > 0 ? `+${delta}` : delta})` : '未记录'}
        />
      </dl>

      <div className="flex flex-wrap items-center gap-2">
        {match.replay_available ? (
          <>
            <Badge onDark tone="jade">轨迹已归档</Badge>
            <Button
              variant="on-dark"
              size="sm"
              leftIcon={Download}
              loading={downloading}
              onClick={() => {
                setCheckError(undefined)
                void download(match.id)
              }}
            >
              下载回放归档
            </Button>
            {checkError ? <span role="alert" className="text-xs text-on-dark-danger">{checkError}</span> : null}
          </>
        ) : (
          <span className="text-xs text-on-dark-sub">这一局没有留下轨迹记录。</span>
        )}
      </div>
    </section>
  )
}

/** TimelineFact 渲染一条对局事实。 */
function TimelineFact({ term, value }: { term: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs text-on-dark-sub">{term}</dt>
      <dd className="font-mono text-xs tabular-nums text-on-dark">{value}</dd>
    </div>
  )
}

interface LadderSectionProps {
  resource: AsyncResourceState<PaginatedResponse<LadderRank>>
  pending: number
}

/** LadderSection 展示当前榜单与待处理对局，不推算未留存的历史排名。 */
function LadderSection({ resource, pending }: LadderSectionProps) {
  const ranks = resource.data?.list ?? []
  return (
    <>
      <section className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Trophy aria-hidden="true" className="size-4 text-accent" />
          <h2 className="text-sm font-medium text-on-dark">当前天梯</h2>
        </div>
        {resource.status === 'loading' ? (
          <p role="status" className="text-xs text-on-dark-sub">正在读取榜单…</p>
        ) : resource.status === 'error' ? (
          <ResourceError
            onDark
            message={resource.error?.message ?? '榜单暂时读不到，请重新加载。'}
            traceId={resource.error?.traceId}
            onRetry={resource.reload}
          />
        ) : ranks.length === 0 ? (
          <p className="text-xs text-on-dark-sub">还没有队伍上榜。</p>
        ) : (
          <ol className="flex flex-col gap-1">
            {ranks.map((rank) => (
              <li
                key={rank.team_id}
                className="flex items-center justify-between gap-2 rounded-md border border-dark-line bg-dark-surface px-2 py-1.5"
              >
                <span className="min-w-0 truncate font-mono text-xs tabular-nums text-on-dark-sub">
                  第 {rank.rank} 名 · {rank.team_name}
                </span>
                <span className="font-mono text-xs tabular-nums text-on-dark">{rank.score} 分</span>
              </li>
            ))}
          </ol>
        )}
        <p className="text-xs text-on-dark-faint">
          榜单显示的是现在的名次,不随时间轴回溯 —— 历史名次没有留存,回溯它就成了编造。
        </p>
      </section>

      {pending > 0 ? (
        <section className="flex flex-col gap-1">
          <h3 className="text-xs font-medium text-on-dark-sub">还没打完</h3>
          <p className="text-xs text-on-dark-sub">
            有 {pending} 局在等待匹配或正在进行,打完后会出现在时间轴末尾。
          </p>
        </section>
      ) : null}
    </>
  )
}
