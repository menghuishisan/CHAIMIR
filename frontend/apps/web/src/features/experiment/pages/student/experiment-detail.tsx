// 实验详情页(深页,/student/experiments/:experimentId)。
// 这也是实验工作台退出后的回落页(immersiveRoutes 的 exitPath),故必须支持深链与刷新:
// 数据取自学生专用单读 GET /experiment/student/experiments/{id},与列表同一投影。
//
// 阶段与检查点只呈现学生可见部分:分值、解锁条件、阶段内组件数量。
// 判题器编号、题目引用、环境初始化脚本已由后端投影剔除,前端不做二次过滤(铁律 1)。

import { useCallback, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { FlaskConical, Layers, Play, Target } from 'lucide-react'
import {
  EXPERIMENT_STAGE_STATUS,
  ExperimentCollabMode,
  type StudentExperiment,
  type StudentStageConfig,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  ChainProgress,
  DataPanel,
  DescriptionList,
  Empty,
  MetricStrip,
  ObjectIdentity,
  PageBody,
  PageHeader,
  PageScaffold,
  StatusIndicator,
  Table,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { experimentStageStatusLabel, experimentStatusLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { experimentStageStatusTone, experimentStatusTone } from '../../statusPresentation'
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

  return (
    <PageScaffold>
      <ResourceState
        resource={experiment}
        emptyIcon={FlaskConical}
        emptyTitle="实验暂不可用"
        emptyDescription="这个实验可能已被老师下架,请回到实验实训查看其他实验。"
      >
        {(data) => <ExperimentDetailContent experiment={data} />}
      </ResourceState>
    </PageScaffold>
  )
}

/**
 * ExperimentDetailContent 渲染实验说明、阶段清单与进入入口。
 */
function ExperimentDetailContent({ experiment }: { experiment: StudentExperiment }) {
  const navigate = useNavigate()
  const [entering, setEntering] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const isGroup = experiment.collab_mode === ExperimentCollabMode.GROUP
  const needsGroup = isGroup && !experiment.my_group_id
  const checkpointTotal = experiment.components.checkpoints.reduce(
    (sum, checkpoint) => sum + checkpoint.score,
    0,
  )
  const stages = experiment.components.stages

  /** enterExperiment 创建或恢复实例后进入工作台(接口对活跃实例幂等)。 */
  const enterExperiment = useCallback(async () => {
    if (needsGroup) {
      setActionError('这个实验需要分组完成,请等老师把你分到小组后再进入。')
      return
    }
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
  }, [experiment.id, experiment.my_group_id, navigate, needsGroup])

  const stageColumns: TableColumn<StudentStageConfig>[] = [
    { key: 'stage', header: '阶段', align: 'right', mono: true },
    {
      key: 'title',
      header: '阶段名称',
      render: (stage) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{stage.title}</div>
          {stage.description ? (
            <div className="line-clamp-1 text-xs text-ink-sub">{stage.description}</div>
          ) : null}
        </div>
      ),
    },
    {
      key: 'components',
      header: '包含内容',
      render: (stage) => (
        <div className="flex flex-wrap gap-1.5">
          {(stage.components.envs?.length ?? 0) > 0 ? (
            <Badge tone="neutral">代码环境 {stage.components.envs?.length}</Badge>
          ) : null}
          {(stage.components.sims?.length ?? 0) > 0 ? (
            <Badge tone="neutral">仿真场景 {stage.components.sims?.length}</Badge>
          ) : null}
          {(stage.components.envs?.length ?? 0) === 0 &&
          (stage.components.sims?.length ?? 0) === 0 ? (
            <span className="text-ink-sub">—</span>
          ) : null}
        </div>
      ),
    },
    {
      key: 'unlock',
      header: '进入条件',
      render: (stage) => (
        <span className="text-ink-sub">
          {stage.unlock_condition
            ? stage.unlock_condition.type === 'checkpoint'
              ? `通过前一检查点${
                  stage.unlock_condition.min_score
                    ? `并达到 ${stage.unlock_condition.min_score} 分`
                    : ''
                }`
              : '由老师开放'
            : '直接进入'}
        </span>
      ),
    },
    {
      key: 'status',
      header: '当前状态',
      render: () => (
        // 阶段实时状态属实例上下文(工作台内),清单页只说明尚未开始
        <StatusIndicator
          tone={experimentStageStatusTone(EXPERIMENT_STAGE_STATUS.LOCKED)}
          label={experimentStageStatusLabel(EXPERIMENT_STAGE_STATUS.LOCKED)}
        />
      ),
    },
  ]

  return (
    <>
      {/*
        归族:详情族(§6.5.3 第 ④)。h1 由 ObjectIdentity 的实验名承担,
        故页面头只出面包屑,面包屑末节到「实验实训」为止(§6.5.0 通则 1)。
      */}
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '实验实训', href: '/student/experiments' }]} />}
      />

      {/*
        对象身份区:实验名 + 状态 + 关键属性横排 + 主操作(进入实验)。
        「完成方式/实验报告/小组规模」是实验属性,横排在这里就够,不再占 Stat 大卡;
        检查点与阶段这两个可度量数字降为内联摘要,与下方的阶段表、检查点表对照着看。
      */}
      <ObjectIdentity
        name={experiment.name}
        status={
          <StatusIndicator
            tone={experimentStatusTone(experiment.status)}
            label={experimentStatusLabel(experiment.status)}
          />
        }
        subtitle={experiment.description}
        actions={
          <Button
            variant="primary"
            leftIcon={Play}
            loading={entering}
            disabled={needsGroup}
            onClick={() => void enterExperiment()}
          >
            进入实验
          </Button>
        }
        properties={[
          { label: '完成方式', value: isGroup ? `小组 ${experiment.group_config.size} 人` : '独立完成' },
          { label: '实验报告', value: experiment.require_report ? '需要提交' : '不需要' },
          { label: '代码环境', value: `${experiment.components.envs.length} 个` },
          { label: '仿真场景', value: `${experiment.components.sims.length} 个` },
        ]}
      />

      {actionError ? (
        <Callout tone="danger" className="mt-4">
          {actionError}
        </Callout>
      ) : null}
      {needsGroup ? (
        <Callout tone="warning" title="等待老师分组" className="mt-4">
          这是小组实验,老师把你分到小组后才能进入。进入后会为你准备独立的实验环境;
          中途退出再进入会接着上次继续。
        </Callout>
      ) : null}

      <MetricStrip
        label="实验构成摘要"
        className="mt-4 mb-5"
        items={[
          {
            label: '检查点',
            value: experiment.components.checkpoints.length,
            hint: '进入实验后按顺序判分',
          },
          { label: '检查点总分', value: checkpointTotal, hint: '与报告分相加即实验得分' },
          { label: '实验阶段', value: stages.length, hint: '逐阶段解锁' },
        ]}
      />

      <PageBody
        rail={
          isGroup && experiment.my_group_id ? (
            <ExperimentGroupCard groupId={experiment.my_group_id} />
          ) : undefined
        }
      >
        {/* 子域用 Tabs 而不是长条纵向堆叠(§6.5.3 第 ④):阶段与检查点是同一实验的两个视角 */}
        <Tabs defaultValue="stages">
          <TabsList aria-label="实验构成">
            <TabsTrigger value="stages" icon={Layers}>
              实验阶段
            </TabsTrigger>
            <TabsTrigger value="checkpoints" icon={Target}>
              检查点
            </TabsTrigger>
          </TabsList>

          <TabsContent value="stages">
            {/* 列表型子视图走 DataPanel 片段(§6.5.5 B):阶段清单不分页也不筛选,只用片本身 */}
            <DataPanel label="实验阶段">
              <Table
                columns={stageColumns}
                data={stages}
                rowKey={(stage) => String(stage.stage)}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):阶段名一行、包含内容与进入条件一行
                mobileCard={(stage) => ({
                  title: `第 ${stage.stage} 阶段 · ${stage.title}`,
                  meta: stage.description || '按阶段推进,前一阶段的检查点通过后解锁下一阶段。',
                })}
                empty={
                  <Empty
                    icon={Layers}
                    title="这个实验没有分阶段"
                    description="进入后在同一个环境内完成全部检查点。"
                  />
                }
              />
            </DataPanel>
          </TabsContent>

          <TabsContent value="checkpoints">
            <CheckpointList experiment={experiment} />
          </TabsContent>
        </Tabs>
      </PageBody>
    </>
  )
}

/**
 * CheckpointList 列出检查点分值。
 * 判题器与题目引用属答案链路,后端投影已剔除,这里只呈现序号与分值。
 * 这是页内子视图(§6.5.5 B)的只读属性型形态,故用 DescriptionList 而不是再排一张表。
 */
function CheckpointList({ experiment }: { experiment: StudentExperiment }) {
  const checkpoints = experiment.components.checkpoints

  if (checkpoints.length === 0) {
    return (
      <Empty
        icon={Target}
        title="这个实验没有检查点"
        description="完成实验后按老师批改的实验报告计分。"
      />
    )
  }

  return (
    <Card>
      <CardHeader title="检查点" description="每个检查点单独判分,通过后计入实验成绩。" />
      <CardBody className="flex flex-col gap-3">
        <ChainProgress
          total={checkpoints.length}
          done={0}
          label="检查点进度"
        />
        <DescriptionList
          columns={2}
          dense
          items={checkpoints.map((checkpoint, index) => ({
            term: `检查点 ${index + 1}`,
            description: `${checkpoint.score} 分`,
            mono: true,
          }))}
        />
      </CardBody>
    </Card>
  )
}
