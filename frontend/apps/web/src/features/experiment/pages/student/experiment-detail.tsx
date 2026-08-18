// 实验详情页(深页,/student/experiments/:experimentId)。
// 这也是实验工作台退出后的回落页(immersiveRoutes 的 exitPath),故必须支持深链与刷新:
// 数据取自学生专用单读 GET /experiment/student/experiments/{id},与列表同一投影。
//
// 阶段与检查点只呈现学生可见部分:分值、解锁条件、阶段内组件数量。
// 判题器编号、题目引用、环境初始化脚本已由后端投影剔除,前端不做二次过滤(铁律 1)。

import { useCallback, useMemo, useState } from 'react'
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
  DescriptionList,
  Empty,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Stat,
  StatusIndicator,
  Table,
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
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '实验实训', href: '/student/experiments' },
              { label: experiment.name },
            ]}
          />
        }
        title={experiment.name}
        description={experiment.description}
        icon={FlaskConical}
        actions={
          <StatusIndicator
            tone={experimentStatusTone(experiment.status)}
            label={experimentStatusLabel(experiment.status)}
          />
        }
      />

      {/* 指标带只放可度量的检查点与阶段数;完成方式与是否交报告是实验属性,
          在右侧「实验要求」里已逐项列出,不占指标位(规范 §6.5) */}
      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat
            label="检查点"
            value={experiment.components.checkpoints.length}
            icon={Target}
            chain={{ done: 0, total: experiment.components.checkpoints.length }}
            hint="进入实验后按顺序判分"
          />
          <Stat label="检查点总分" value={checkpointTotal} icon={Target} />
          <Stat label="实验阶段" value={stages.length} icon={Layers} />
        </div>
      </PageSection>

      <PageBody
        rail={
          <div className="flex flex-col gap-4">
            <Card>
              <CardHeader
                title="进入实验"
                description="进入后会为你准备独立的实验环境;中途退出再进入会接着上次继续。"
              />
              <CardBody className="flex flex-col gap-3">
                {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
                {needsGroup ? (
                  <Callout tone="warning" title="等待老师分组">
                    这是小组实验,老师把你分到小组后才能进入。
                  </Callout>
                ) : null}
                <Button
                  variant="primary"
                  leftIcon={Play}
                  loading={entering}
                  disabled={needsGroup}
                  onClick={() => void enterExperiment()}
                >
                  进入实验
                </Button>
              </CardBody>
            </Card>

            {isGroup && experiment.my_group_id ? (
              <ExperimentGroupCard groupId={experiment.my_group_id} />
            ) : null}

            <ExperimentRequirementCard experiment={experiment} />
          </div>
        }
      >
        <PageSection title="实验阶段" description="按阶段推进,前一阶段的检查点通过后解锁下一阶段。">
          <Table
            columns={stageColumns}
            data={stages}
            rowKey={(stage) => String(stage.stage)}
            empty={
              <Empty
                icon={Layers}
                title="这个实验没有分阶段"
                description="进入后在同一个环境内完成全部检查点。"
              />
            }
          />
        </PageSection>

        <PageSection title="检查点" description="每个检查点单独判分,通过后计入实验成绩。">
          <CheckpointList experiment={experiment} />
        </PageSection>
      </PageBody>
    </>
  )
}

/**
 * CheckpointList 列出检查点分值。
 * 判题器与题目引用属答案链路,后端投影已剔除,这里只呈现序号与分值。
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

/**
 * ExperimentRequirementCard 说明实验要求:环境、仿真场景与报告。
 */
function ExperimentRequirementCard({ experiment }: { experiment: StudentExperiment }) {
  const items = useMemo(
    () => [
      { term: '代码环境', description: `${experiment.components.envs.length} 个` },
      { term: '仿真场景', description: `${experiment.components.sims.length} 个` },
      { term: '实验报告', description: experiment.require_report ? '需要提交' : '不需要' },
      {
        term: '小组规模',
        description:
          experiment.collab_mode === ExperimentCollabMode.GROUP
            ? `${experiment.group_config.size} 人`
            : '独立完成',
      },
    ],
    [experiment],
  )

  return (
    <Card>
      <CardHeader title="实验要求" />
      <CardBody>
        <DescriptionList dense items={items} />
      </CardBody>
    </Card>
  )
}
