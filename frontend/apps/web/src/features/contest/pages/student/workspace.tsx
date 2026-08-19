// 竞赛答题工作台(学生沉浸态,/student/contests/:contestId/workspace)。
//
// 一道题的作答方式由题目自己声明(对齐清单 §6.19),工作台按声明分叉,不猜:
//   题面带运行时     → 起实操环境写代码,保存工作区拿到代码引用后提交;
//   题面带答案提交键 → 给一个答案输入框,按声明的键提交;
//   对抗题(带对抗配置) → 起环境做参战物,提交参战后由平台自动匹配对局。
// 三者都没有的题目,说明它还没声明作答方式并指引联系老师 —— 不构造任何猜测的提交体。
//
// 答案黑盒:题面来自 M5 的 face 投影(答案、判题配置与 flag 已在后端剥离),
// 本页只读 face,不请求 full,也不展示任何判题细节。

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { History, LoaderCircle, Send, Swords, Trophy, TriangleAlert } from 'lucide-react'
import {
  BattleRole,
  ContestStatus,
  type BattleEntry,
  type Contest,
  type ContestProblem,
  type ContestSubmission,
  type LadderRank,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  ChainProgress,
  Input,
  Pagination,
  WorkbenchShell,
  WorkbenchTopbar,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { AppStatusScreen } from '../../../../components/AppStatusScreen'
import { SandboxIdeWorkspace } from '../../../sandbox/components/SandboxIdeWorkspace'
import { useSession } from '../../../../components/RoleGuard'
import { useAsyncResource, usePagedResource, useTicketedWebSocket } from '../../../../hooks'
import { useImmersive } from '../../../../layouts/immersive/context'
import { readString, readStringArray } from '../../jsonReaders'
import { formatDateTime } from '../../../../utils/formatters'
import { battleRoleLabel, contestStatusLabel } from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CONTEST_LADDER_PREVIEW_SIZE } from '../../queryLimits'
import { isContestLeaderboardFrozen } from '../../rules'

/** 题面开放载荷里的键:与题库正文写入的键一致。 */
const FACE_KEYS = {
  scenario: 'scenario',
  statement: 'statement',
  title: 'title',
  runtimeCode: 'runtime_code',
  tools: 'tools',
  submitKey: 'submit_key',
} as const

/** 参战角色:攻防题分攻守,博弈题只有策略方。 */
const BATTLE_ROLES = [BattleRole.STRATEGY, BattleRole.ATTACK, BattleRole.DEFENSE] as const

/**
 * StudentContestWorkspacePage 装配竞赛答题工作台。
 */
export default function StudentContestWorkspacePage() {
  const { contestId = '' } = useParams<{ contestId: string }>()
  const { exit } = useImmersive()

  const contest = useAsyncResource(
    () => api.contest.getStudentContest(contestId),
    [contestId],
    () => false
  )
  const problems = useAsyncResource(() => api.contest.getProblems(contestId), [contestId])

  if (contest.status === 'loading' || problems.status === 'loading') {
    return <AppStatusScreen icon={LoaderCircle} spinning title="正在进入赛场" fullScreen={false} />
  }

  if (contest.status === 'error' || !contest.data) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="进不去这场比赛"
        description={contest.error?.message}
        traceId={contest.error?.traceId}
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回竞赛详情
          </Button>
        }
      />
    )
  }

  if (problems.status === 'empty' || !problems.data || problems.data.length === 0) {
    return (
      <AppStatusScreen
        icon={Swords}
        title="赛题还没有公布"
        description="开赛后赛题会出现在这里。"
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回竞赛详情
          </Button>
        }
      />
    )
  }

  return <ContestWorkbench contest={contest.data} problems={problems.data} />
}

interface ContestWorkbenchProps {
  contest: Contest
  problems: ContestProblem[]
}

/**
 * ContestWorkbench 持有当前赛题、实操环境与提交状态。
 */
function ContestWorkbench({ contest, problems }: ContestWorkbenchProps) {
  const { title, exit } = useImmersive()
  const navigate = useNavigate()
  const { me } = useSession()

  const [problemId, setProblemId] = useState(problems[0].id)
  const [sandboxId, setSandboxId] = useState<string>()
  const [codeRef, setCodeRef] = useState<{ key: string; hash: string }>()
  const [answer, setAnswer] = useState('')
  const [role, setRole] = useState<BattleRole>(BattleRole.STRATEGY)
  const [submission, setSubmission] = useState<ContestSubmission>()
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const problem = useMemo(
    () => problems.find((item) => item.id === problemId) ?? problems[0],
    [problemId, problems]
  )

  const spec = useMemo(() => problemSpec(problem), [problem])
  const answerable = contest.status === ContestStatus.RUNNING

  const ladder = useAsyncResource(
    () =>
      api.contest.getLadder(contest.id, {
        page: 1,
        size: CONTEST_LADDER_PREVIEW_SIZE,
      }),
    [contest.id],
    () => false
  )

  // 榜单实时更新:订阅本场比赛的榜单 topic(经 M10 统一业务 WS,短时票据建连)。
  // 推送只当刷新信号 —— 榜单本身回接口读,推送内容不用来改本地状态。
  const ladderTopic = useMemo(
    () =>
      me.account.tenant_id
        ? api.contest.getLeaderboardTopic(me.account.tenant_id, contest.id)
        : undefined,
    [contest.id, me.account.tenant_id]
  )
  const ladderSocket = useTicketedWebSocket({
    url: ladderTopic ? api.eventWebSocketUrl() : undefined,
    onOpen: () => {
      if (ladderTopic) {
        ladderSocket.send(JSON.stringify({ action: 'subscribe', topics: [ladderTopic] }))
      }
    },
    onMessage: (data) => {
      if (data.includes('"type":"subscribed"')) return
      ladder.reload()
    },
  })

  const entries = usePagedResource<BattleEntry>(
    (params) =>
      spec.kind === 'battle'
        ? api.contest.listBattleEntries(contest.id, params)
        : Promise.resolve({ list: [], total: 0, page: params.page, size: params.size }),
    [contest.id, spec.kind]
  )

  // 换题即换环境:上一题的沙箱与代码引用对这一题没有意义
  useEffect(() => {
    setSandboxId(undefined)
    setCodeRef(undefined)
    setAnswer('')
    setSubmission(undefined)
    setActionError(undefined)
  }, [problemId])

  /** createEnv 起一个实操环境:运行时按题目声明,镜像版本留空即用该运行时的默认镜像。 */
  const createEnv = useCallback(async () => {
    if (!spec.runtimeCode) return
    setBusy(true)
    setActionError(undefined)
    try {
      const env = await api.contest.createEnv(contest.id, problem.id, {
        runtime_code: spec.runtimeCode,
        runtime_image_version: spec.runtimeImageVersion,
        tool_codes: spec.toolCodes,
      })
      setSandboxId(env.sandbox_id)
      toast.success('实操环境已就绪')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '实操环境没能准备好,请稍后重试。'))
    } finally {
      setBusy(false)
    }
  }, [contest.id, problem.id, spec])

  /** submitSolve 提交解题赛答案:代码题带代码引用,答题类按声明的键带答案。 */
  const submitSolve = useCallback(async () => {
    setBusy(true)
    setActionError(undefined)
    try {
      const contentRef: Record<string, unknown> =
        spec.submitKey && answer.trim() !== '' ? { [spec.submitKey]: answer.trim() } : {}
      const result = await api.contest.submitSolve(contest.id, problem.id, {
        content_ref: contentRef,
        code_storage_key: codeRef?.key,
        code_hash: codeRef?.hash,
        sandbox_ref: sandboxId,
      })
      setSubmission(result)
      toast.success(result.passed ? '提交通过' : '已提交,等判定结果')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '提交没有成功,请稍后重试。'))
    } finally {
      setBusy(false)
    }
  }, [answer, codeRef, contest.id, problem.id, sandboxId, spec.submitKey])

  /** submitBattle 提交参战物:参战后由平台自动匹配对局,结果在对局回放里看。 */
  const submitBattle = useCallback(async () => {
    if (!codeRef) {
      setActionError('先在实操环境里点「保存工作区」,把参战代码存成一份快照再提交。')
      return
    }
    setBusy(true)
    setActionError(undefined)
    try {
      await api.contest.submitBattleEntry(contest.id, {
        problem_id: problem.id,
        role,
        artifact_ref: codeRef.key,
        code_hash: codeRef.hash,
      })
      toast.success('参战物已提交,等待匹配对局')
      entries.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '参战提交没有成功,请稍后重试。'))
    } finally {
      setBusy(false)
    }
  }, [codeRef, contest.id, entries, problem.id, role])

  /** refreshSubmission 重读提交结果:判题是异步的,提交回来时结论可能还没出。 */
  const refreshSubmission = useCallback(async () => {
    if (!submission) return
    try {
      setSubmission(await api.contest.getSubmission(submission.id))
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '判定结果暂时读不到,稍后再试一次。'))
    }
  }, [submission])

  const solvedCount = ladder.data?.list.find(() => true)?.solved_count ?? 0

  return (
    <WorkbenchShell
      workbench="contest"
      topbar={
        <WorkbenchTopbar
          onExit={exit}
          exitLabel="退出赛场"
          title={title}
          subtitle={contest.name}
          progress={
            <ChainProgress
              onDark
              size="sm"
              label="已解题数"
              total={problems.length}
              done={Math.min(solvedCount, problems.length)}
            />
          }
          cta={
            spec.kind === 'battle' ? (
              <Button
                variant="seal"
                size="sm"
                leftIcon={Swords}
                loading={busy}
                disabled={!answerable}
                onClick={() => void submitBattle()}
              >
                提交参战
              </Button>
            ) : (
              <Button
                variant="seal"
                size="sm"
                leftIcon={Send}
                loading={busy}
                disabled={!answerable || spec.kind === 'undeclared'}
                onClick={() => void submitSolve()}
              >
                提交答案
              </Button>
            )
          }
        />
      }
      left={
        <ProblemBrief
          problems={problems}
          currentId={problem.id}
          onPick={setProblemId}
          spec={spec}
        />
      }
      leftLabel="赛题"
      stage={
        <ProblemStage
          spec={spec}
          sandboxId={sandboxId}
          busy={busy}
          answerable={answerable}
          answer={answer}
          onAnswerChange={setAnswer}
          onCreateEnv={() => void createEnv()}
          onSaved={(result) => setCodeRef({ key: result.code_storage_key, hash: result.code_hash })}
        />
      }
      right={
        // 壳不给槽位套滚动(主舞台永不滚动,规范 §7.1),这一栏的两块内容自己滚
        <div className="flex h-full min-h-0 flex-col overflow-y-auto">
          {spec.kind === 'battle' ? (
            <BattlePanel
              role={role}
              onRoleChange={setRole}
              entries={entries.data?.list ?? []}
              entryStatus={entries.status}
              entryError={entries.error?.message}
              entryTraceId={entries.error?.traceId}
              onEntryRetry={entries.reload}
              entryPage={entries.page}
              entryPageSize={entries.pageSize}
              entryTotal={entries.total}
              onEntryPageChange={entries.setPage}
              onOpenReplay={() => navigate(`/student/contests/${contest.id}/replay`)}
            />
          ) : (
            <SubmissionPanel submission={submission} onRefresh={() => void refreshSubmission()} />
          )}
          <LadderPanel
            ranks={ladder.data?.list ?? []}
            live={ladderSocket.status === 'open'}
            frozen={isContestLeaderboardFrozen(contest.status)}
          />
        </div>
      }
      rightLabel="提交与榜单"
      footer={
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-2">
          <div className="flex min-w-0 flex-col">
            <span className="flex flex-wrap items-center gap-2 text-xs text-on-dark-sub">
              <Badge onDark tone={answerable ? 'jade' : 'neutral'}>
                {contestStatusLabel(contest.status)}
              </Badge>
              <span className="font-mono tabular-nums">本题 {problem.score} 分</span>
              {codeRef ? <span>代码已保存</span> : null}
            </span>
            {actionError ? (
              <span className="mt-0.5 text-xs text-on-dark-danger">{actionError}</span>
            ) : null}
            {!answerable ? (
              <span className="mt-0.5 text-xs text-on-dark-sub">
                当前不在作答时间内,只能查看题面与榜单。
              </span>
            ) : null}
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button variant="on-dark" size="sm" leftIcon={Trophy} onClick={ladder.reload}>
              刷新榜单
            </Button>
          </div>
        </div>
      }
    />
  )
}

/** ProblemSpec 是一道题声明出来的作答方式。 */
interface ProblemSpec {
  kind: 'code' | 'answer' | 'battle' | 'undeclared'
  title: string
  scenario: string
  statement: string
  runtimeCode?: string
  runtimeImageVersion: string
  toolCodes: string[]
  submitKey?: string
}

/**
 * problemSpec 从题面与对抗配置里读出这道题怎么答。
 * 对抗题优先:它的环境参数在对抗配置里(后端要求运行时与镜像版本必填),
 * 与解题赛不共用一条路径。
 */
function problemSpec(problem: ContestProblem): ProblemSpec {
  const face = problem.face ?? {}
  const base = {
    title: readString(face, FACE_KEYS.title) || `第 ${problem.seq} 题`,
    scenario: readString(face, FACE_KEYS.scenario),
    statement: readString(face, FACE_KEYS.statement),
  }

  if (problem.battle_rule !== undefined) {
    const config = problem.battle_config
    return {
      ...base,
      kind: 'battle',
      runtimeCode: config?.runtime_code || undefined,
      runtimeImageVersion: config?.runtime_image_version ?? '',
      toolCodes: config?.tool_codes ?? [],
    }
  }

  const runtimeCode = readString(face, FACE_KEYS.runtimeCode)
  const submitKey = readString(face, FACE_KEYS.submitKey)
  return {
    ...base,
    kind: runtimeCode !== '' ? 'code' : submitKey !== '' ? 'answer' : 'undeclared',
    runtimeCode: runtimeCode || undefined,
    // 题面不声明镜像版本:留空即用该运行时的默认镜像
    runtimeImageVersion: '',
    toolCodes: readStringArray(face, FACE_KEYS.tools),
    submitKey: submitKey || undefined,
  }
}

interface ProblemBriefProps {
  problems: ContestProblem[]
  currentId: string
  onPick: (problemId: string) => void
  spec: ProblemSpec
}

/**
 * ProblemBrief 渲染赛题清单与当前题面。
 */
function ProblemBrief({ problems, currentId, onPick, spec }: ProblemBriefProps) {
  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-y-auto p-4">
      <section className="flex flex-col gap-2">
        <h2 className="text-sm font-medium text-on-dark">赛题</h2>
        <ul className="flex flex-col gap-1">
          {problems.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                onClick={() => onPick(item.id)}
                className={
                  'hit-target relative flex w-full items-center justify-between gap-2 rounded-md border px-2 py-1.5 text-left focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2 ' +
                  (item.id === currentId
                    ? 'border-accent bg-dark-elevated'
                    : 'border-dark-line bg-dark-surface hover:bg-dark-elevated')
                }
              >
                <span className="min-w-0 truncate text-sm text-on-dark">
                  {readString(item.face ?? {}, FACE_KEYS.title) || `第 ${item.seq} 题`}
                </span>
                <span className="shrink-0 font-mono text-xs tabular-nums text-on-dark-sub">
                  {item.score}
                </span>
              </button>
            </li>
          ))}
        </ul>
      </section>

      <section className="flex flex-col gap-2">
        <h3 className="text-xs font-medium text-on-dark-sub">题面</h3>
        {spec.scenario ? (
          <p className="whitespace-pre-wrap text-xs leading-relaxed text-on-dark-sub">
            {spec.scenario}
          </p>
        ) : null}
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-on-dark">
          {spec.statement || '这道题还没有题面内容。'}
        </p>
      </section>
    </div>
  )
}

interface ProblemStageProps {
  spec: ProblemSpec
  sandboxId?: string
  busy: boolean
  answerable: boolean
  answer: string
  onAnswerChange: (value: string) => void
  onCreateEnv: () => void
  onSaved: (result: { code_storage_key: string; code_hash: string }) => void
}

/**
 * ProblemStage 按题目声明的作答方式渲染中间区。
 */
function ProblemStage({
  spec,
  sandboxId,
  busy,
  answerable,
  answer,
  onAnswerChange,
  onCreateEnv,
  onSaved,
}: ProblemStageProps) {
  if (spec.kind === 'undeclared') {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 p-6 text-center">
        <p className="text-base text-on-dark">这道题还没声明作答方式</p>
        <p className="max-w-md text-sm text-on-dark-sub">
          它既没有配实操环境,也没有说明答案该填在哪里。请把这道题的编号告诉老师。
        </p>
      </div>
    )
  }

  if (spec.kind === 'answer') {
    return (
      <div className="flex h-full flex-col gap-3 p-6">
        <label className="flex flex-col gap-1">
          <span className="text-sm text-on-dark">你的答案</span>
          <Input
            variant="underline"
            value={answer}
            disabled={!answerable}
            placeholder="按题面要求填写"
            className="font-mono text-sm"
            onChange={(event) => onAnswerChange(event.target.value)}
          />
        </label>
        <p className="text-xs text-on-dark-sub">
          填好后点右上角「提交答案」。同一题连续提交有冷却时间,判错也会短暂锁一会儿。
        </p>
      </div>
    )
  }

  if (!sandboxId) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="text-base text-on-dark">
          {spec.kind === 'battle' ? '准备参战物需要一个实操环境' : '这道题要在实操环境里完成'}
        </p>
        <p className="max-w-md text-sm text-on-dark-sub">
          环境按题目指定的链运行时准备,起好后可以写代码、开终端、做链上操作。
        </p>
        <Button variant="primary" loading={busy} disabled={!answerable} onClick={onCreateEnv}>
          准备实操环境
        </Button>
        {!spec.runtimeCode ? (
          <p className="text-xs text-on-dark-danger">
            这道题没有指定链运行时,暂时起不了环境。请把题号告诉老师。
          </p>
        ) : null}
      </div>
    )
  }

  return <SandboxIdeWorkspace key={sandboxId} sandboxId={sandboxId} onSaved={onSaved} />
}

/**
 * SubmissionPanel 展示最近一次提交的判定结果。
 * 判题是异步的:提交回来时可能还没有结论,故给一个重读入口而不是假装已完成。
 */
function SubmissionPanel({
  submission,
  onRefresh,
}: {
  submission?: ContestSubmission
  onRefresh: () => void
}) {
  return (
    <section className="flex flex-col gap-2 border-b border-dark-line p-4">
      <h2 className="text-sm font-medium text-on-dark">提交结果</h2>
      {submission ? (
        <div className="flex flex-col gap-2 rounded-md border border-dark-line bg-dark-surface p-2">
          <div className="flex items-center justify-between gap-2">
            <Badge onDark tone={submission.passed ? 'success' : 'neutral'}>
              {submission.passed ? '已通过' : '判定中或未通过'}
            </Badge>
            <span className="font-mono text-xs tabular-nums text-on-dark-sub">
              {submission.score} 分
            </span>
          </div>
          <span className="font-mono text-xs text-on-dark-faint">
            {formatDateTime(submission.submitted_at)}
          </span>
          <Button variant="on-dark" size="sm" onClick={onRefresh}>
            重读判定结果
          </Button>
        </div>
      ) : (
        <p className="text-xs text-on-dark-sub">还没有提交。提交后这里显示判定结果。</p>
      )}
    </section>
  )
}

interface BattlePanelProps {
  role: BattleRole
  onRoleChange: (role: BattleRole) => void
  entries: BattleEntry[]
  entryStatus: 'loading' | 'success' | 'empty' | 'error'
  entryError?: string
  entryTraceId?: string
  onEntryRetry: () => void
  entryPage: number
  entryPageSize: number
  entryTotal: number
  onEntryPageChange: (page: number) => void
  onOpenReplay: () => void
}

/**
 * BattlePanel 承载参战角色选择与参战历史。
 * 只有最新一版参战物生效(后端按版本号取 is_active),历史版本留档便于对照。
 */
function BattlePanel({
  role,
  onRoleChange,
  entries,
  entryStatus,
  entryError,
  entryTraceId,
  onEntryRetry,
  entryPage,
  entryPageSize,
  entryTotal,
  onEntryPageChange,
  onOpenReplay,
}: BattlePanelProps) {
  return (
    <section className="flex flex-col gap-3 border-b border-dark-line p-4">
      <h2 className="text-sm font-medium text-on-dark">参战</h2>

      <div className="flex flex-col gap-1">
        <span className="text-xs text-on-dark-sub">参战角色</span>
        <div className="flex flex-wrap gap-2">
          {BATTLE_ROLES.map((item) => (
            <Button
              key={item}
              variant={item === role ? 'seal' : 'on-dark'}
              size="sm"
              onClick={() => onRoleChange(item)}
            >
              {battleRoleLabel(item)}
            </Button>
          ))}
        </div>
        <p className="text-xs text-on-dark-faint">
          攻防题按攻方或守方参战,博弈题选策略方。角色要与题面要求一致。
        </p>
      </div>

      <div className="flex flex-col gap-2">
        <span className="text-xs text-on-dark-sub">参战历史</span>
        {entryStatus === 'loading' ? (
          <p className="text-xs text-on-dark-sub">正在读取参战历史...</p>
        ) : entryStatus === 'error' ? (
          <div className="flex flex-col gap-2 rounded-md border border-danger bg-danger-bg p-3">
            <p className="text-xs text-on-dark">
              {entryError ?? '参战历史暂时读不到,请稍后重试。'}
            </p>
            {entryTraceId ? (
              <p className="font-mono text-xs text-on-dark-faint">报障编号: {entryTraceId}</p>
            ) : null}
            <Button variant="on-dark" size="sm" onClick={onEntryRetry}>
              重新加载
            </Button>
          </div>
        ) : entryStatus === 'empty' ? (
          <p className="text-xs text-on-dark-sub">还没有提交过参战物。</p>
        ) : (
          <>
            <ul className="flex flex-col gap-1">
              {entries.map((entry) => (
                <li
                  key={entry.id}
                  className="flex items-center justify-between gap-2 rounded-md border border-dark-line bg-dark-surface px-2 py-1.5"
                >
                  <span className="min-w-0 truncate text-xs text-on-dark">
                    第 {entry.version_no} 版 · {battleRoleLabel(entry.role)}
                  </span>
                  {entry.is_active ? (
                    <Badge onDark tone="jade">
                      生效中
                    </Badge>
                  ) : null}
                </li>
              ))}
            </ul>
            <Pagination
              page={entryPage}
              pageSize={entryPageSize}
              total={entryTotal}
              onPageChange={onEntryPageChange}
            />
          </>
        )}
        <Button variant="on-dark" size="sm" leftIcon={History} onClick={onOpenReplay}>
          看对局回放
        </Button>
      </div>
    </section>
  )
}

interface LadderPanelProps {
  ranks: LadderRank[]
  /** 榜单实时通道是否已连上;没连上时说明需要手动刷新 */
  live: boolean
  /** 封榜期:榜单不再更新,当前名次可能与最终结果不同 */
  frozen: boolean
}

/**
 * LadderPanel 展示天梯前列。
 * 队伍只显示名次与分数:队伍编号是内部标识,榜单上显示它既不可读也没有意义。
 */
function LadderPanel({ ranks, live, frozen }: LadderPanelProps) {
  return (
    <section className="flex flex-col gap-2 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Trophy aria-hidden="true" className="size-4 text-accent" />
        <h2 className="text-sm font-medium text-on-dark">天梯前列</h2>
        {frozen ? (
          <Badge onDark tone="warning">
            封榜中
          </Badge>
        ) : live ? (
          <Badge onDark tone="jade">
            实时
          </Badge>
        ) : null}
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
              <span className="font-mono text-xs tabular-nums text-on-dark">
                {rank.score} 分 · 解出 {rank.solved_count}
              </span>
            </li>
          ))}
        </ol>
      )}
      <p className="text-xs text-on-dark-faint">
        {frozen
          ? '封榜期间榜单不再更新,最终名次以赛后公布为准。'
          : live
            ? '有队伍得分时榜单会自动更新。'
            : '榜单不会自动更新,可以点下面的按钮手动刷新。'}
      </p>
    </section>
  )
}

/** readString 从开放对象里读字符串;缺失或类型不符回空串。 */
