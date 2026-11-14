// 实时监控页(教师侧栏,/teacher/monitoring)。设计 §7.4:
// 页头右侧「刷新」与「更新于 N 秒前」;最上方「需要处理」白卡,把判题失败的任务与准备失败的实验环境
// 合并成一个列表(来源、对象、问题、处理动作),没有时显示「目前一切正常」;下方「判题任务 / 实验环境 /
// 竞赛对局」三个标签,标签上带进行中数量,各自是状态筛选 + 表格;标签与筛选都写进地址。
//
// 数据全部是真实状态(M3 判题任务、M7 实例列表、M8 对局记录),不做装饰性雷达或假飞线。
// 刷新是显式动作:高频路径不自动轮询(规范 §4.1),由教师按需刷新,处理完一项后自动刷新。
// 对局按竞赛查看:「竞赛对局」标签里先选竞赛,再按「全部 / 进行中 / 失败」分段(服务端筛选,数字取 facets);
// 失败对局不并入「需要处理」:那要为每场进行中的对抗赛各发一次请求,改在标签里的「失败」分段查看。

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router'
import { CircleCheck, FlaskConical, RefreshCw, RotateCcw, Swords } from 'lucide-react'
import {
  TENANT_MODULE,
  BattleMatchStatus,
  ContestMode,
  ContestStatus,
  ExperimentInstanceStatus,
  JUDGE_TASK_STATE,
  PAGINATION_MAX_SIZE,
  type BattleMatch,
  type Contest,
  type Experiment,
  type ExperimentInstance,
  type JudgeTask,
  type JudgeTaskState,
  type PaginatedResponse,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  buttonVariants,
  Callout,
  DataPanel,
  DescriptionList,
  Empty,
  FilterBar,
  FilterField,
  Icon,
  IconButton,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  SegmentedControl,
  Select,
  Skeleton,
  StatusIndicator,
  Table,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useSession } from '../../../../components/RoleGuard/session'
import { ResourceState } from '../../../../components/ResourceState'
import { isJudgeTaskAbnormal, isJudgeTaskActive } from '../../../judge/rules'
import { judgeTaskStatusTone } from '../../../judge/statusPresentation'
import { useRejudgeTask } from '../../../judge/useRejudgeTask'
import { useAsyncResource, useUrlState, type AsyncResourceState } from '../../../../hooks'
import { facetCount, segmentCounts } from '../../../../utils/facets'
import { collectAllPages } from '../../../../utils/allPages'
import { formatDateTime, formatRelativeOccurrence, formatShortDateTime } from '../../../../utils/formatters'
import { battleMatchStatusLabel, battleResultLabel, contestStatusLabel } from '../../../../utils/labels/contest'
import { experimentInstanceStatusLabel } from '../../../../utils/labels/experiment'
import { judgeTaskStatusLabel } from '../../../../utils/labels/judge'
import { battleMatchStatusTone } from '../../../contest/statusPresentation'
import { experimentInstanceStatusTone } from '../../../experiment/statusPresentation'
import { StudentName } from '../../components/StudentName'
import { useCourseMemberNames, type MemberName } from '../../useCourseMemberNames'

/** 每张表一次取回的条数:监控看的是「当下有没有问题」,不是全量历史。 */
const MONITOR_SIZE = 30

/** 地址参数:tab 下方标签;judge、env、battle 三张表的状态分段;contest 对局标签里选中的竞赛(空为第一场) */
const MONITOR_QUERY = {
  tab: { defaultValue: 'judge', allowed: ['judge', 'experiment', 'battle'] },
  judge: { defaultValue: 'active', allowed: ['active', 'failed', 'all'] },
  env: { defaultValue: 'running', allowed: ['running', 'paused', 'failed'] },
  battle: { defaultValue: 'all', allowed: ['all', 'running', 'failed'] },
  contest: { defaultValue: '' },
} as const

const JUDGE_SEGMENTS = [
  { value: 'active', label: '进行中' },
  { value: 'failed', label: '判题失败' },
  { value: 'all', label: '全部' },
] as const

/** 对局分段:全部、进行中、失败;取值对应服务端 status 参数 */
const BATTLE_SEGMENTS = [
  { value: 'all', label: '全部' },
  { value: 'running', label: '进行中', status: BattleMatchStatus.RUNNING },
  { value: 'failed', label: '失败', status: BattleMatchStatus.FAILED },
] as const

const ENV_SEGMENTS = [
  { value: 'running', label: '进行中' },
  { value: 'paused', label: '已暂停' },
  { value: 'failed', label: '准备失败' },
] as const

/** judgeStateOf 判题分段对应的服务端 state 参数;「全部」不筛 */
function judgeStateOf(segment: string): JudgeTaskState | undefined {
  if (segment === 'active') return JUDGE_TASK_STATE.ACTIVE
  if (segment === 'failed') return JUDGE_TASK_STATE.ABNORMAL
  return undefined
}

/** envStatusOf 实验环境分段对应的实例状态 */
function envStatusOf(segment: string): ExperimentInstanceStatus {
  if (segment === 'paused') return ExperimentInstanceStatus.PAUSED
  if (segment === 'failed') return ExperimentInstanceStatus.ERROR
  return ExperimentInstanceStatus.RUNNING
}

/** emptyPage 模块未开放时不发请求,直接给一个空页 */
function emptyPage<T>(): PaginatedResponse<T> {
  return { list: [], total: 0, page: 1, size: MONITOR_SIZE }
}

/** capNote 列表只取了最近一批时说清楺共有多少;全部列出时不显示 */
function capNote(page: PaginatedResponse<unknown> | null | undefined, unit: string): ReactNode {
  if (!page || page.total <= page.list.length) return undefined
  return (
    <p className="text-xs text-ink-sub">
      只列出最近的 {page.list.length} {unit},共 {page.total} {unit}。
    </p>
  )
}

/**
 * TeacherMonitoringPage 汇总判题、实验与对局的实时状态。
 * 学校没有开放任何会产生这些状态的功能时只给一句说明,不发请求。
 */
export default function TeacherMonitoringPage() {
  const { me } = useSession()
  const experimentsEnabled = me.enabled_modules.includes(TENANT_MODULE.EXPERIMENT)
  const contestsEnabled = me.enabled_modules.includes(TENANT_MODULE.CONTEST)
  const judgeEnabled =
    experimentsEnabled || contestsEnabled || me.enabled_modules.includes(TENANT_MODULE.TEACHING)

  if (!judgeEnabled) {
    return (
      <PageScaffold>
        <PageHeader title="实时监控" />
        <Callout tone="info">学校尚未开放可监控的教学、实验或竞赛功能。</Callout>
      </PageScaffold>
    )
  }
  return <MonitoringBoard experimentsEnabled={experimentsEnabled} contestsEnabled={contestsEnabled} />
}

interface MonitoringBoardProps {
  experimentsEnabled: boolean
  contestsEnabled: boolean
}

/**
 * MonitoringBoard 页面主体。「需要处理」与两张表「失败」分段上的数字共用同一次读取;
 * 标签与分段上的其余数字只取服务端总数(全量口径),不随当前打开的标签变化。
 */
function MonitoringBoard({ experimentsEnabled, contestsEnabled }: MonitoringBoardProps) {
  const [query, setQuery] = useUrlState(MONITOR_QUERY)
  const [refreshVersion, setRefreshVersion] = useState(0)
  const [refreshedAt, setRefreshedAt] = useState(() => Date.now())
  const [detailTask, setDetailTask] = useState<JudgeTask>()

  /** refresh 重新读取整页;处理完一项后也走这里,数字与列表一起更新 */
  const refresh = useCallback(() => {
    setRefreshVersion((value) => value + 1)
    setRefreshedAt(Date.now())
  }, [])

  const failedTasks = useAsyncResource(
    () => api.judge.getTasks({ state: JUDGE_TASK_STATE.ABNORMAL, page: 1, size: MONITOR_SIZE }),
    [refreshVersion],
    () => false,
  )
  const failedInstances = useAsyncResource(
    () =>
      experimentsEnabled
        ? api.experiment.listInstances({ status: ExperimentInstanceStatus.ERROR, page: 1, size: MONITOR_SIZE })
        : Promise.resolve(emptyPage<ExperimentInstance>()),
    [experimentsEnabled, refreshVersion],
    () => false,
  )
  // 标签与分段上的数字取上面两个「需要处理」列表一并返回的 facets(判题按运维分组 state,环境按 status),
  // 它们是全量口径且不套用本组筛选,所以不必为每个数字另发一次只取总数的请求
  const taskFacets = failedTasks.status === 'success' ? failedTasks.data?.facets : undefined
  const instanceFacets = failedInstances.status === 'success' ? failedInstances.data?.facets : undefined
  const activeTaskCount = facetCount(taskFacets, 'state', JUDGE_TASK_STATE.ACTIVE)
  const runningCount = experimentsEnabled ? facetCount(instanceFacets, 'status', ExperimentInstanceStatus.RUNNING) : undefined
  const pausedCount = experimentsEnabled ? facetCount(instanceFacets, 'status', ExperimentInstanceStatus.PAUSED) : undefined
  // 进行中(含封榜)的对抗赛:标签上的数字与对局标签里的竞赛选择。数量本来就小,按页取完;
  // 解题赛没有对局,不列进来
  const battleContests = useAsyncResource<Contest[]>(
    () =>
      contestsEnabled
        ? Promise.all(
            [ContestStatus.RUNNING, ContestStatus.FROZEN].map((status) =>
              collectAllPages((params) => api.contest.getContests({ status, ...params }), PAGINATION_MAX_SIZE),
            ),
          ).then((lists) => lists.flat().filter((contest) => contest.mode === ContestMode.BATTLE))
        : Promise.resolve([]),
    [contestsEnabled, refreshVersion],
    () => false,
  )
  const labels = useInstanceLabels(experimentsEnabled)

  const tab =
    (query.tab === 'experiment' && !experimentsEnabled) || (query.tab === 'battle' && !contestsEnabled)
      ? 'judge'
      : query.tab
  const failedTaskCount = facetCount(taskFacets, 'state', JUDGE_TASK_STATE.ABNORMAL)
  const failedInstanceCount = experimentsEnabled ? facetCount(instanceFacets, 'status', ExperimentInstanceStatus.ERROR) : undefined

  return (
    <PageScaffold>
      <PageHeader
        title="实时监控"
        actions={
          <div className="flex items-center gap-3">
            <RefreshedAgo at={refreshedAt} />
            <Button variant="outline" leftIcon={RefreshCw} loading={failedTasks.status === 'loading'} onClick={refresh}>
              刷新
            </Button>
          </div>
        }
      />

      <AttentionCard
        failedTasks={failedTasks}
        failedInstances={failedInstances}
        experimentsEnabled={experimentsEnabled}
        labels={labels}
        onHandled={refresh}
        onOpenTask={setDetailTask}
      />

      <Tabs value={tab} onValueChange={(value) => setQuery({ tab: value })}>
        <TabsList aria-label="监控来源">
          <TabsTrigger value="judge" count={activeTaskCount}>
            判题任务
          </TabsTrigger>
          {experimentsEnabled ? (
            <TabsTrigger value="experiment" count={runningCount}>
              实验环境
            </TabsTrigger>
          ) : null}
          {contestsEnabled ? (
            <TabsTrigger
              value="battle"
              count={battleContests.status === 'success' ? battleContests.data?.length : undefined}
            >
              竞赛对局
            </TabsTrigger>
          ) : null}
        </TabsList>
        <TabsContent value="judge">
          <JudgeTab
            segment={query.judge}
            onSegmentChange={(value) => setQuery({ judge: value })}
            counts={{ active: activeTaskCount, failed: failedTaskCount }}
            refreshVersion={refreshVersion}
            onChanged={refresh}
            onOpenTask={setDetailTask}
          />
        </TabsContent>
        {experimentsEnabled ? (
          <TabsContent value="experiment">
            <ExperimentTab
              segment={query.env}
              onSegmentChange={(value) => setQuery({ env: value })}
              counts={{ running: runningCount, paused: pausedCount, failed: failedInstanceCount }}
              refreshVersion={refreshVersion}
              labels={labels}
            />
          </TabsContent>
        ) : null}
        {contestsEnabled ? (
          <TabsContent value="battle">
            <BattleTab
              contests={battleContests}
              contestId={query.contest}
              onContestChange={(value) => setQuery({ contest: value })}
              segment={query.battle}
              onSegmentChange={(value) => setQuery({ battle: value })}
              refreshVersion={refreshVersion}
            />
          </TabsContent>
        ) : null}
      </Tabs>

      {detailTask ? <JudgeTaskDetailModal task={detailTask} onClose={() => setDetailTask(undefined)} /> : null}
    </PageScaffold>
  )
}

/** RefreshedAgo 「更新于 N 秒前」;每 10 秒重算一次文字,不重新读数据 */
function RefreshedAgo({ at }: { at: number }) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 10_000)
    return () => window.clearInterval(timer)
  }, [])
  const seconds = Math.max(0, Math.floor((now - at) / 1000))
  const text =
    seconds < 10 ? '刚刚更新' : seconds < 60 ? `更新于 ${seconds} 秒前` : `更新于 ${Math.floor(seconds / 60)} 分钟前`
  return <span className="text-sm text-ink-sub">{text}</span>
}

interface InstanceLabels {
  /** experimentName 实验名;读取中为 undefined */
  experimentName: (experimentId: string) => string | undefined
  /** student 开这个环境的学生;实验不属于课程时为 undefined(没有可查的成员名单) */
  student: (instance: ExperimentInstance) => MemberName | undefined
}

/**
 * useInstanceLabels 把实验环境上的实验编号、学生编号换成实验名与姓名学号,不显示编号。
 * 教师只看得到自己实验的环境,所以取教师自己的实验(数量小,按页取完),再取这些实验所属课程的成员。
 * 只在进入页面时读一次:实验与成员不是「实时状态」,不跟着刷新重读。
 */
function useInstanceLabels(enabled: boolean): InstanceLabels {
  const experiments = useAsyncResource<Experiment[]>(
    () =>
      enabled
        ? collectAllPages((params) => api.experiment.getExperiments(params), PAGINATION_MAX_SIZE)
        : Promise.resolve([]),
    [enabled],
    () => false,
  )
  const byId = useMemo(
    () => new Map((experiments.data ?? []).map((item) => [item.id, item])),
    [experiments.data],
  )
  const courseIds = useMemo(() => (experiments.data ?? []).map((item) => item.course_id), [experiments.data])
  const { lookup } = useCourseMemberNames(courseIds)
  const status = experiments.status

  return useMemo(
    () => ({
      experimentName: (experimentId: string) => {
        if (status === 'loading') return undefined
        if (status === 'error') return '实验名暂时读不到'
        return byId.get(experimentId)?.name ?? '其他实验'
      },
      student: (instance: ExperimentInstance): MemberName | undefined => {
        if (status === 'loading') return { state: 'loading' }
        if (status === 'error') return { state: 'failed' }
        return byId.get(instance.experiment_id)?.course_id ? lookup(instance.owner_account_id) : undefined
      },
    }),
    [byId, lookup, status],
  )
}

/** InstanceObject 一个实验环境是谁的:实验名 · 学生(小组实验标注小组) */
function InstanceObject({ instance, labels }: { instance: ExperimentInstance; labels: InstanceLabels }) {
  const name = labels.experimentName(instance.experiment_id)
  const student = labels.student(instance)
  return (
    <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
      {name === undefined ? <Skeleton variant="line" width="half" /> : <span className="font-medium text-ink">{name}</span>}
      {student ? <StudentName member={student} compact /> : null}
      {instance.group_id ? <span className="text-xs text-ink-sub">小组</span> : null}
    </span>
  )
}

interface AttentionCardProps {
  failedTasks: AsyncResourceState<PaginatedResponse<JudgeTask>>
  failedInstances: AsyncResourceState<PaginatedResponse<ExperimentInstance>>
  experimentsEnabled: boolean
  labels: InstanceLabels
  onHandled: () => void
  onOpenTask: (task: JudgeTask) => void
}

/**
 * AttentionCard 「需要处理」:判题失败的任务(动作是重判)与准备失败的实验环境合并成一个列表。
 * 实验环境只能由学生本人在工作台里重建(教师无权操作别人的环境),所以这一行给的是提醒学生怎么做。
 * 只在两类都读到且都为零时才说「目前一切正常」,读不到的部分单独说明。
 */
function AttentionCard({
  failedTasks,
  failedInstances,
  experimentsEnabled,
  labels,
  onHandled,
  onOpenTask,
}: AttentionCardProps) {
  const { rejudge, rejudgingId, actionError } = useRejudgeTask(onHandled)
  const loading = failedTasks.status === 'loading' || failedInstances.status === 'loading'
  const partlyUnread = failedTasks.status === 'error' || failedInstances.status === 'error'
  const taskPage = failedTasks.status === 'success' ? failedTasks.data : null
  const instancePage = failedInstances.status === 'success' ? failedInstances.data : null
  const tasks = taskPage?.list ?? []
  const instances = instancePage?.list ?? []
  const total = (taskPage?.total ?? 0) + (instancePage?.total ?? 0)
  const listed = tasks.length + instances.length

  return (
    <section aria-labelledby="attention-title" className="mb-8 rounded-lg bg-surface p-5 shadow-xs">
      <h2 id="attention-title" className="flex items-baseline gap-2 text-md font-semibold text-ink">
        需要处理
        {!loading && total > 0 ? <span className="text-sm font-normal tabular-nums text-danger">{total} 项</span> : null}
      </h2>
      {actionError ? (
        <Callout tone="danger" className="mt-3">
          {actionError}
        </Callout>
      ) : null}
      {!loading && partlyUnread ? (
        <Callout tone="warning" className="mt-3">
          有一部分状态暂时读不到,点右上角「刷新」再试。
        </Callout>
      ) : null}
      {loading ? (
        <div aria-busy="true" className="mt-3">
          <Skeleton variant="line" lines={3} />
        </div>
      ) : total === 0 ? (
        partlyUnread ? null : (
          <p className="mt-3 flex items-center gap-2 text-sm text-ink-sub">
            <Icon icon={CircleCheck} size="sm" className="text-success" />
            {experimentsEnabled
              ? '目前一切正常:没有判题失败的任务,也没有准备失败的实验环境。'
              : '目前一切正常:没有判题失败的任务。'}
          </p>
        )
      ) : (
        <>
          <ul className="mt-2 divide-y divide-line">
            {tasks.map((task) => (
              <AttentionRow
                key={task.task_id}
                source="判题"
                object={
                  <button
                    type="button"
                    onClick={() => onOpenTask(task)}
                    className="rounded-sm text-left underline-offset-2 hover:text-primary hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                  >
                    {judgeSourceLabel(task.source_ref)}
                  </button>
                }
                problem={judgeTaskStatusLabel(task.status)}
                action={
                  <Button
                    variant="secondary"
                    size="sm"
                    leftIcon={RotateCcw}
                    loading={rejudgingId === task.task_id}
                    onClick={() => void rejudge(task.task_id)}
                  >
                    重判
                  </Button>
                }
              />
            ))}
            {instances.map((instance) => (
              <AttentionRow
                key={instance.instance_id}
                source="实验环境"
                object={<InstanceObject instance={instance} labels={labels} />}
                problem={experimentInstanceStatusLabel(instance.status)}
                action={<span className="text-sm text-ink-sub">请学生在实验里点「重建实验环境」</span>}
              />
            ))}
          </ul>
          {total > listed ? (
            <p className="mt-2 text-xs text-ink-sub">
              只列出了最近的 {listed} 项,共 {total} 项。
            </p>
          ) : null}
        </>
      )}
    </section>
  )
}

/** AttentionRow 「需要处理」里的一行:来源、对象、问题、处理动作;窄屏时自动换行 */
function AttentionRow({
  source,
  object,
  problem,
  action,
}: {
  source: string
  object: ReactNode
  problem: string
  action: ReactNode
}) {
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-1.5 py-3">
      <span className="w-18 shrink-0">
        <Badge tone="neutral">{source}</Badge>
      </span>
      <span className="min-w-0 flex-1 text-sm text-ink">{object}</span>
      <span className="text-sm text-danger">{problem}</span>
      <span className="shrink-0">{action}</span>
    </li>
  )
}

interface JudgeTabProps {
  segment: string
  onSegmentChange: (value: string) => void
  /** 各分段的全量计数;读取中为 undefined,「全部」不带数字 */
  counts: { active?: number; failed?: number }
  refreshVersion: number
  onChanged: () => void
  onOpenTask: (task: JudgeTask) => void
}

/**
 * JudgeTab 判题任务:状态分段(进行中 / 判题失败 / 全部)+ 表格;整行打开逐项判定,失败的可以重判。
 * 筛选交给服务端:在本机对最近 30 条再筛一次,会在这 30 条里恰好没有失败任务时
 * 显示「没有判题失败的任务」,而队列里可能积压着更早的失败任务。
 */
function JudgeTab({ segment, onSegmentChange, counts, refreshVersion, onChanged, onOpenTask }: JudgeTabProps) {
  const tasks = useAsyncResource(
    () => api.judge.getTasks({ state: judgeStateOf(segment), page: 1, size: MONITOR_SIZE }),
    [segment, refreshVersion],
    () => false,
  )
  const { rejudge, rejudgingId, actionError } = useRejudgeTask(onChanged)

  const columns: TableColumn<JudgeTask>[] = [
    {
      key: 'source_ref',
      header: '来源',
      primary: true,
      // source_ref 格式是 <来源>:<年份>:<资源类型>:<id>,取来源段显示业务归属,不直出全串
      render: (task) => <span className="text-ink">{judgeSourceLabel(task.source_ref)}</span>,
    },
    {
      key: 'status',
      header: '状态',
      render: (task) => (
        <StatusIndicator
          tone={judgeTaskStatusTone(task.status)}
          label={judgeTaskStatusLabel(task.status)}
          loading={isJudgeTaskActive(task.status)}
        />
      ),
    },
    {
      key: 'result',
      header: '判题结果',
      render: (task) =>
        task.result ? (
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge tone={task.result.passed ? 'success' : 'danger'}>{task.result.passed ? '通过' : '未通过'}</Badge>
            <span className="font-mono text-xs tabular-nums text-ink-sub">
              {task.result.score} / {task.result.max_score}
            </span>
          </div>
        ) : (
          <span className="text-ink-sub">尚无结果</span>
        ),
    },
    {
      key: 'detail',
      header: '说明',
      render: (task) =>
        task.result && task.result.details.length > 0 ? (
          <span className="line-clamp-1 text-xs text-ink-sub">
            {task.result.details[0].hint ??
              task.result.details[0].expected_label ??
              task.result.details[0].case ??
              '点开看逐项判定'}
          </span>
        ) : (
          <span className="text-ink-sub">—</span>
        ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (task) =>
        isJudgeTaskAbnormal(task.status) ? (
          <Button
            variant="ghost"
            size="sm"
            leftIcon={RotateCcw}
            loading={rejudgingId === task.task_id}
            onClick={() => void rejudge(task.task_id)}
          >
            重判
          </Button>
        ) : null,
    },
  ]

  const emptyText =
    segment === 'active'
      ? '现在没有排队或判题中的任务。'
      : segment === 'failed'
        ? '没有判题失败的任务。'
        : '还没有判题任务。学生提交编程题、实验检查点或竞赛答题后会出现在这里。'

  return (
    <>
      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}
      <DataPanel
        label="判题任务"
        filter={
          <FilterBar label="判题任务筛选">
            <FilterField label="状态" group>
              <SegmentedControl
                aria-label="按判题状态筛选"
                size="sm"
                options={JUDGE_SEGMENTS.map((item) => ({
                  value: item.value,
                  label: item.label,
                  count: item.value === 'active' ? counts.active : item.value === 'failed' ? counts.failed : undefined,
                }))}
                value={segment}
                onValueChange={onSegmentChange}
              />
            </FilterField>
          </FilterBar>
        }
        footer={tasks.status === 'success' ? capNote(tasks.data, '条') : undefined}
      >
        <ResourceState
          resource={tasks}
          emptyIcon={RefreshCw}
          emptyTitle="暂无判题任务"
          emptyDescription={emptyText}
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(page) =>
            page.list.length === 0 ? (
              <p className="px-4 py-8 text-center text-sm text-ink-sub">{emptyText}</p>
            ) : (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(task) => task.task_id}
                elevated={false}
                onRowClick={onOpenTask}
                // <md 换行卡(§6.4.1 规则 3):来源一行、判题结论一行,状态在右。
                // JudgeTask 没有时间字段,没有结论时直接说还在队列里
                mobileCard={(task) => ({
                  title: judgeSourceLabel(task.source_ref),
                  meta: task.result
                    ? `${task.result.passed ? '通过' : '未通过'} · 得分 ${task.result.score}`
                    : '尚未产出结论',
                  badge: (
                    <StatusIndicator
                      tone={judgeTaskStatusTone(task.status)}
                      label={judgeTaskStatusLabel(task.status)}
                    />
                  ),
                  action: isJudgeTaskAbnormal(task.status) ? (
                    <IconButton
                      variant="ghost"
                      size="sm"
                      icon={RotateCcw}
                      aria-label={`重判 ${judgeSourceLabel(task.source_ref)}`}
                      disabled={rejudgingId === task.task_id}
                      onClick={() => void rejudge(task.task_id)}
                    />
                  ) : undefined,
                })}
              />
            )
          }
        </ResourceState>
      </DataPanel>
    </>
  )
}

interface ExperimentTabProps {
  segment: string
  onSegmentChange: (value: string) => void
  /** 各分段的全量计数;读取中为 undefined */
  counts: { running?: number; paused?: number; failed?: number }
  refreshVersion: number
  labels: InstanceLabels
}

/**
 * ExperimentTab 学生实验环境:状态分段 + 表格(实验、学生、状态、最近活跃、开始时间)。
 * 数据来自实例本身的运行状态(GET /experiment/instances),按最近活跃排序;整行进入该实验的报告与小组。
 */
function ExperimentTab({ segment, onSegmentChange, counts, refreshVersion, labels }: ExperimentTabProps) {
  const navigate = useNavigate()
  const instances = useAsyncResource(
    () => api.experiment.listInstances({ status: envStatusOf(segment), page: 1, size: MONITOR_SIZE }),
    [segment, refreshVersion],
    () => false,
  )

  const columns: TableColumn<ExperimentInstance>[] = [
    {
      key: 'experiment',
      header: '实验',
      primary: true,
      render: (instance) => {
        const name = labels.experimentName(instance.experiment_id)
        return name === undefined ? <Skeleton variant="line" width="half" /> : name
      },
    },
    {
      key: 'student',
      header: '学生',
      render: (instance) => {
        const student = labels.student(instance)
        return (
          <span className="flex items-center gap-2">
            {student ? <StudentName member={student} compact /> : <span className="text-ink-sub">—</span>}
            {instance.group_id ? <span className="text-xs text-ink-sub">小组</span> : null}
          </span>
        )
      },
    },
    {
      key: 'status',
      header: '状态',
      render: (instance) => (
        <StatusIndicator
          tone={experimentInstanceStatusTone(instance.status)}
          label={experimentInstanceStatusLabel(instance.status)}
          loading={instance.status === ExperimentInstanceStatus.CREATING}
        />
      ),
    },
    {
      key: 'last_active_at',
      header: '最近活跃',
      render: (instance) => (
        <span title={formatDateTime(instance.last_active_at)} className="whitespace-nowrap text-ink-sub">
          {formatRelativeOccurrence(instance.last_active_at)}
        </span>
      ),
    },
    {
      key: 'started_at',
      header: '开始时间',
      render: (instance) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(instance.started_at)}
        </span>
      ),
    },
  ]

  const emptyText =
    segment === 'running'
      ? '现在没有进行中的实验环境。学生开始实验后会出现在这里。'
      : segment === 'paused'
        ? '没有已暂停的实验环境。'
        : '没有准备失败的实验环境。'

  return (
    <DataPanel
      label="实验环境"
      filter={
        <FilterBar label="实验环境筛选">
          <FilterField label="状态" group>
            <SegmentedControl
              aria-label="按环境状态筛选"
              size="sm"
              options={ENV_SEGMENTS.map((item) => ({
                value: item.value,
                label: item.label,
                count: item.value === 'running' ? counts.running : item.value === 'paused' ? counts.paused : counts.failed,
              }))}
              value={segment}
              onValueChange={onSegmentChange}
            />
          </FilterField>
        </FilterBar>
      }
      footer={instances.status === 'success' ? capNote(instances.data, '个') : undefined}
    >
      <ResourceState
        resource={instances}
        emptyIcon={FlaskConical}
        emptyTitle="暂无实验环境"
        emptyDescription={emptyText}
        skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
      >
        {(page) =>
          page.list.length === 0 ? (
            <p className="px-4 py-8 text-center text-sm text-ink-sub">{emptyText}</p>
          ) : (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(instance) => instance.instance_id}
              elevated={false}
              getRowHref={(instance) => `/teacher/experiments/${instance.experiment_id}/reports`}
              onNavigate={(href) => navigate(href)}
              // <md 换行卡(§6.4.1 规则 3):实验一行、学生与最近活跃一行,状态在右
              mobileCard={(instance) => {
                const student = labels.student(instance)
                return {
                  title: labels.experimentName(instance.experiment_id) ?? <Skeleton variant="line" width="half" />,
                  meta: (
                    <span className="flex flex-wrap items-center gap-x-2">
                      {student ? <StudentName member={student} compact /> : null}
                      <span>最近活跃 {formatRelativeOccurrence(instance.last_active_at)}</span>
                    </span>
                  ),
                  badge: (
                    <StatusIndicator
                      tone={experimentInstanceStatusTone(instance.status)}
                      label={experimentInstanceStatusLabel(instance.status)}
                    />
                  ),
                }
              }}
            />
          )
        }
      </ResourceState>
    </DataPanel>
  )
}

interface BattleTabProps {
  contests: AsyncResourceState<Contest[]>
  /** 地址里选中的竞赛;空或已不在进行中时取第一场 */
  contestId: string
  onContestChange: (value: string) => void
  /** 对局状态分段(写进地址) */
  segment: string
  onSegmentChange: (value: string) => void
  refreshVersion: number
}

/** BattleTab 进行中对抗赛的对局;没有进行中的对抗赛时说明何时会有 */
function BattleTab({ contests, contestId, onContestChange, segment, onSegmentChange, refreshVersion }: BattleTabProps) {
  return (
    <ResourceState
      resource={contests}
      emptyIcon={Swords}
      emptyTitle="现在没有进行中的对抗赛"
      emptyDescription="对抗赛开赛后,对局的执行状态会显示在这里。"
      skeleton={<Skeleton variant="line" lines={4} />}
    >
      {(list) =>
        list.length === 0 ? (
          <div className="rounded-lg bg-surface shadow-xs">
            <Empty
              icon={Swords}
              title="现在没有进行中的对抗赛"
              description="对抗赛开赛后,对局的执行状态会显示在这里。解题赛没有对局。"
            />
          </div>
        ) : (
          <BattleMatches
            contests={list}
            contestId={contestId}
            segment={segment}
            onSegmentChange={onSegmentChange}
            onContestChange={onContestChange}
            refreshVersion={refreshVersion}
          />
        )
      }
    </ResourceState>
  )
}

/**
 * BattleMatches 一场对抗赛的最近对局:选竞赛 + 表格(开局时间、状态、结果、结束时间)。
 * 长时间未结束或失败的对局要去竞赛管理里检查参战作品,所以工具条上直接给入口。
 */
function BattleMatches({
  contests,
  contestId,
  onContestChange,
  segment,
  onSegmentChange,
  refreshVersion,
}: Omit<BattleTabProps, 'contests'> & { contests: Contest[] }) {
  const active = contests.find((contest) => contest.id === contestId) ?? contests[0]
  const battleSegment = BATTLE_SEGMENTS.find((item) => item.value === segment)
  const battleStatus = battleSegment && 'status' in battleSegment ? battleSegment.status : undefined
  const matches = useAsyncResource(
    () => api.contest.listBattleMatches(active.id, { status: battleStatus, page: 1, size: MONITOR_SIZE }),
    [active.id, battleStatus, refreshVersion],
    () => false,
  )
  // 分段上的数字取列表返回的 facets(套用所选竞赛,不套用状态);读取中或失败时不显示
  const battleFacets = matches.status === 'success' || matches.status === 'empty' ? matches.data?.facets : undefined
  const battleCounts = segmentCounts(battleFacets, 'status', BATTLE_SEGMENTS, (item) =>
    'status' in item ? item.status : undefined,
  )

  const columns: TableColumn<BattleMatch>[] = [
    {
      key: 'matched_at',
      header: '开局时间',
      render: (match) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink">
          {formatShortDateTime(match.matched_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '对局状态',
      render: (match) => (
        <StatusIndicator
          tone={battleMatchStatusTone(match.status)}
          label={battleMatchStatusLabel(match.status)}
          loading={match.status === BattleMatchStatus.RUNNING}
        />
      ),
    },
    {
      key: 'result',
      header: '结果',
      render: (match) =>
        match.result ? (
          <Badge tone="neutral">{battleResultLabel(match.result)}</Badge>
        ) : (
          <span className="text-ink-sub">未出结果</span>
        ),
    },
    {
      key: 'finished_at',
      header: '结束时间',
      render: (match) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {match.finished_at ? formatShortDateTime(match.finished_at) : '—'}
        </span>
      ),
    },
  ]

  return (
    <DataPanel
      label="竞赛对局"
      filter={
        <FilterBar label="竞赛对局筛选">
          <FilterField label="竞赛" htmlFor="monitor-contest">
            <Select
              id="monitor-contest"
              value={active.id}
              onValueChange={onContestChange}
              options={contests.map((contest) => ({
                value: contest.id,
                label: `${contest.name} · ${contestStatusLabel(contest.status)}`,
              }))}
              className="w-full sm:w-72"
            />
          </FilterField>
          <FilterField label="状态" group>
            <SegmentedControl
              aria-label="按对局状态筛选"
              size="sm"
              options={BATTLE_SEGMENTS.map((item) => ({
                value: item.value,
                label: item.label,
                count: battleCounts[item.value],
              }))}
              value={segment}
              onValueChange={onSegmentChange}
            />
          </FilterField>
          <Link to={`/teacher/contests/${active.id}`} className={buttonVariants({ variant: 'ghost', size: 'sm' })}>
            管理这场竞赛
          </Link>
        </FilterBar>
      }
      footer={matches.status === 'success' ? capNote(matches.data, '场') : undefined}
    >
      <ResourceState
        resource={matches}
        emptyIcon={Swords}
        emptyTitle="还没有对局"
        emptyDescription="学生提交参战作品后,系统会自动安排对局。"
        skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
      >
        {(page) =>
          page.list.length === 0 ? (
            <p className="px-4 py-8 text-center text-sm text-ink-sub">
              {battleSegment && 'status' in battleSegment
                ? `这场竞赛目前没有${battleSegment.label}的对局。`
                : '这场竞赛还没有对局。学生提交参战作品后,系统会自动安排。'}
            </p>
          ) : (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(match) => match.id}
              elevated={false}
              // <md 换行卡(§6.4.1 规则 3):对局只回内部编号,用开局时刻标识这一局(与桌面列同一口径)
              mobileCard={(match) => ({
                title: `开局 ${formatShortDateTime(match.matched_at)}`,
                meta: `${match.result ? battleResultLabel(match.result) : '未出结果'}${match.finished_at ? ` · 结束 ${formatShortDateTime(match.finished_at)}` : ' · 尚未结束'}`,
                badge: (
                  <StatusIndicator
                    tone={battleMatchStatusTone(match.status)}
                    label={battleMatchStatusLabel(match.status)}
                    loading={match.status === BattleMatchStatus.RUNNING}
                  />
                ),
              })}
            />
          )
        }
      </ResourceState>
    </DataPanel>
  )
}

interface JudgeTaskDetailModalProps {
  task: JudgeTask
  onClose: () => void
}

/**
 * JudgeTaskDetailModal 展示一个判题任务的逐项判定。
 * 列表只放首条说明(空间有限),完整的逐项结果在这里 —— 教师排查「为什么没通过」
 * 需要看到每一条,而不是一句结论。
 *
 * 打开时重读一次:列表是快照,而判题是异步的,点开的这一刻结果可能已经出来了。
 * 判定详情里不显示测试用例内容或答案(后端 detail 已按 privacy 规则过滤),
 * 只呈现用例名、期望说明与提示。
 */
function JudgeTaskDetailModal({ task, onClose }: JudgeTaskDetailModalProps) {
  const detail = useAsyncResource(() => api.judge.getTask(task.task_id), [task.task_id], () => false)

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>判题详情</ModalTitle>
          <ModalDescription>逐项判定结果。判题是异步的,打开时会重新读一次最新状态。</ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <ResourceState
            resource={detail}
            emptyIcon={RefreshCw}
            emptyTitle="读不到这条任务"
            emptyDescription="任务可能已被清理。回列表刷新看看。"
            skeleton={<Skeleton variant="line" lines={5} />}
          >
            {(current) => <JudgeTaskDetailBody task={current} />}
          </ResourceState>
        </ModalBody>
        <ModalFooter>
          <Button variant="ghost" leftIcon={RefreshCw} onClick={detail.reload}>
            重新读取
          </Button>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/**
 * JudgeTaskDetailBody 渲染任务概要与逐项判定。
 */
function JudgeTaskDetailBody({ task }: { task: JudgeTask }) {
  const details = task.result?.details ?? []

  return (
    <div className="flex flex-col gap-4">
      <DescriptionList
        dense
        columns={2}
        items={[
          { term: '判题状态', description: judgeTaskStatusLabel(task.status) },
          { term: '业务归属', description: judgeSourceLabel(task.source_ref) },
          ...(task.result
            ? [
                {
                  term: '得分',
                  description: `${task.result.score} / ${task.result.max_score}`,
                  mono: true,
                },
                { term: '判定结论', description: task.result.passed ? '通过' : '未通过' },
                ...(task.result.is_rejudge ? [{ term: '本次来源', description: '重判' }] : []),
              ]
            : []),
        ]}
      />

      {details.length === 0 ? (
        <Callout tone="info">
          {task.result ? '这次判题没有逐项结果,判定结论见上方。' : '判题还没有结论。稍后重新读取即可。'}
        </Callout>
      ) : (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-ink-sub">逐项判定({details.length} 项)</span>
          <ul className="flex flex-col gap-2">
            {details.map((item, index) => (
              <li
                key={`${item.case ?? item.target ?? item.source ?? 'item'}-${index}`}
                className="flex flex-col gap-1 rounded-md bg-surface-hover p-3"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="min-w-0 truncate text-sm text-ink">
                    {item.case ?? item.target ?? item.source ?? `第 ${index + 1} 项`}
                  </span>
                  <Badge tone={item.passed ? 'success' : 'danger'}>{item.passed ? '通过' : '未通过'}</Badge>
                </div>
                {item.expected_label ? <p className="text-xs text-ink-sub">期望:{item.expected_label}</p> : null}
                {item.actual ? <p className="font-mono text-xs text-ink-sub">实际:{item.actual}</p> : null}
                {item.hint ? <p className="text-xs text-ink-sub">提示:{item.hint}</p> : null}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

/**
 * judgeSourceLabel 把 source_ref 的来源段翻成业务归属。
 * source_ref 是 <来源>:<年份>:<资源类型>:<id>,全串是内部标识,不直出给用户。
 */
function judgeSourceLabel(sourceRef: string): string {
  const [scope, , resourceType] = sourceRef.split(':')
  const scopeName =
    scope === 'teaching'
      ? '课程作业'
      : scope === 'experiment'
        ? '实验检查点'
        : scope === 'contest'
          ? '竞赛提交'
          : '平台任务'
  const resourceName =
    resourceType === 'submission-item'
      ? '作业题目'
      : resourceType === 'checkpoint'
        ? '检查点'
        : resourceType === 'submission'
          ? '提交'
          : resourceType === 'match'
            ? '对局'
            : ''
  return resourceName ? `${scopeName} · ${resourceName}` : scopeName
}
