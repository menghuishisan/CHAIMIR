import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { History, LoaderCircle, Send, Swords, Trophy, TriangleAlert } from 'lucide-react'
import {
  API_ERROR_CODES,
  ApiError,
  BattleRole,
  BattleRule,
  CONTEST_SUBMISSION_ERROR_CODES,
  ContestMode,
  ContestStatus,
  JudgeTaskStatusCode,
  type BattleEntry,
  type Contest,
  type ContestProblem,
  type ContestRecord,
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
  type BadgeTone,
  type WorkbenchShortcut,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { AppStatusScreen } from '../../../../components/AppStatusScreen'
import { ResourceError } from '../../../../components/ResourceState'
import {
  SandboxIdeWorkspace,
  type SandboxWorkspaceApi,
  type SandboxWorkspaceHandle,
} from '../../../sandbox/components/SandboxIdeWorkspace'
import { useWorkspaceActions } from '../../../sandbox/components/workspace/useWorkspaceActions'
import { useAsyncResource, usePagedResource, useTicketedWebSocket } from '../../../../hooks'
import type { AsyncResourceState } from '../../../../hooks/useAsyncResource'
import { useImmersive } from '../../../../layouts/immersive/context'
import { ContestCountdown } from '../../components/ContestCountdown'
import { readString } from '../../jsonReaders'
import { formatDateTime } from '../../../../utils/formatters'
import { battleRoleLabel, contestStatusLabel } from '../../../../utils/labels/contest'
import { errorDiagnostics, userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CONTEST_LADDER_PREVIEW_SIZE } from '../../queryLimits'
import { battleEntryRoles, isContestAnswerable, isContestLeaderboardFrozen } from '../../rules'

/** 竞赛工作区只通过 M8 授权网关访问沙箱,不复用 M2 租户内路径。 */
const contestWorkspaceApi: SandboxWorkspaceApi = {
  getInstance: (sandboxId) => api.contest.getSandboxInstance(sandboxId),
  getTerminalWsUrl: (sandboxId, container) =>
    api.contest.getSandboxTerminalWsUrl(sandboxId, container),
  getProgressWsUrl: (sandboxId) => api.contest.getSandboxProgressWsUrl(sandboxId),
  readFile: (sandboxId, path) => api.contest.readSandboxFile(sandboxId, path),
  listFiles: (sandboxId, path) => api.contest.listSandboxFiles(sandboxId, path),
  writeFile: (sandboxId, data) => api.contest.writeSandboxFile(sandboxId, data),
  saveFiles: (sandboxId) => api.contest.saveSandboxFiles(sandboxId),
  runCommandTool: (sandboxId, toolCode, data) =>
    api.contest.runSandboxCommandTool(sandboxId, toolCode, data),
  chainDeploy: (sandboxId, data) => api.contest.sandboxChainDeploy(sandboxId, data),
  chainSendTx: (sandboxId, data) => api.contest.sandboxChainSendTx(sandboxId, data),
  chainQuery: (sandboxId, runtimeInstance, target) =>
    api.contest.sandboxChainQuery(sandboxId, runtimeInstance, target),
  getToolProxyUrl: (sandboxId, toolCode, proxyPath, toolOrigin) =>
    api.contest.getSandboxToolProxyUrl(sandboxId, toolCode, proxyPath, toolOrigin),
}

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

  if (problems.status === 'error') {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="赛题暂时无法读取"
        description={problems.error?.message}
        traceId={problems.error?.traceId}
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={problems.reload}>
            重新读取
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

  return <ContestWorkbench key={contest.data.id} contest={contest.data} problems={problems.data} />
}

interface ContestWorkbenchProps {
  contest: Contest
  problems: ContestProblem[]
}

/**
 * 当前赛题写在地址的 problem 参数里:赛事详情的赛题行直接链到某一题,刷新后仍是这题;
 * 参数不属于本场赛题时回到第一题(地址可被手改,属外部输入)。
 */
function ContestWorkbench({ contest, problems }: ContestWorkbenchProps) {
  const [searchParams, setSearchParams] = useSearchParams()
  const requested = searchParams.get('problem')
  const problemId = problems.some((item) => item.id === requested)
    ? (requested as string)
    : problems[0].id
  const setProblemId = useCallback(
    (next: string) =>
      setSearchParams(
        (previous) => {
          const params = new URLSearchParams(previous)
          params.set('problem', next)
          return params
        },
        { replace: true }
      ),
    [setSearchParams]
  )
  const problem = useAsyncResource(
    () => api.contest.getProblem(contest.id, problemId),
    [contest.id, problemId]
  )

  if (problem.status === 'loading' || (problem.data && problem.data.id !== problemId)) {
    return <AppStatusScreen icon={LoaderCircle} spinning title="正在读取题面" fullScreen={false} />
  }

  if (problem.status === 'error' || !problem.data) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="题面暂时无法读取"
        description={problem.error?.message}
        traceId={problem.error?.traceId}
        fullScreen={false}
      />
    )
  }

  return (
    <ContestWorkbenchSession
      key={problem.data.id}
      contest={contest}
      problems={problems}
      problem={problem.data}
      onProblemChange={setProblemId}
    />
  )
}
interface ContestWorkbenchSessionProps {
  contest: Contest
  problems: ContestProblem[]
  problem: ContestProblem
  onProblemChange: (problemId: string) => void
}

function ContestWorkbenchSession({
  contest,
  problems,
  problem,
  onProblemChange,
}: ContestWorkbenchSessionProps) {
  const { title, exit } = useImmersive()
  const navigate = useNavigate()
  const spec = problemSpec(problem)

  const [sandboxId, setSandboxId] = useState<string>()
  const [sandboxReady, setSandboxReady] = useState(false)
  const [answer, setAnswer] = useState('')
  const [role, setRole] = useState<BattleRole | undefined>(() =>
    spec.battleRule === undefined ? undefined : battleEntryRoles(spec.battleRule)[0]
  )
  const [submission, setSubmission] = useState<ContestSubmission>()
  const { workspaceRef, busy, actionError, setActionError, perform } = useWorkspaceActions()

  const environmentRequestRef = useRef(0)
  const submissionRequestRef = useRef(0)
  const ownRecord = useAsyncResource(
    () =>
      contest.mode === ContestMode.SOLVE
        ? api.contest
            .getMyContestRecords()
            .then((records: ContestRecord[]) =>
              records.find((record) => record.contest_id === contest.id)
            )
        : Promise.resolve(undefined),
    [contest.id, contest.mode],
    () => false
  )
  const allowedBattleRoles = spec.battleRule === undefined ? [] : battleEntryRoles(spec.battleRule)
  const envRequestRef = `contest-problem:${problem.id}`
  const answerable = isContestAnswerable(contest.status)
  const submissionReady =
    spec.kind === 'environment'
      ? Boolean(sandboxId && sandboxReady)
      : spec.kind === 'answer'
        ? Boolean(spec.submitKey && answer.trim())
        : false

  const ladder = useAsyncResource(
    () =>
      api.contest.getLadder(contest.id, {
        page: 1,
        size: CONTEST_LADDER_PREVIEW_SIZE,
      }),
    [contest.id],
    () => false
  )
  const [ladderFeed, setLadderFeed] = useState<'waiting' | 'live' | 'invalid'>('waiting')

  // 推送只触发重读，不在浏览器重建排行榜。
  const ladderSocket = useTicketedWebSocket({
    url: contest.status === ContestStatus.RUNNING ? api.eventWebSocketUrl() : undefined,
    onOpen: () => {
      setLadderFeed('waiting')
      ladderSocket.send(
        JSON.stringify({ action: 'subscribe', topics: [contest.leaderboard_topic] })
      )
    },
    onMessage: (data) => {
      try {
        const frame: unknown = JSON.parse(data)
        if (!frame || typeof frame !== 'object' || Array.isArray(frame)) throw new Error('榜单推送格式不正确')
        if ('type' in frame && frame.type === 'subscribed') {
          if (!('topics' in frame) || !Array.isArray(frame.topics) || !frame.topics.includes(contest.leaderboard_topic)) {
            throw new Error('榜单订阅确认与请求主题不一致')
          }
          setLadderFeed('live')
          ladder.reload()
        } else if ('contest_id' in frame && frame.contest_id === contest.id) {
          ladder.reload()
        }
      } catch (cause) {
        console.error('[contest] 榜单推送读取失败', errorDiagnostics(cause))
        setLadderFeed('invalid')
      }
    },
  })

  const entries = usePagedResource<BattleEntry>(
    (params) =>
      spec.kind === 'battle'
        ? api.contest.listBattleEntries(contest.id, params)
        : Promise.resolve({ list: [], total: 0, page: params.page, size: params.size }),
    [contest.id, spec.kind]
  )

  // 页面刷新或关闭重开时先读取服务端仍存活的环境,不把浏览器本地状态当作权威。
  const restoreEnv = useCallback(async () => {
    if (!spec.hasEnvironment) return
    const request = ++environmentRequestRef.current
    try {
      const env = await api.contest.getEnv(contest.id, problem.id, envRequestRef)
      if (request !== environmentRequestRef.current) return
      setSandboxId(env.sandbox_id)
      setSandboxReady(false)
    } catch (error) {
      if (request !== environmentRequestRef.current) return
      if (error instanceof ApiError && error.code === API_ERROR_CODES.RESOURCE_NOT_FOUND) return
      setActionError(userFacingErrorMessage(error, '实操环境状态暂时读不到,请稍后重试。'))
    }
  }, [contest.id, envRequestRef, problem.id, setActionError, spec.hasEnvironment])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- effect starts an external request; state updates belong to its async callbacks
    void restoreEnv()
    return () => {
      environmentRequestRef.current += 1
      submissionRequestRef.current += 1
    }
  }, [restoreEnv])

  function createEnv() {
    if (!spec.hasEnvironment) return
    return perform(async () => {
      const request = ++environmentRequestRef.current
      const env = await api.contest.createEnv(contest.id, problem.id, {
        request_ref: envRequestRef,
      })
      if (request !== environmentRequestRef.current) return
      setSandboxId(env.sandbox_id)
      setSandboxReady(false)
      toast.success('实操环境已开始准备')
    })
  }

  function submitSolve() {
    if (!answerable) return
    if (!submissionReady) {
      setActionError(spec.kind === 'environment' ? '请先准备好实操环境。' : '请先填写答案。')
      return
    }
    return perform(async () => {
      submissionRequestRef.current += 1
      const contentRef: Record<string, unknown> =
        spec.submitKey && answer.trim() !== '' ? { [spec.submitKey]: answer.trim() } : {}
      const result = await api.contest.submitSolve(contest.id, problem.id, {
        content_ref: contentRef,
        sandbox_ref: spec.kind === 'environment' ? sandboxId : undefined,
      })
      submissionRequestRef.current += 1
      setSubmission(result)
      if (result.passed) {
        ladder.reload()
        ownRecord.reload()
      }
      toast.success('已提交')
    })
  }

  function submitBattle() {
    if (!answerable) return
    if (!sandboxId || !sandboxReady) {
      setActionError('请先准备好实操环境,再提交参战物。')
      return
    }
    if (role === undefined || !allowedBattleRoles.includes(role)) {
      setActionError('请选择符合题面要求的参战角色。')
      return
    }
    return perform(async () => {
      await api.contest.submitBattleEntry(contest.id, {
        problem_id: problem.id,
        sandbox_id: sandboxId,
        role,
      })
      toast.success('参战物已提交,等待匹配对局')
      entries.reload()
    })
  }

  /** refreshSubmission 读取服务端最近一次提交:刷新与关闭重开都恢复真实历史。 */
  const refreshSubmission = useCallback(async () => {
    if (spec.kind === 'battle' || spec.kind === 'undeclared') return
    const request = ++submissionRequestRef.current
    try {
      const latest = await api.contest.getLatestSubmission(contest.id, problem.id)
      if (request !== submissionRequestRef.current) return
      setSubmission(latest)
    } catch (error) {
      if (request !== submissionRequestRef.current) return
      if (error instanceof ApiError && error.code === CONTEST_SUBMISSION_ERROR_CODES.NOT_FOUND) {
        setSubmission(undefined)
        return
      }
      setActionError(userFacingErrorMessage(error, '判定结果暂时读不到,稍后再试一次。'))
    }
  }, [contest.id, problem.id, setActionError, spec.kind])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- effect starts an external request; state updates belong to its async callbacks
    void refreshSubmission()
  }, [refreshSubmission])

  // 主操作的快捷键(设计 §7.3):与顶栏按钮同一个动作、同样的可用条件;提交中不重复发出
  const submitShortcuts: WorkbenchShortcut[] =
    spec.kind === 'battle'
      ? [
          {
            key: 'Enter',
            mod: true,
            description: '提交参战',
            disabled:
              busy ||
              !answerable ||
              !sandboxReady ||
              role === undefined ||
              !allowedBattleRoles.includes(role),
            run: () => void submitBattle(),
          },
        ]
      : [
          {
            key: 'Enter',
            mod: true,
            description: '提交答案',
            disabled: busy || !answerable || !submissionReady,
            run: () => void submitSolve(),
          },
        ]

  return (
    <WorkbenchShell
      workbench="contest"
      topbar={
        <WorkbenchTopbar
          onExit={() => void perform(exit, true)}
          exitLabel="退出赛场"
          shortcuts={submitShortcuts}
          title={title}
          subtitle={contest.name}
          progress={
            contest.mode !== ContestMode.SOLVE ? undefined : ownRecord.status === 'success' &&
              ownRecord.data ? (
              <ChainProgress
                onDark
                size="sm"
                label="已解题数"
                total={problems.length}
                done={Math.min(ownRecord.data.solved_count, problems.length)}
              />
            ) : ownRecord.status === 'error' ? (
              <span className="text-xs text-on-dark-danger">
                个人解题进度暂时读不到
                {ownRecord.error?.traceId ? ` · 报障编号 ${ownRecord.error.traceId}` : ''}
              </span>
            ) : undefined
          }
          cta={
            <div className="flex items-center gap-3">
              {answerable ? <ContestCountdown endAt={contest.end_at} /> : null}
              {spec.kind === 'battle' ? (
                <Button
                  variant="seal"
                  size="sm"
                  leftIcon={Swords}
                  loading={busy}
                  disabled={
                    !answerable ||
                    !sandboxReady ||
                    role === undefined ||
                    !allowedBattleRoles.includes(role)
                  }
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
                  disabled={!answerable || !submissionReady}
                  onClick={() => void submitSolve()}
                >
                  提交答案
                </Button>
              )}
            </div>
          }
        />
      }
      left={
        <ProblemBrief
          problems={problems}
          currentId={problem.id}
          onPick={(id) => {
            if (id !== problem.id) void perform(() => onProblemChange(id), true)
          }}
          spec={spec}
        />
      }
      leftLabel="赛题"
      stage={
        <ProblemStage
          workspaceRef={workspaceRef}
          spec={spec}
          sandboxId={sandboxId}
          busy={busy}
          answerable={answerable}
          answer={answer}
          onAnswerChange={setAnswer}
          onCreateEnv={() => void createEnv()}
          onReadinessChange={setSandboxReady}
        />
      }
      right={
        // 壳不给槽位套滚动(主舞台永不滚动,规范 §7.1),这一栏的两块内容自己滚
        <div className="flex h-full min-h-0 flex-col overflow-y-auto">
          {spec.kind === 'battle' ? (
            <BattlePanel
              role={role}
              allowedRoles={allowedBattleRoles}
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
              onOpenReplay={() =>
                void perform(() => navigate(`/student/contests/${contest.id}/replay`), true)
              }
            />
          ) : (
            <SubmissionPanel submission={submission} onRefresh={() => void refreshSubmission()} />
          )}
          <LadderPanel
            resource={ladder}
            mode={contest.mode}
            live={ladderSocket.status === 'open' && ladderFeed === 'live'}
            feedError={ladderFeed === 'invalid' ? '榜单自动更新暂不可用，请重新连接。' : ladderSocket.error ?? (ladderSocket.status === 'closed' ? '榜单自动更新已断开。' : undefined)}
            onReconnect={ladderSocket.reconnect}
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
              {spec.kind === 'answer' && answer.trim() ? <span>答案已填写</span> : null}
            </span>
            {actionError ? (
              <span className="mt-0.5 text-xs text-on-dark-danger">{actionError}</span>
            ) : null}
            {!answerable ? (
              <span className="mt-0.5 text-xs text-on-dark-sub">
                当前不在作答时间内,只能查看题面与榜单。
              </span>
            ) : null}
            {answerable && !submissionReady && spec.kind === 'environment' ? (
              <span className="mt-0.5 text-xs text-on-dark-sub">
                实操环境准备好后即可提交,平台会在提交时保存当前工作区。
              </span>
            ) : null}
            {answerable && !submissionReady && spec.kind === 'answer' ? (
              <span className="mt-0.5 text-xs text-on-dark-sub">填写答案后才能提交。</span>
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

interface ProblemSpec {
  kind: 'environment' | 'answer' | 'battle' | 'undeclared'
  title: string
  scenario: string
  statement: string
  hasEnvironment: boolean
  runtimeName: string
  submitKey?: string
  battleRule?: BattleRule
}

// 答案键与实操环境独立；学生端不拼装组合或读取私有判题配置。
function problemSpec(problem: ContestProblem): ProblemSpec {
  const face = problem.face ?? {}
  const runtimeName = readCompositionRuntime(face)
  const base = {
    title: readString(face, 'title') || `第 ${problem.seq} 题`,
    scenario: readString(face, 'scenario'),
    statement: readString(face, 'statement'),
    hasEnvironment: runtimeName !== '',
    runtimeName,
  }

  if (problem.battle_rule !== undefined) {
    return { ...base, kind: 'battle', battleRule: problem.battle_rule }
  }

  const submitKey = readString(face, 'submit_key').trim()
  return {
    ...base,
    kind: submitKey !== '' ? 'answer' : runtimeName !== '' ? 'environment' : 'undeclared',
    submitKey: submitKey || undefined,
  }
}

function readCompositionRuntime(face: Record<string, unknown>): string {
  const composition = face.composition
  if (typeof composition !== 'object' || composition === null) return ''
  const runtimes = (composition as Record<string, unknown>).runtimes
  if (!Array.isArray(runtimes)) return ''
  return runtimes
    .filter((item): item is Record<string, unknown> => typeof item === 'object' && item !== null)
    .map((item) => readString(item, 'runtime_code'))
    .filter(Boolean)
    .join(' / ')
}

interface ProblemBriefProps {
  problems: ContestProblem[]
  currentId: string
  onPick: (problemId: string) => void
  spec: ProblemSpec
}

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
                  {item.title || `第 ${item.seq} 题`}
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
  workspaceRef: React.Ref<SandboxWorkspaceHandle>
  spec: ProblemSpec
  sandboxId?: string
  busy: boolean
  answerable: boolean
  answer: string
  onAnswerChange: (value: string) => void
  onCreateEnv: () => void
  onReadinessChange: (ready: boolean) => void
}

function ProblemStage({
  workspaceRef,
  spec,
  sandboxId,
  busy,
  answerable,
  answer,
  onAnswerChange,
  onCreateEnv,
  onReadinessChange,
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

  return (
    <div className="flex h-full min-h-0 flex-col">
      {spec.kind === 'answer' ? (
        <div className="flex shrink-0 flex-col gap-3 p-4">
          <label className="flex flex-col gap-1">
            <span className="text-sm text-on-dark">你的答案</span>
            <Input
              variant="dark"
              value={answer}
              disabled={!answerable || busy}
              placeholder="按题面要求填写"
              className="font-mono text-sm"
              onChange={(event) => onAnswerChange(event.target.value)}
            />
          </label>
          <p className="text-xs text-on-dark-sub">
            填好后点右上角「提交答案」。同一题连续提交有冷却时间,判错也会短暂锁一会儿。
          </p>
        </div>
      ) : null}
      {spec.hasEnvironment || spec.kind !== 'answer' ? (
        sandboxId ? (
          <div className="min-h-0 flex-1">
            <SandboxIdeWorkspace
              ref={workspaceRef}
              sandboxId={sandboxId}
              workspaceApi={contestWorkspaceApi}
              onReadinessChange={onReadinessChange}
              onRebuild={onCreateEnv}
              rebuilding={busy}
            />
          </div>
        ) : (
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 overflow-y-auto p-6 text-center">
            <p className="text-base text-on-dark">
              {spec.kind === 'battle' ? '准备参战物需要一个实操环境' : '这道题要在实操环境里完成'}
            </p>
            <p className="max-w-md text-sm text-on-dark-sub">
              环境按题目锁定版本里的声明准备,起好后可以写代码、开终端、做链上操作。
              {spec.runtimeName ? `这道题用的是 ${spec.runtimeName}。` : ''}
            </p>
            <Button
              variant="primary"
              loading={busy}
              disabled={!answerable || !spec.hasEnvironment}
              onClick={onCreateEnv}
            >
              准备实操环境
            </Button>
            {!spec.hasEnvironment ? (
              <p className="text-xs text-on-dark-danger">
                这道题的题目版本里没有声明实操环境,暂时起不了。请把题号告诉老师。
              </p>
            ) : null}
          </div>
        )
      ) : null}
    </div>
  )
}

function SubmissionPanel({
  submission,
  onRefresh,
}: {
  submission?: ContestSubmission
  onRefresh: () => void
}) {
  const state =
    submission &&
    (
      {
        [JudgeTaskStatusCode.QUEUED]: { label: '排队中', tone: 'neutral' },
        [JudgeTaskStatusCode.JUDGING]: { label: '评测中', tone: 'info' },
        [JudgeTaskStatusCode.DONE]: {
          label: submission.passed ? '已通过' : '未通过',
          tone: submission.passed ? 'success' : 'warning',
        },
        [JudgeTaskStatusCode.FAILED]: { label: '评测未完成', tone: 'danger' },
        [JudgeTaskStatusCode.CANCELLED]: { label: '已取消', tone: 'neutral' },
      } satisfies Record<JudgeTaskStatusCode, { label: string; tone: BadgeTone }>
    )[submission.judge_status]

  return (
    <section className="flex flex-col gap-2 border-b border-dark-line p-4">
      <h2 className="text-sm font-medium text-on-dark">提交结果</h2>
      {submission && state ? (
        <div className="flex flex-col gap-2 rounded-md border border-dark-line bg-dark-surface p-2">
          <div className="flex items-center justify-between gap-2">
            <Badge onDark tone={state.tone}>
              {state.label}
            </Badge>
            {submission.judge_status === JudgeTaskStatusCode.DONE ? (
              <span className="font-mono text-xs tabular-nums text-on-dark-sub">
                {submission.score === null ? '得分结算中' : `${submission.score} 分`}
              </span>
            ) : null}
          </div>
          {submission.judge_status === JudgeTaskStatusCode.FAILED ||
          submission.judge_status === JudgeTaskStatusCode.CANCELLED ? (
            <p className="text-xs text-on-dark-sub">
              这次评测未能完成，请重新提交；不会按答错处理。
            </p>
          ) : null}
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
  role?: BattleRole
  allowedRoles: BattleRole[]
  onRoleChange: (role: BattleRole) => void
  entries: BattleEntry[]
  entryStatus: 'loading' | 'success' | 'empty' | 'error'
  entryError?: string
  entryTraceId?: string
  onEntryRetry: () => void
  entryPage: number
  entryPageSize: number
  entryTotal: number | undefined
  onEntryPageChange: (page: number) => void
  onOpenReplay: () => void
}

/**
 * 只有最新一版参战物生效(后端按版本号取 is_active),历史版本留档便于对照。
 */
function BattlePanel({
  role,
  allowedRoles,
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
          {allowedRoles.map((item) => (
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
          <ResourceError onDark message={entryError ?? '参战历史暂时读不到,请稍后重试。'} traceId={entryTraceId} onRetry={onEntryRetry} />
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
  resource: AsyncResourceState<{ list: LadderRank[] }>
  mode: ContestMode
  /** 榜单实时通道是否已连上;没连上时说明需要手动刷新 */
  live: boolean
  feedError?: string
  onReconnect: () => void
  /** 封榜期:榜单不再更新,当前名次可能与最终结果不同 */
  frozen: boolean
}

/**
 * 队伍只显示名次与分数:队伍编号是内部标识,榜单上显示它既不可读也没有意义。
 */
function LadderPanel({ resource, mode, live, feedError, onReconnect, frozen }: LadderPanelProps) {
  const ranks = resource.data?.list ?? []
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
      {resource.status === 'error' ? (
        <ResourceError onDark message={resource.error?.message ?? '榜单暂时无法读取，请重试。'} traceId={resource.error?.traceId} onRetry={resource.reload} />
      ) : resource.status === 'loading' ? (
        <p role="status" className="text-xs text-on-dark-sub">正在读取榜单...</p>
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
              <span className="font-mono text-xs tabular-nums text-on-dark">
                {mode === ContestMode.SOLVE
                  ? `${rank.score} 分 · 通过 ${rank.solved_count} 题`
                  : `对抗积分 ${rank.score}`}
              </span>
            </li>
          ))}
        </ol>
      )}
      {feedError ? (
        <div role="status" className="flex flex-col items-start gap-2">
          <p className="text-xs text-on-dark-sub">{feedError}</p>
          <Button variant="on-dark" size="sm" onClick={onReconnect}>重新连接</Button>
        </div>
      ) : null}
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
