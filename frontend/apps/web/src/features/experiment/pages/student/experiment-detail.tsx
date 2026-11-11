// 实验详情页(深页,/student/experiments/:experimentId)。设计 §7.2:
// 页头为实验名 + 方式(个人 / 小组)+ 一行属性(所属课程 · 是否交报告 · 检查点总分),右侧主按钮「开始实验」;
// 小组实验在页头下方放小组卡。主体单栏顺序阅读:实验说明 → 阶段(编号列表 + 链式进度)→
// 检查点(名称、分值、判分方式、公开题面)→ 实验环境(代码环境、仿真场景)。
// 小组实验还未分组时按钮不可用,旁边写「等待老师分组后才能开始」。
//
// 这也是实验工作台退出后的回落页(immersiveRoutes 的 exitPath),故必须支持深链与刷新:
// 数据取自学生专用单读 GET /experiment/student/experiments/{id},与列表同一投影。
// 阶段与检查点只呈现学生可见部分;判题器编号、题目引用、环境初始化脚本已由后端投影剔除,
// 前端不做二次过滤(铁律 1)。阶段的实时状态属于实验环境(工作台内),这里只讲解锁规则。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Boxes, FlaskConical, Play, Radar, Users } from 'lucide-react'
import {
  ExperimentCollabMode,
  JUDGE_SANDBOX_MODE,
  PAGINATION_MAX_SIZE,
  type StudentCheckpointConfig,
  type StudentExperiment,
  type StudentStageConfig,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  ChainProgress,
  Icon,
  PageHeader,
  PageScaffold,
  Skeleton,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePageLocation, locationLabel } from '../../../../hooks'
import { experimentCollabModeLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ExperimentGroupCard } from './experiment-group'

/**
 * StudentExperimentDetailPage 读取单个已发布实验的学生视图。
 */
export default function StudentExperimentDetailPage() {
  const { experimentId = '' } = useParams<{ experimentId: string }>()

  const experiment = useAsyncResource(
    () => api.experiment.getPublishedExperiment(experimentId),
    [experimentId],
    () => false,
  )

  usePageLocation([{ label: locationLabel(experiment, (data) => data.name, '实验详情') }])

  return (
    <PageScaffold>
      <ResourceState
        resource={experiment}
        emptyIcon={FlaskConical}
        emptyTitle="实验暂不可用"
        emptyDescription="这个实验可能已被老师下架,请回到实验实训查看其他实验。"
        skeleton={
          <div aria-busy="true" className="flex max-w-3xl flex-col gap-4">
            <Skeleton variant="title" width="half" />
            <Skeleton variant="line" width="third" />
            <Skeleton variant="block" />
            <Skeleton variant="block" />
          </div>
        }
      >
        {(data) => <ExperimentDetailContent experiment={data} />}
      </ResourceState>
    </PageScaffold>
  )
}

/**
 * ExperimentDetailContent 按阅读顺序渲染实验说明、阶段、检查点与实验环境,并提供进入入口。
 */
function ExperimentDetailContent({ experiment }: { experiment: StudentExperiment }) {
  const navigate = useNavigate()
  const [entering, setEntering] = useState(false)
  const [actionError, setActionError] = useState<string>()

  // 所属课程名:用学生自己的课程列表解析(一学期几门课,一页取完);读不到时不显示这一项
  const courses = useAsyncResource(
    () => api.teaching.getCourses({ role: 'student', size: PAGINATION_MAX_SIZE }),
    [],
    () => false,
  )
  const courseName = useMemo(
    () => courses.data?.list.find((course) => course.id === experiment.course_id)?.name,
    [courses.data, experiment.course_id],
  )

  const isGroup = experiment.collab_mode === ExperimentCollabMode.GROUP
  const needsGroup = isGroup && !experiment.my_group_id
  const { checkpoints, stages, envs, sims } = experiment.components
  const checkpointTotal = checkpoints.reduce((sum, checkpoint) => sum + checkpoint.score, 0)

  /** enterExperiment 创建或恢复实例后进入工作台(接口对活跃实例幂等)。 */
  const enterExperiment = useCallback(async () => {
    setEntering(true)
    setActionError(undefined)
    try {
      const instance = await api.experiment.createInstance(experiment.id, {
        group_id: experiment.my_group_id,
      })
      navigate(`/student/experiments/${experiment.id}/workspace?instance=${instance.instance_id}`)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '实验环境没能准备好,请稍后重试。'))
    } finally {
      setEntering(false)
    }
  }, [experiment.id, experiment.my_group_id, navigate])

  return (
    <div className="max-w-3xl">
      <PageHeader
        title={experiment.name}
        status={
          <Badge tone={isGroup ? 'info' : 'neutral'}>
            {isGroup ? <Icon icon={Users} size="xs" /> : null}
            {isGroup
              ? `小组 ${experiment.group_config.size} 人`
              : experimentCollabModeLabel(experiment.collab_mode)}
          </Badge>
        }
        meta={[
          { value: courseName },
          { value: experiment.require_report ? '需交实验报告' : '不用交实验报告' },
          { label: '检查点总分', value: checkpoints.length > 0 ? checkpointTotal : undefined },
        ]}
        actions={
          <div className="flex flex-col items-stretch gap-1.5 sm:items-end">
            <Button
              variant="primary"
              leftIcon={Play}
              loading={entering}
              disabled={needsGroup}
              onClick={() => void enterExperiment()}
            >
              开始实验
            </Button>
            {needsGroup ? (
              <span className="text-sm text-warning">等待老师分组后才能开始</span>
            ) : null}
          </div>
        }
      />

      {actionError ? (
        <Callout tone="danger" className="mb-5">
          {actionError}
        </Callout>
      ) : null}

      <div className="flex flex-col gap-8">
        {isGroup && experiment.my_group_id ? <ExperimentGroupCard groupId={experiment.my_group_id} /> : null}

        <DetailSection id="experiment-intro" title="实验说明">
          {experiment.description ? (
            <div className="flex flex-col gap-3 text-base leading-relaxed text-ink">
              {experiment.description
                .split('\n')
                .map((paragraph, index) => (paragraph.trim() === '' ? null : <p key={index}>{paragraph}</p>))}
            </div>
          ) : (
            <p className="text-sm text-ink-sub">老师没有写实验说明,进入实验后按各阶段目标完成即可。</p>
          )}
          {experiment.require_report ? (
            <p className="mt-3 text-sm text-ink-sub">完成实验后还要提交实验报告,报告由老师批改,得分与检查点分数一起计入实验成绩。</p>
          ) : null}
        </DetailSection>

        <DetailSection id="experiment-stages" title="实验阶段" count={stages.length}>
          {stages.length === 0 ? (
            <p className="text-sm text-ink-sub">这个实验不分阶段,进入后在同一个环境里完成全部检查点。</p>
          ) : (
            <>
              <ChainProgress total={stages.length} done={0} label="实验阶段" className="mb-4" />
              <ol className="flex flex-col gap-3">
                {[...stages]
                  .sort((a, b) => a.stage - b.stage)
                  .map((stage) => (
                    <StageItem key={stage.stage} stage={stage} />
                  ))}
              </ol>
            </>
          )}
        </DetailSection>

        <DetailSection id="experiment-checkpoints" title="检查点" count={checkpoints.length}>
          {checkpoints.length === 0 ? (
            <p className="text-sm text-ink-sub">这个实验没有检查点,完成后按老师批改的实验报告计分。</p>
          ) : (
            <ol className="flex flex-col gap-3">
              {checkpoints.map((checkpoint, index) => (
                <CheckpointItem key={checkpoint.id} checkpoint={checkpoint} index={index} />
              ))}
            </ol>
          )}
        </DetailSection>

        <DetailSection id="experiment-environment" title="实验环境">
          {envs.length === 0 && sims.length === 0 ? (
            <p className="text-sm text-ink-sub">这个实验不需要代码环境或仿真场景。</p>
          ) : (
            <ul className="flex flex-col gap-3">
              {envs.map((env, index) => (
                <li key={env.id} className="flex items-start gap-3 rounded-lg bg-surface p-4 shadow-xs">
                  <Icon icon={Boxes} size="md" className="mt-0.5 shrink-0 text-primary" />
                  <span className="min-w-0">
                    <span className="block text-base font-medium text-ink">
                      代码环境{envs.length > 1 ? ` ${index + 1}` : ''}
                    </span>
                    <span className="block text-sm text-ink-sub">
                      {[
                        env.runtimes.length > 0 ? `${env.runtimes.length} 条链` : undefined,
                        env.tools.length > 0 ? `${env.tools.length} 个工具` : undefined,
                        env.infra.length > 0 ? `${env.infra.length} 个配套服务` : undefined,
                      ]
                        .filter(Boolean)
                        .join(' · ') || '进入后自动准备'}
                    </span>
                  </span>
                </li>
              ))}
              {sims.map((sim) => (
                <li key={sim.id} className="flex items-start gap-3 rounded-lg bg-surface p-4 shadow-xs">
                  <Icon icon={Radar} size="md" className="mt-0.5 shrink-0 text-primary" />
                  <span className="min-w-0">
                    <span className="block text-base font-medium text-ink">仿真场景</span>
                    <span className="block truncate font-mono text-sm text-ink-sub">
                      {sim.package_code} · 版本 {sim.version}
                    </span>
                  </span>
                </li>
              ))}
            </ul>
          )}
          <p className="mt-3 text-sm text-ink-sub">
            进入实验会为{isGroup ? '小组' : '你'}准备独立的实验环境;中途退出,再进入会接着上次继续。
          </p>
        </DetailSection>
      </div>
    </div>
  )
}

/** DetailSection 顺序阅读里的一节:标题(可带计数)+ 内容,不画容器 */
function DetailSection({
  id,
  title,
  count,
  children,
}: {
  id: string
  title: string
  count?: number
  children: React.ReactNode
}) {
  return (
    <section aria-labelledby={id}>
      <h2 id={id} className="mb-3 flex items-baseline gap-2 text-lg font-semibold text-ink">
        {title}
        {count !== undefined && count > 0 ? (
          <span className="text-sm font-normal tabular-nums text-ink-sub">{count}</span>
        ) : null}
      </h2>
      {children}
    </section>
  )
}

/** unlockText 阶段的进入条件,用学生能懂的话说 */
function unlockText(stage: StudentStageConfig): string {
  const condition = stage.unlock_condition
  if (!condition) return '直接进入'
  if (condition.type === 'checkpoint') {
    return condition.min_score ? `上一个检查点达到 ${condition.min_score} 分后开放` : '上一个检查点通过后开放'
  }
  return '由老师开放'
}

/** StageItem 阶段列表里的一项:编号、名称、说明、包含内容与进入条件 */
function StageItem({ stage }: { stage: StudentStageConfig }) {
  const envCount = stage.components.envs?.length ?? 0
  const simCount = stage.components.sims?.length ?? 0
  const contents = [
    envCount > 0 ? `代码环境 ${envCount} 个` : undefined,
    simCount > 0 ? `仿真场景 ${simCount} 个` : undefined,
  ].filter(Boolean)
  return (
    <li className="flex gap-4 rounded-lg bg-surface p-4 shadow-xs">
      <span className="grid size-8 shrink-0 place-items-center rounded-full bg-primary-soft text-sm font-semibold tabular-nums text-primary">
        {stage.stage}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-base font-medium text-ink">{stage.title}</span>
        {stage.description ? (
          <span className="mt-1 block whitespace-pre-wrap text-sm leading-relaxed text-ink-sub">{stage.description}</span>
        ) : null}
        <span className="mt-1.5 block text-sm text-ink-sub">
          {[...contents, unlockText(stage)].join(' · ')}
        </span>
      </span>
    </li>
  )
}

/** 学生可见的公开题面字段与对应标题;同义字段只显示第一个有值的 */
const FACE_FIELDS = [
  ['scenario', '场景'],
  ['statement', '题目说明'],
  ['objective', '目标'],
  ['input', '输入'],
  ['input_contract', '输入要求'],
  ['output', '输出'],
  ['output_contract', '输出要求'],
  ['constraints', '限制'],
  ['limits', '限制'],
  ['acceptance', '验收条件'],
  ['acceptance_criteria', '验收条件'],
  ['samples', '样例'],
  ['examples', '样例'],
  ['attachments', '公开材料'],
] as const

/** CheckpointItem 检查点列表里的一项:名称、分值、判分方式与公开题面 */
function CheckpointItem({ checkpoint, index }: { checkpoint: StudentCheckpointConfig; index: number }) {
  const shown = new Set<string>()
  const face = FACE_FIELDS.map(([key, label]) => ({ key, label, value: formatFaceValue(checkpoint.face?.[key]) })).filter(
    (item) => {
      if (item.value === '' || shown.has(item.label)) return false
      shown.add(item.label)
      return true
    },
  )
  return (
    <li className="rounded-lg bg-surface p-4 shadow-xs">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <span className="text-base font-medium text-ink">
          <span className="mr-2 tabular-nums text-ink-sub">{index + 1}</span>
          {checkpoint.title || `检查点 ${index + 1}`}
        </span>
        <span className="text-sm font-semibold tabular-nums text-ink">{checkpoint.score} 分</span>
      </div>
      <p className="mt-1 text-sm text-ink-sub">
        {checkpoint.mode === JUDGE_SANDBOX_MODE.REUSE
          ? '在你的实验环境里直接判分'
          : '另起一个干净的环境重新运行你的作答来判分'}
      </p>
      {face.length > 0 ? (
        <div className="mt-3 flex flex-col gap-3 border-t border-line pt-3">
          {face.map((item) => (
            <div key={item.key}>
              <p className="text-sm font-medium text-ink">{item.label}</p>
              <p className="mt-1 whitespace-pre-wrap text-sm leading-relaxed text-ink-sub">{item.value}</p>
            </div>
          ))}
        </div>
      ) : null}
    </li>
  )
}

/** formatFaceValue 将公开题面值转为稳定可读文本,不把内部字段名暴露给学生。 */
function formatFaceValue(value: unknown): string {
  if (typeof value === 'string') return value.trim()
  if (value === undefined || value === null) return ''
  return JSON.stringify(value, null, 2) ?? ''
}
