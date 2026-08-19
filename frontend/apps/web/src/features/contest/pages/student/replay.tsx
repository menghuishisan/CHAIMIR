// 对局回放 · 时空回溯器(学生沉浸态,/student/contests/:contestId/replay)。
//
// 这是阶段 5 的创新②:把本队的对抗历程做成一条可以拖回去的时间轴,并在选定的一局里
// 继续按链上动作逐帧回放。为什么这样做 —— 对抗赛的结果是一串对局,每局都改变积分与名次;
// 学生看到的却只有一个当前分数,不知道是哪一版参战物、在哪一局、被哪一步链上操作打穿的。
// 于是把时间本身做成可操作对象:
//   外层游标拖到任一时刻 → 按**那一刻之前已结束的对局**重算当时战绩、积分与生效参战物;
//   内层游标拖到某一步   → 按**归档里已执行到的动作**画出攻防拓扑并让链上日志流增长。
//
// 状态一律由记录重算,不插值、不补帧(对齐清单 §6.13)。积分变化取对局自带的评分明细
// (delta/before/after 都是后端写下的事实),不在前端另算一套。
//
// 逐帧轨迹走「读取引用 → 签发授权 → 统一文件服务取件」,不把对象存储地址交给浏览器。
// 服务端回放窗口会提供本队方向与当局生效参战物摘要;逐帧轨迹仍以归档的 initial_state
// 为准。未取件前如实说明细粒度拓扑暂不可用,不拿猜出来的角色先画一个。
//
// 三区分工遵守规范 §7.1:左=当时的战绩与榜单(有界状态)、中=拓扑与两级回溯条(主体,不滚动)、
// 右=链上日志流(无界序列)。

import { useCallback, useEffect, useMemo, useState } from 'react'
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
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  ChainProgress,
  Pagination,
  WorkbenchShell,
  WorkbenchTopbar,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { AppStatusScreen } from '../../../../components/AppStatusScreen'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
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

/**
 * StudentContestReplayPage 取回本队对局与参战记录。
 */
export default function StudentContestReplayPage() {
  const { contestId = '' } = useParams<{ contestId: string }>()
  const { exit } = useImmersive()

  const replay = usePagedResource<BattleReplayWindowItem, BattleReplayWindow>(
    (params) => api.contest.getBattleReplayWindow(contestId, params),
    [contestId],
    PAGINATION_MAX_SIZE
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
      window={replay.data}
      ranks={ladder.data?.list ?? []}
      page={replay.page}
      pageSize={replay.pageSize}
      onPageChange={replay.setPage}
    />
  )
}

interface RewinderProps {
  window: {
    list: BattleReplayWindowItem[]
    total: number
    page: number
    size: number
    pending: number
    checkpoint: {
      wins: number
      losses: number
      draws: number
      rating_delta: number
      rating: number
    }
  }
  ranks: LadderRank[]
  page: number
  pageSize: number
  onPageChange: (page: number) => void
}

/**
 * Rewinder 渲染时间轴并按游标重算当时状态。
 */
function Rewinder({ window, ranks, page, pageSize, onPageChange }: RewinderProps) {
  const { title, exit } = useImmersive()

  // 回放窗口由服务端按完成时间和本队视角生成,不从通用分页切片重新猜测身份或历史。
  const timeline = window.list
  const timelineStart = (page - 1) * pageSize

  // 游标:指向时间轴上的第几局(0 表示开局之前,也就是还没打过)
  const [cursor, setCursor] = useState(timeline.length)

  // 切换服务端时间窗后默认停在该窗口的最新一局。
  useEffect(() => {
    setCursor(timeline.length)
  }, [page, timeline.length])

  const played = timeline.slice(0, cursor)
  const currentItem = cursor > 0 ? timeline[cursor - 1] : undefined
  const currentMatch = currentItem?.match

  const standing = useMemo(
    () => recomputeStanding(played, window.checkpoint),
    [played, window.checkpoint]
  )
  const activeEntry = currentItem?.active_entry

  const failedIndexes = useMemo(
    () =>
      timeline.reduce<number[]>((indexes, item, index) => {
        if (index < cursor && outcomeOf(item) === 'lose') indexes.push(timelineStart + index)
        return indexes
      }, []),
    [cursor, timeline, timelineStart]
  )

  // 攻击锚点:本队为攻方且获胜,或本队为守方且失利 —— 两种情况都意味着这一局攻方得手。
  // 角色取「那一刻生效的参战物」,分不出角色时不标锚点(与其猜一个红刻度,不如不给)。
  const attackAnchors = useMemo(
    () =>
      new Set(
        timeline.reduce<number[]>((indexes, item, index) => {
          if (attackLandedAt(item)) indexes.push(index)
          return indexes
        }, [])
      ),
    [timeline]
  )

  // 逐帧轨迹:每换一局都重新取件(授权是一次性短时凭据,不跨局缓存)
  const [trace, setTrace] = useState<TraceState>({ loading: false })
  const [step, setStep] = useState(0)

  useEffect(() => {
    setTrace({ loading: false })
    setStep(0)
  }, [currentMatch?.id])

  const loadTrace = useCallback(async () => {
    if (!currentMatch) return
    setTrace({ loading: true })
    try {
      const archive = await readBattleReplayArchive(currentMatch.id)
      setTrace({ loading: false, archive })
      // 取件后直接停在最后一步:先给完整结果,再让用户往回拖
      setStep(archive.actions.length)
    } catch (error) {
      setTrace({
        loading: false,
        error: userFacingErrorMessage(error, '这一局的轨迹没有读到,请稍后重试。'),
      })
    }
  }, [currentMatch])

  return (
    <WorkbenchShell
      workbench="replay"
      topbar={
        <WorkbenchTopbar
          onExit={exit}
          exitLabel="退出回放"
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
        />
      }
      left={
        <SituationPanel
          standing={standing}
          activeEntry={activeEntry}
          globalCursor={timelineStart + cursor}
          total={window.total}
          ranks={ranks}
          pending={window.pending}
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
          archive={trace.archive}
          step={step}
          onStepChange={setStep}
        />
      }
      right={
        <ChainLogStream
          archive={trace.archive}
          step={step}
          loading={trace.loading}
          error={trace.error}
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
                  : '回溯到开赛之前'}
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

/** Outcome 是本队在一局里的结果。 */
type Outcome = 'win' | 'lose' | 'draw' | 'unknown'

/** TraceState 是当前这一局逐帧轨迹的取件状态。 */
interface TraceState {
  loading: boolean
  archive?: BattleReplayArchive
  /** 用户向失败文案;技术原因只进控制台之外的错误链,不上界面 */
  error?: string
}

/**
 * attackLandedAt 判断这一局「攻方是否得手」,用于时间轴上的攻击锚点。
 * 判据只用事实:本队为攻方且获胜、或本队为守方且失利 —— 两者都说明攻击成功。
 * 那一刻的本队角色取生效的参战物;角色或胜负分不出来时回 false,不给猜出来的红刻度。
 */
function attackLandedAt(item: BattleReplayWindowItem): boolean {
  const role = item.active_entry?.role
  if (role !== BattleRole.ATTACK && role !== BattleRole.DEFENSE) return false
  const outcome = outcomeOf(item)
  if (outcome === 'unknown' || outcome === 'draw') return false
  return role === BattleRole.ATTACK ? outcome === 'win' : outcome === 'lose'
}

/** Standing 是某一时刻重算出来的战绩。 */
interface Standing {
  win: number
  lose: number
  draw: number
  /** 当时的积分(取最后一局记录里的赛后评分);没打过则未知 */
  rating?: number
  /** 这段时间里的积分净变化 */
  ratingDelta: number
}

/**
 * recomputeStanding 按已打完的对局重算战绩与积分。
 * 积分取对局记录里后端写下的赛后评分,净变化按每局的本队增量累加 ——
 * 不在前端重算 ELO,那会与服务端产生第二套算法。
 */
function recomputeStanding(
  played: BattleReplayWindowItem[],
  checkpoint: { wins: number; losses: number; draws: number; rating_delta: number; rating: number }
): Standing {
  const standing: Standing = {
    win: checkpoint.wins,
    lose: checkpoint.losses,
    draw: checkpoint.draws,
    ratingDelta: checkpoint.rating_delta,
    rating: checkpoint.rating || undefined,
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
        ? (item.match.score_delta?.rating_a_after ?? 0)
        : (item.match.score_delta?.rating_b_after ?? 0)
    if (after !== 0) standing.rating = after
  }
  return standing
}

/** outcomeOf 使用服务端确定的本队方向,不从当前页参战物猜测。 */
function outcomeOf(item: BattleReplayWindowItem): Outcome {
  const match = item.match
  if (match.result === undefined) return 'unknown'
  if (match.result === BattleResult.DRAW) return 'draw'
  const won = match.result === (item.my_side === 'a' ? BattleResult.A_WIN : BattleResult.B_WIN)
  return won ? 'win' : 'lose'
}

/** matchTime 取一局的时间刻度:优先结束时间,缺失时退到匹配时间。 */
interface SituationPanelProps {
  standing: Standing
  activeEntry?: BattleReplayWindowItem['active_entry']
  globalCursor: number
  total: number
  ranks: LadderRank[]
  pending: number
}

/**
 * SituationPanel 是左辅助区:当时的战绩、当时生效的参战物、当前天梯与待打对局。
 * 这些都是**有界状态**,按规范 §7.1 的状态与事件分流留在辅助区;
 * 无界的链上动作序列在右侧事件流,不在这里重复一份。
 */
function SituationPanel({
  standing,
  activeEntry,
  globalCursor,
  total,
  ranks,
  pending,
}: SituationPanelProps) {
  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-y-auto p-4">
      <StandingSection
        standing={standing}
        activeEntry={activeEntry}
        cursor={globalCursor}
        total={total}
      />
      <LadderSection ranks={ranks} pending={pending} />
    </div>
  )
}

interface StandingSectionProps {
  standing: Standing
  activeEntry?: BattleReplayWindowItem['active_entry']
  cursor: number
  total: number
}

/**
 * StandingSection 渲染当时的战绩与当时生效的参战物。
 */
function StandingSection({ standing, activeEntry, cursor, total }: StandingSectionProps) {
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
        <h3 className="text-xs font-medium text-on-dark-sub">当时生效的参战物</h3>
        {activeEntry ? (
          <div className="flex flex-col gap-1 rounded-md border border-dark-line bg-dark-surface p-2">
            <span className="text-sm text-on-dark">
              第 {activeEntry.version_no} 版 · {battleRoleLabel(activeEntry.role)}
            </span>
            <span className="font-mono text-xs text-on-dark-sub">
              {formatDateTime(activeEntry.submitted_at)} 提交
            </span>
          </div>
        ) : (
          <p className="text-xs text-on-dark-sub">
            {cursor === 0 ? '开赛之前还没有参战物。' : '这一刻之前没有找到对应的参战记录。'}
          </p>
        )}
        <p className="text-xs text-on-dark-faint">
          换版本会改变后续对局的打法,把时间轴拖过版本更替的位置就能看出差别。
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

/**
 * TimelineStage 是主舞台:攻防拓扑 + 两级回溯条(对局级 / 逐帧级)+ 这一局的事实。
 * 舞台永不滚动(规范 §7.1):两条回溯条与拓扑钉在上方,详情在下方自己滚 ——
 * 回溯条是这一台的主体,不能被局数或步数一多顶出视口。
 */
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
                  aria-label={`回溯到第 ${index + 1} 局${anchored ? ',这一局攻方得手' : ''}`}
                  className={
                    'hit-target relative rounded-md border px-2 py-1 font-mono text-xs tabular-nums focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2 ' +
                    outcomeClass(outcome, reached)
                  }
                >
                  {index + 1}
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
            <MatchDetail match={currentMatch} mySide={mySide} />
            {archive ? <TraceAssertions archive={archive} /> : null}
          </div>
        ) : (
          <p className="text-sm text-on-dark-sub">
            时间轴在开赛之前。往右拖或点一个局次,就能看到那一局发生了什么。
          </p>
        )}
      </div>
    </div>
  )
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

/**
 * MatchDetail 渲染游标处这一局的事实,并提供归档回放的受控取件入口。
 */
function MatchDetail({ match, mySide }: MatchDetailProps) {
  const [archived, setArchived] = useState<boolean>()
  const [checkError, setCheckError] = useState<string>()
  const [downloading, setDownloading] = useState(false)

  const side = mySide
  const outcome = side ? outcomeOf({ match, my_side: side, sequence: 0 }) : 'unknown'

  /** verifyTrace 确认这一局的轨迹已归档且本队有权访问(后端按队伍归属校验)。 */
  const verifyTrace = useCallback(async () => {
    setCheckError(undefined)
    try {
      const ref = await api.contest.getBattleReplay(match.id)
      setArchived(ref.available)
    } catch (error) {
      setArchived(false)
      setCheckError(userFacingErrorMessage(error, '这一局的轨迹暂时确认不了。'))
    }
  }, [match.id])

  /** downloadReplay 每次重新签发一次性授权,再交给统一文件服务下载。 */
  const downloadReplay = useCallback(async () => {
    setDownloading(true)
    setCheckError(undefined)
    try {
      const grant = await api.contest.issueBattleReplayDownloadGrant(match.id)
      const file = await api.storage.consumeGrant(grant.token)
      downloadAttachment(file)
      toast.success('回放归档已开始下载')
    } catch (error) {
      setCheckError(userFacingErrorMessage(error, '回放下载没有完成,请稍后重试。'))
    } finally {
      setDownloading(false)
    }
  }, [match.id])

  useEffect(() => {
    setArchived(undefined)
    setCheckError(undefined)
    if (match.replay_available) void verifyTrace()
  }, [match.replay_available, verifyTrace])

  const before =
    side === 'b'
      ? (match.score_delta?.rating_b_before ?? 0)
      : (match.score_delta?.rating_a_before ?? 0)
  const after =
    side === 'b'
      ? (match.score_delta?.rating_b_after ?? 0)
      : (match.score_delta?.rating_a_after ?? 0)
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
        <TimelineFact term="赛前积分" value={before !== 0 ? String(before) : '未记录'} />
        <TimelineFact
          term="赛后积分"
          value={after !== 0 ? `${after}(${delta > 0 ? `+${delta}` : delta})` : '未记录'}
        />
      </dl>

      <div className="flex flex-wrap items-center gap-2">
        {match.replay_available ? (
          archived === undefined ? (
            <span className="text-xs text-on-dark-sub">正在确认这一局的轨迹…</span>
          ) : archived ? (
            <>
              <Badge onDark tone="jade">
                轨迹已归档
              </Badge>
              <Button
                variant="on-dark"
                size="sm"
                leftIcon={Download}
                loading={downloading}
                onClick={() => void downloadReplay()}
              >
                下载回放归档
              </Button>
            </>
          ) : (
            <>
              <span className="text-xs text-on-dark-danger">{checkError}</span>
              <Button variant="on-dark" size="sm" onClick={() => void verifyTrace()}>
                重新确认
              </Button>
            </>
          )
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
  ranks: LadderRank[]
  pending: number
}

/**
 * LadderSection 展示当前天梯与还没打完的对局数。
 * 榜单是「现在」的,不随时间轴回溯 —— 后端只存当前投影,回溯它会变成前端编造。
 */
function LadderSection({ ranks, pending }: LadderSectionProps) {
  return (
    <>
      <section className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Trophy aria-hidden="true" className="size-4 text-accent" />
          <h2 className="text-sm font-medium text-on-dark">当前天梯</h2>
        </div>
        {ranks.length === 0 ? (
          <p className="text-xs text-on-dark-sub">还没有队伍上榜。</p>
        ) : (
          <ol className="flex flex-col gap-1">
            {ranks.map((rank) => (
              <li
                key={rank.team_id}
                className="flex items-center justify-between gap-2 rounded-md border border-dark-line bg-dark-surface px-2 py-1.5"
              >
                <span className="font-mono text-xs tabular-nums text-on-dark-sub">
                  第 {rank.rank} 名
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
