// 实验编排页(教师侧栏,/teacher/experiments)。
// 实验定义是多步骤配置(基础信息 → 组件 → 阶段 → 检查点),wizard_step 落服务端,
// 刷新不丢(FE-7)。步进用 Steps 组件,不做成手填数字输入框
// (旧前端把 wizard_step 做成数字输入是被审查列为 P0 的问题)。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { CircleCheck, ClipboardCheck, FlaskConical, LayoutTemplate, Pencil, Plus, Send, Undo2 } from 'lucide-react'
import { ExperimentStatus, type Experiment, type ValidationResult } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Stat,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import {
  experimentCollabModeLabel,
  experimentStatusLabel,
  experimentStatusTone,
} from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(ExperimentStatus.DRAFT), label: '草稿' },
  { value: String(ExperimentStatus.PUBLISHED), label: '已发布' },
  { value: String(ExperimentStatus.UNPUBLISHED), label: '已下架' },
] as const

/**
 * TeacherExperimentsPage 列出实验定义并承载发布与下架。
 */
export default function TeacherExperimentsPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [validateResult, setValidateResult] = useState<{ experiment: Experiment; result: ValidationResult }>()
  const [publishTarget, setPublishTarget] = useState<Experiment>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const experiments = usePagedResource<Experiment>(
    (params) =>
      api.experiment.getExperiments({
        status: statusFilter ? (Number(statusFilter) as ExperimentStatus) : undefined,
        ...params,
      }),
    [statusFilter],
  )

  /** validateExperiment 发布前校验:把问题列清楚,而不是让发布直接失败。 */
  const validateExperiment = useCallback(async (experiment: Experiment) => {
    setWorking(true)
    setActionError(undefined)
    try {
      const result = await api.experiment.validateExperiment(experiment.id)
      setValidateResult({ experiment, result })
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '校验没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [])

  /** publishExperiment 发布实验:发布后学生可见并可创建实例。 */
  const publishExperiment = useCallback(async () => {
    if (!publishTarget) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.experiment.publishExperiment(publishTarget.id)
      toast.success('实验已发布')
      setPublishTarget(undefined)
      setValidateResult(undefined)
      experiments.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '发布没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [experiments, publishTarget])

  /** unpublishExperiment 下架实验:已有实例不受影响,但学生不能再新建。 */
  const unpublishExperiment = useCallback(
    async (experiment: Experiment) => {
      setActionError(undefined)
      try {
        await api.experiment.unpublishExperiment(experiment.id)
        toast.success('实验已下架')
        experiments.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '下架没有成功,请稍后重试。'))
      }
    },
    [experiments],
  )

  const stats = useMemo(() => {
    const list = experiments.data ? experiments.data.list : []
    return {
      published: list.filter((item) => item.status === ExperimentStatus.PUBLISHED).length,
      draft: list.filter((item) => item.status === ExperimentStatus.DRAFT).length,
      checkpoints: list.reduce((sum, item) => sum + item.components.checkpoints.length, 0),
    }
  }, [experiments.data])

  const columns: TableColumn<Experiment>[] = [
    {
      key: 'name',
      header: '实验',
      render: (experiment) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{experiment.name}</div>
          <div className="line-clamp-1 text-xs text-ink-sub">{experiment.description}</div>
        </div>
      ),
    },
    {
      key: 'components',
      header: '组件',
      render: (experiment) => (
        <div className="flex flex-wrap gap-1.5">
          <Badge tone="neutral">环境 {experiment.components.envs.length}</Badge>
          <Badge tone="neutral">仿真 {experiment.components.sims.length}</Badge>
          <Badge tone="neutral">检查点 {experiment.components.checkpoints.length}</Badge>
        </div>
      ),
    },
    {
      key: 'collab_mode',
      header: '完成方式',
      render: (experiment) => (
        <span className="text-ink-sub">{experimentCollabModeLabel(experiment.collab_mode)}</span>
      ),
    },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (experiment) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(experiment.updated_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (experiment) => (
        <StatusIndicator
          tone={experimentStatusTone(experiment.status)}
          label={experimentStatusLabel(experiment.status)}
        />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (experiment) => (
        <div className="flex justify-end gap-1">
          <Button
            variant="ghost"
            size="sm"
            leftIcon={Pencil}
            onClick={() => navigate(`/teacher/experiments/${experiment.id}`)}
          >
            编排
          </Button>
          <Button
            variant="ghost"
            size="sm"
            leftIcon={ClipboardCheck}
            onClick={() => navigate(`/teacher/experiments/${experiment.id}/reports`)}
          >
            报告与小组
          </Button>
          {experiment.status === ExperimentStatus.PUBLISHED ? (
            <Button variant="ghost" size="sm" leftIcon={Undo2} onClick={() => void unpublishExperiment(experiment)}>
              下架
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={CircleCheck}
              loading={working}
              onClick={() => void validateExperiment(experiment)}
            >
              校验并发布
            </Button>
          )}
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '实践' }, { label: '实验编排' }]} />}
        title="实验编排"
        description="配置实验的代码环境、仿真场景、阶段与检查点。发布前会先校验依赖是否完整。"
        icon={LayoutTemplate}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => navigate('/teacher/experiments/new')}>
            新建实验
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="实验总数" value={experiments.total} icon={FlaskConical} />
          <Stat label="本页已发布" value={stats.published} icon={Send} hint="学生可进入" />
          <Stat label="本页草稿" value={stats.draft} icon={Pencil} />
          <Stat label="本页检查点合计" value={stats.checkpoints} icon={CircleCheck} />
        </div>
      </PageSection>

      <PageSection
        title="实验列表"
        description={`共 ${experiments.total} 个实验`}
        actions={
          <SegmentedControl
            aria-label="按实验状态筛选"
            size="sm"
            options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
            value={statusFilter}
            onValueChange={setStatusFilter}
          />
        }
      >
        <div className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={experiments}
            emptyIcon={LayoutTemplate}
            emptyTitle={statusFilter ? '这个状态下没有实验' : '还没有实验'}
            emptyDescription={
              statusFilter ? '换个状态看看。' : '新建实验后配置环境与检查点,校验通过即可发布给学生。'
            }
            emptyAction={
              statusFilter ? undefined : (
                <Button variant="primary" leftIcon={Plus} onClick={() => navigate('/teacher/experiments/new')}>
                  新建实验
                </Button>
              )
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={experiments.page}
                  pageSize={experiments.pageSize}
                  total={experiments.total}
                  onPageChange={experiments.setPage}
                />
              </>
            )}
          </ResourceState>
        </div>
      </PageSection>

      <Modal
        open={validateResult !== undefined}
        onOpenChange={(open) => !open && setValidateResult(undefined)}
      >
        <ModalContent size="lg">
          <ModalHeader>
            <ModalTitle>发布前校验结果</ModalTitle>
            <ModalDescription>
              校验会检查运行时是否可用、题目版本是否锁定、检查点分值是否合理。
            </ModalDescription>
          </ModalHeader>
          <ModalBody className="flex flex-col gap-3">
            {validateResult ? (
              <>
                <p className="text-base text-ink">{validateResult.experiment.name}</p>
                {validateResult.result.ok ? (
                  <Callout tone="success" title="校验通过">
                    依赖完整,可以发布给学生。
                  </Callout>
                ) : (
                  <Callout tone="warning" title="有需要处理的问题">
                    修正后再发布,否则学生进入实验时可能无法准备环境。
                  </Callout>
                )}
                {validateResult.result.issues.length > 0 ? (
                  <ul className="flex flex-col gap-2">
                    {validateResult.result.issues.map((issue, index) => (
                      <li key={index} className="flex items-start gap-2 text-sm">
                        <Badge tone={issue.level === 'error' ? 'danger' : 'warning'}>
                          {issue.level === 'error' ? '必须修正' : '建议检查'}
                        </Badge>
                        <span className="min-w-0 text-ink">{issue.message}</span>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </>
            ) : null}
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setValidateResult(undefined)}>
              先去修改
            </Button>
            <Button
              variant="seal"
              disabled={!validateResult?.result.ok}
              onClick={() => validateResult && setPublishTarget(validateResult.experiment)}
            >
              确认发布
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <Modal open={publishTarget !== undefined} onOpenChange={(open) => !open && setPublishTarget(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>确认发布实验</ModalTitle>
            <ModalDescription>发布后学生可以进入实验并创建实验环境。</ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="text-base text-ink">{publishTarget?.name}</p>
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setPublishTarget(undefined)}>
              取消
            </Button>
            <Button variant="seal" loading={working} onClick={() => void publishExperiment()}>
              发布实验
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}
