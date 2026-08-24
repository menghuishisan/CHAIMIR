// 实时监控页(教师侧栏,/teacher/monitoring)。
//
// 把教师需要盯的三件事放在一屏:判题任务是否堵住、实验环境是否有异常、
// 竞赛对局是否卡住。数据来自 M3 判题任务、M7 实例统计与 M8 对局记录 ——
// 全部是真实状态,不做装饰性雷达或假飞线(对齐清单 §5「可视化」)。
//
// 刷新是显式动作:高频路径不自动轮询(规范 §4.1 高频路径不动),
// 由教师按需刷新或在处理完一项后自动刷新。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Activity, FlaskConical, RefreshCw, RotateCcw, Swords } from 'lucide-react'
import {
  BattleMatchStatus,
  ContestStatus,
  ExperimentInstanceStatus,
  JUDGE_TASK_STATE,
  type BattleMatch,
  type Contest,
  type ExperimentInstance,
  type JudgeTask,
  type JudgeTaskState,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  DescriptionList,
  FilterBar,
  FilterField,
  IconButton,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  MetricStrip,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { isJudgeTaskAbnormal, isJudgeTaskActive } from '../../../judge/rules'
import { judgeTaskStatusTone } from '../../../judge/statusPresentation'
import { useRejudgeTask } from '../../../judge/useRejudgeTask'
import { useAsyncResource, useResourceTotal } from '../../../../hooks'
import { formatDateTime, formatShortDateTime } from '../../../../utils/formatters'
import {
  battleMatchStatusLabel,
  battleResultLabel,
  contestStatusLabel,
} from '../../../../utils/labels/contest'
import { experimentInstanceStatusLabel } from '../../../../utils/labels/experiment'
import { judgeTaskStatusLabel } from '../../../../utils/labels/judge'
import { battleMatchStatusTone } from '../../../contest/statusPresentation'
import { experimentInstanceStatusTone } from '../../../experiment/statusPresentation'

/** 监控面板一次取回的条数:监控看的是"当下有没有问题",不是全量历史。 */
const MONITOR_SIZE = 30

/** 判题状态筛选:异常态是教师真正要处理的。值直接作为服务端 state 参数,空串表示不筛。 */
const JUDGE_FILTERS = [
  { value: JUDGE_TASK_STATE.ABNORMAL, label: '需要处理' },
  { value: JUDGE_TASK_STATE.ACTIVE, label: '进行中' },
  { value: '', label: '全部' },
] as const

/**
 * TeacherMonitoringPage 汇总判题、实验与对局的实时状态。
 */
export default function TeacherMonitoringPage() {
  const [judgeFilter, setJudgeFilter] = useState<string>(JUDGE_TASK_STATE.ABNORMAL)
  const [detailTask, setDetailTask] = useState<JudgeTask>()

  // 筛选交给服务端:在本机对最近 30 条再筛一次,会在这 30 条里恰好没有异常任务时
  // 显示「没有需要处理的判题任务」,而队列里可能积压着更早的失败任务。
  const judgeTasks = useAsyncResource(
    () =>
      api.judge.getTasks({
        state: judgeFilter ? (judgeFilter as JudgeTaskState) : undefined,
        page: 1,
        size: MONITOR_SIZE,
      }),
    [judgeFilter],
    () => false
  )
  // 对局监控要的是「正在打的赛事」,按状态向服务端取,而不是在最近 30 场里挑
  const runningContestsResource = useAsyncResource(
    () => api.contest.getContests({ status: ContestStatus.RUNNING, page: 1, size: MONITOR_SIZE }),
    [],
    () => false
  )
  const frozenContestsResource = useAsyncResource(
    () => api.contest.getContests({ status: ContestStatus.FROZEN, page: 1, size: MONITOR_SIZE }),
    [],
    () => false
  )

  // 指标带取服务端全量口径:队列积压多少条不能由最近 30 条推断
  const activeTaskCount = useResourceTotal(
    (params) => api.judge.getTasks({ state: JUDGE_TASK_STATE.ACTIVE, ...params }),
    []
  )
  const abnormalTaskCount = useResourceTotal(
    (params) => api.judge.getTasks({ state: JUDGE_TASK_STATE.ABNORMAL, ...params }),
    []
  )
  const runningContestCount = useResourceTotal(
    (params) => api.contest.getContests({ status: ContestStatus.RUNNING, ...params }),
    []
  )
  const frozenContestCount = useResourceTotal(
    (params) => api.contest.getContests({ status: ContestStatus.FROZEN, ...params }),
    []
  )
  const liveContestCount =
    runningContestCount === undefined || frozenContestCount === undefined
      ? undefined
      : runningContestCount + frozenContestCount

  const runningContests = useMemo(
    () => [
      ...(runningContestsResource.data?.list ?? []),
      ...(frozenContestsResource.data?.list ?? []),
    ],
    [frozenContestsResource.data, runningContestsResource.data]
  )

  const refreshAll = useCallback(() => {
    judgeTasks.reload()
    runningContestsResource.reload()
    frozenContestsResource.reload()
  }, [frozenContestsResource, judgeTasks, runningContestsResource])

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '实践' }]} />}
        title="实时监控"
        description="判题是否堵住、实验环境是否异常、对局是否卡住 —— 需要处理的都会列在这里。"
        icon={Activity}
        actions={
          <IconButton
            variant="outline"
            icon={RefreshCw}
            aria-label="刷新监控数据"
            onClick={refreshAll}
          />
        }
      />

      {/*
        归族:资源列表族(§6.5.3 第 ① 族)—— 本页主体是三块要盯的清单,不是三个数字。
        指标降为内联摘要:三项都取服务端全量口径,「监控范围 最近 30 条」是范围常量而非
        可度量数字,由判题任务分组说明承载(§6.5.4)。
      */}
      <MetricStrip
        label="待处理摘要"
        className="mb-5"
        items={[
          { label: '判题进行中', value: activeTaskCount ?? '—', hint: '排队与判题中的任务' },
          {
            label: '判题需要处理',
            value: abnormalTaskCount ?? '—',
            hint: abnormalTaskCount === 0 ? '暂无异常' : '判题失败',
          },
          { label: '进行中竞赛', value: liveContestCount ?? '—', hint: '含封榜中' },
        ]}
      />

      <JudgeMonitorSection
        tasks={judgeTasks}
        filter={judgeFilter}
        onFilterChange={setJudgeFilter}
        onOpenDetail={setDetailTask}
      />

      <ExperimentMonitorSection />

      <BattleMonitorSection contests={runningContests} />

      {detailTask ? (
        <JudgeTaskDetailModal task={detailTask} onClose={() => setDetailTask(undefined)} />
      ) : null}
    </PageScaffold>
  )
}

interface JudgeMonitorSectionProps {
  tasks: ReturnType<
    typeof useAsyncResource<{ list: JudgeTask[]; total: number; page: number; size: number }>
  >
  filter: string
  onFilterChange: (filter: string) => void
  onOpenDetail: (task: JudgeTask) => void
}

/**
 * JudgeMonitorSection 列出判题任务并支持重判。
 * 默认只看需要处理的:全量任务列表对教师没有行动价值。
 */
function JudgeMonitorSection({
  tasks,
  filter,
  onFilterChange,
  onOpenDetail,
}: JudgeMonitorSectionProps) {
  const { rejudge, rejudgingId, actionError } = useRejudgeTask(tasks.reload)

  const columns: TableColumn<JudgeTask>[] = [
    {
      key: 'status',
      header: '判题状态',
      render: (task) => (
        <StatusIndicator
          tone={judgeTaskStatusTone(task.status)}
          label={judgeTaskStatusLabel(task.status)}
          loading={isJudgeTaskActive(task.status)}
        />
      ),
    },
    {
      key: 'source_ref',
      header: '来源',
      // source_ref 格式是 <来源>:<年份>:<资源类型>:<id>,取来源段显示业务归属,不直出全串
      render: (task) => <span className="text-ink-sub">{judgeSourceLabel(task.source_ref)}</span>,
    },
    {
      key: 'result',
      header: '判题结果',
      render: (task) =>
        task.result ? (
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge tone={task.result.passed ? 'success' : 'danger'}>
              {task.result.passed ? '通过' : '未通过'}
            </Badge>
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
      header: '首条详情',
      render: (task) =>
        task.result && task.result.details.length > 0 ? (
          <span className="line-clamp-1 text-xs text-ink-sub">
            {task.result.details[0].hint ??
              task.result.details[0].expected_label ??
              task.result.details[0].case ??
              '判题详情见结果'}
          </span>
        ) : (
          <span className="text-ink-sub">—</span>
        ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (task) => (
        <div className="flex items-center justify-end gap-1">
          <Button variant="ghost" size="sm" onClick={() => onOpenDetail(task)}>
            看详情
          </Button>
          {isJudgeTaskAbnormal(task.status) ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={RotateCcw}
              loading={rejudgingId === task.task_id}
              onClick={() => void rejudge(task.task_id)}
            >
              重判
            </Button>
          ) : null}
        </div>
      ),
    },
  ]

  return (
    <PageSection
      title="判题任务"
      description={`按提交时间从新到旧,最多列出最近 ${MONITOR_SIZE} 条。判题失败的任务可以重判,重判会用原始提交重新执行。`}
    >
      {/* 动作失败就近内联(§6.7 C) */}
      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {/* 筛选井与数据表同处一块抬起片(§6.5.2);本区最多 30 条不分页,故无页脚 */}
      <DataPanel
        label="判题任务"
        filter={
          <FilterBar label="判题任务筛选">
            <FilterField label="判题状态" group>
              <SegmentedControl
                aria-label="按判题状态筛选"
                size="sm"
                options={JUDGE_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={filter}
                onValueChange={onFilterChange}
              />
            </FilterField>
          </FilterBar>
        }
      >
        <ResourceState
          resource={tasks}
          emptyIcon={RefreshCw}
          emptyTitle="暂无判题任务"
          emptyDescription="学生提交编程题或竞赛答题后会产生判题任务。"
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(page) =>
            page.list.length === 0 ? (
              // 筛选无结果不是空态:数据源有内容,只是当前条件筛不出来,故用行内说明而不是空态块
              <p className="px-4 py-8 text-center text-sm text-ink-sub">
                {filter === JUDGE_TASK_STATE.ABNORMAL
                  ? '没有需要处理的判题任务。'
                  : '这个筛选下没有任务。'}
              </p>
            ) : (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(task) => task.task_id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):题目一行、提交人与时间一行,状态在右
                mobileCard={(task) => ({
                  title: judgeSourceLabel(task.source_ref),
                  // JudgeTask 没有时间字段(见 api-client types/judge.ts),故次要行给判题结论;
                  // 没有结论时说明还在队列里,直接说出来而不是留空
                  meta: task.result
                    ? `${task.result.passed ? '通过' : '未通过'} · 得分 ${task.result.score}`
                    : '尚未产出结论',
                  badge: (
                    <StatusIndicator
                      tone={judgeTaskStatusTone(task.status)}
                      label={judgeTaskStatusLabel(task.status)}
                    />
                  ),
                })}
              />
            )
          }
        </ResourceState>
      </DataPanel>
    </PageSection>
  )
}

interface JudgeTaskDetailModalProps {
  task: JudgeTask
  onClose: () => void
}

/**
 * JudgeTaskDetailModal 展示一个判题任务的逐项判定。
 * 列表只放首条详情(空间有限),完整的逐项结果在这里 —— 教师排查「为什么没通过」
 * 需要看到每一条,而不是一句结论。
 *
 * 打开时重读一次:列表是快照,而判题是异步的,点开的这一刻结果可能已经出来了。
 * 判定详情里不显示测试用例内容或答案(后端 detail 已按 privacy 规则过滤),
 * 只呈现用例名、期望说明与提示。
 */
function JudgeTaskDetailModal({ task, onClose }: JudgeTaskDetailModalProps) {
  const detail = useAsyncResource(
    () => api.judge.getTask(task.task_id),
    [task.task_id],
    () => false
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>判题详情</ModalTitle>
          <ModalDescription>
            逐项判定结果。判题是异步的,打开时会重新读一次最新状态。
          </ModalDescription>
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
          {task.result
            ? '这次判题没有逐项结果,判定结论见上方。'
            : '判题还没有结论。稍后重新读取即可。'}
        </Callout>
      ) : (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-ink-sub">逐项判定({details.length} 项)</span>
          <ul className="flex flex-col gap-2">
            {details.map((item, index) => (
              <li
                key={`${item.case ?? item.target ?? item.source ?? 'item'}-${index}`}
                className="flex flex-col gap-1 well p-3"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="min-w-0 truncate text-sm text-ink">
                    {item.case ?? item.target ?? item.source ?? `第 ${index + 1} 项`}
                  </span>
                  <Badge tone={item.passed ? 'success' : 'danger'}>
                    {item.passed ? '通过' : '未通过'}
                  </Badge>
                </div>
                {item.expected_label ? (
                  <p className="text-xs text-ink-sub">期望:{item.expected_label}</p>
                ) : null}
                {item.actual ? (
                  <p className="font-mono text-xs text-ink-sub">实际:{item.actual}</p>
                ) : null}
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

/**
 * ExperimentMonitorSection 汇总学生实验环境的真实运行态。
 * 数据走教师侧实例列表接口(GET /experiment/instances):按实例状态呈现「几个在跑、几个出错」,
 * 不再从实验清单推断「可能占用环境」—— 那是猜测,不是监控。
 * 出错的实例单列出来:教师要的是「哪几个学生卡住了」,不是总数。
 */
function ExperimentMonitorSection() {
  const navigate = useNavigate()

  const running = useAsyncResource(
    () =>
      api.experiment.listInstances({
        status: ExperimentInstanceStatus.RUNNING,
        page: 1,
        size: MONITOR_SIZE,
      }),
    [],
    () => false
  )
  const errored = useAsyncResource(
    () =>
      api.experiment.listInstances({
        status: ExperimentInstanceStatus.ERROR,
        page: 1,
        size: MONITOR_SIZE,
      }),
    [],
    () => false
  )
  const paused = useAsyncResource(
    () =>
      api.experiment.listInstances({
        status: ExperimentInstanceStatus.PAUSED,
        page: 1,
        size: MONITOR_SIZE,
      }),
    [],
    () => false
  )

  const columns: TableColumn<ExperimentInstance>[] = [
    {
      key: 'status',
      header: '环境状态',
      render: (instance) => (
        <StatusIndicator
          tone={experimentInstanceStatusTone(instance.status)}
          label={experimentInstanceStatusLabel(instance.status)}
          loading={instance.status === ExperimentInstanceStatus.CREATING}
        />
      ),
    },
    {
      key: 'sandboxes',
      header: '环境数',
      align: 'right',
      mono: true,
      render: (instance) => instance.sandboxes.length,
    },
    {
      key: 'started_at',
      header: '开始时间',
      render: (instance) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(instance.started_at)}
        </span>
      ),
    },
    {
      key: 'last_active_at',
      header: '最近活跃',
      render: (instance) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(instance.last_active_at)}
        </span>
      ),
    },
  ]

  return (
    <PageSection
      title="实验环境"
      description="学生进入实验会占用沙箱资源。下面的数字与列表都来自实例本身的运行状态。"
      actions={
        <Button variant="ghost" size="sm" onClick={() => navigate('/teacher/experiments')}>
          去实验编排
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {/* 三个数字各来自一次带状态参数的服务端查询,故是全量口径而非当前页切片(§6.5.4) */}
        <MetricStrip
          label="实验环境摘要"
          items={[
            { label: '正在进行', value: running.data?.total ?? '—', hint: '学生正在做' },
            { label: '已暂停', value: paused.data?.total ?? '—', hint: '环境已挂起' },
            { label: '出错', value: errored.data?.total ?? '—', hint: '需要提醒学生重进' },
          ]}
        />

        {/* 列表型页内子视图走 DataPanel 片段(§6.5.5 B) */}
        <DataPanel label="出错的实验环境">
          <ResourceState
            resource={errored}
            emptyIcon={FlaskConical}
            emptyTitle="没有出错的实验环境"
            emptyDescription="学生的实验环境都正常。出错的环境会出现在这里。"
            skeleton={
              <Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />
            }
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.instance_id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):状态一行、时间一行
                mobileCard={(item) => ({
                  title: experimentInstanceStatusLabel(item.status),
                  meta: `开始 ${formatDateTime(item.started_at)} · 环境 ${item.sandboxes.length} 个`,
                })}
              />
            )}
          </ResourceState>
        </DataPanel>

        <Callout tone="info" title="关于实验环境状态">
          单个学生的环境细节在实验工作台内可见。如果学生反馈环境准备失败,
          让他退出后重新进入即可重新准备。
        </Callout>
      </div>
    </PageSection>
  )
}

/**
 * BattleMonitorSection 汇总进行中竞赛的对局状态。
 * 只看进行中的赛事:已结束赛事的对局记录在竞赛详情里回顾。
 */
function BattleMonitorSection({ contests }: { contests: Contest[] }) {
  const navigate = useNavigate()
  const [selectedId, setSelectedId] = useState<string>('')
  const activeContestId = selectedId || contests[0]?.id || ''

  const matches = useAsyncResource(
    () =>
      activeContestId
        ? api.contest.listBattleMatches(activeContestId, { page: 1, size: MONITOR_SIZE })
        : Promise.resolve({ list: [], total: 0, page: 1, size: MONITOR_SIZE }),
    [activeContestId],
    (value) => value.list.length === 0
  )

  const columns: TableColumn<BattleMatch>[] = [
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
      key: 'matched_at',
      header: '开局时间',
      render: (match) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(match.matched_at)}
        </span>
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

  if (contests.length === 0) {
    return (
      <PageSection title="竞赛对局">
        <Callout tone="info">当前没有进行中的竞赛。开赛后对局状态会显示在这里。</Callout>
      </PageSection>
    )
  }

  return (
    <PageSection
      title="竞赛对局"
      description="对抗赛的对局执行状态。长时间未结束的对局需要检查参战物是否有问题。"
      actions={
        activeContestId ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigate(`/teacher/contests/${activeContestId}`)}
          >
            管理赛事
          </Button>
        ) : undefined
      }
    >
      {/* 筛选井与数据表同处一块抬起片(§6.5.2):井不摆在光面上。
          不接分页:这一块只看最近 MONITOR_SIZE 条,它是提示区不是全量列表 */}
      <DataPanel
        label="竞赛对局"
        filter={
          <FilterBar label="竞赛对局筛选">
            <FilterField label="监控的竞赛" group>
              <SegmentedControl
                aria-label="选择要监控的竞赛"
                size="sm"
                options={contests.slice(0, 3).map((contest) => ({
                  value: contest.id,
                  label: `${contest.name} · ${contestStatusLabel(contest.status)}`,
                }))}
                value={activeContestId}
                onValueChange={setSelectedId}
              />
            </FilterField>
          </FilterBar>
        }
      >
        <ResourceState
          resource={matches}
          emptyIcon={Swords}
          emptyTitle="还没有对局"
          emptyDescription="学生提交参战物后系统会安排对局。解题赛没有对局记录。"
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(page) => (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(match) => match.id}
              elevated={false}
              // <md 换行卡(§6.4.1 规则 3):对局时间一行、结果与时长一行,对局状态在右。
              // 对局只回内部编号,不把它当标题显示 —— 用开局时刻标识这一局(与桌面列同一口径)
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
          )}
        </ResourceState>
      </DataPanel>
    </PageSection>
  )
}
