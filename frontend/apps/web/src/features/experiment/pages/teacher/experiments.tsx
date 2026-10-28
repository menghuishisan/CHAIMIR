// 实验编排页(教师侧栏,/teacher/experiments)。
// 实验定义是多步骤配置(基础信息 → 组件 → 阶段 → 检查点),wizard_step 落服务端,
// 刷新不丢(FE-7)。步进用 Steps 组件,不做成手填数字输入框
// (旧前端把 wizard_step 做成数字输入是被审查列为 P0 的问题)。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import {
  CircleCheck,
  ClipboardCheck,
  LayoutTemplate,
  MoreVertical,
  Pencil,
  Plus,
  Undo2,
} from 'lucide-react'
import {
  ExperimentStatus,
  type Experiment,
  type ValidationResult,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  FilterBar,
  FilterField,
  IconButton,
  Input,
  MetricStrip,
  Menu,
  MenuContent,
  MenuItem,
  MenuSeparator,
  MenuTrigger,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import {
  experimentCollabModeLabel,
} from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ExperimentIdentityCell, ExperimentStatusCell } from '../../components/ExperimentTableCells'
import { ExperimentValidationIssues } from '../../components/ExperimentValidationIssues'

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
  const [keyword, setKeyword] = useState('')
  const [submittedKeyword, setSubmittedKeyword] = useState('')
  const [validateResult, setValidateResult] = useState<{
    experiment: Experiment
    result: ValidationResult
  }>()
  const [publishTarget, setPublishTarget] = useState<Experiment>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const experiments = usePagedResource<Experiment>(
    (params) =>
      api.experiment.getExperiments({
        status: statusFilter ? (Number(statusFilter) as ExperimentStatus) : undefined,
        keyword: submittedKeyword || undefined,
        ...params,
      }),
    [statusFilter, submittedKeyword]
  )

  const handleSearch = useCallback(() => {
    setSubmittedKeyword(keyword.trim())
  }, [keyword])

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
    [experiments]
  )

  // 指标带取服务端全量口径,不随下方状态筛选变化。
  // 「检查点合计」需要跨全部实验累加,服务端没有这个聚合,故不做这张卡:
  // 每个实验各有多少检查点在列表行里可见(规范 §6.5 指标带口径)。
  const totalCount = useResourceTotal((params) => api.experiment.getExperiments(params), [], experiments.refreshVersion)
  const publishedCount = useResourceTotal(
    (params) => api.experiment.getExperiments({ status: ExperimentStatus.PUBLISHED, ...params }),
    [],
    experiments.refreshVersion
  )
  const draftCount = useResourceTotal(
    (params) => api.experiment.getExperiments({ status: ExperimentStatus.DRAFT, ...params }),
    [],
    experiments.refreshVersion
  )
  const unpublishedCount = useResourceTotal(
    (params) => api.experiment.getExperiments({ status: ExperimentStatus.UNPUBLISHED, ...params }),
    [],
    experiments.refreshVersion
  )

  const columns: TableColumn<Experiment>[] = [
    {
      key: 'name',
      header: '实验',
      render: (experiment) => <ExperimentIdentityCell experiment={experiment} />,
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
      render: (experiment) => <ExperimentStatusCell experiment={experiment} />,
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
            <Button
              variant="ghost"
              size="sm"
              leftIcon={Undo2}
              onClick={() => void unpublishExperiment(experiment)}
            >
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
        kicker={<Breadcrumb items={[{ label: '实践' }]} />}
        title="实验编排"
        description="配置实验的代码环境、仿真场景、阶段与检查点。发布前会先校验依赖是否完整。"
        icon={LayoutTemplate}
        actions={
          <Button
            variant="primary"
            leftIcon={Plus}
            onClick={() => navigate('/teacher/experiments/new')}
          >
            新建实验
          </Button>
        }
      />

      {/* 指标降为内联摘要(§6.5.3 第 ① 族):本页主体是实验列表 */}
      <MetricStrip
        label="实验总量摘要"
        className="mb-5"
        items={[
          { label: '实验总数', value: totalCount ?? '—', hint: '不受下方筛选影响' },
          { label: '已发布', value: publishedCount ?? '—', hint: '学生可进入' },
          { label: '草稿', value: draftCount ?? '—', hint: '校验通过后才能发布' },
          { label: '已下架', value: unpublishedCount ?? '—', hint: '学生不可再进入' },
        ]}
      />

      {/* 动作失败就近内联(§6.7 C) */}
      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {/* 筛选井、数据表、分页同处一块抬起片(§6.5.2) */}
      <DataPanel
        label="实验列表"
        filter={
          <FilterBar label="实验筛选" onSubmit={handleSearch} submitLabel="搜索">
            <FilterField label="关键词" htmlFor="teacher-experiment-keyword">
              <Input
                id="teacher-experiment-keyword"
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder="搜索实验名称或说明"
              />
            </FilterField>
            <FilterField label="实验状态" group>
              <SegmentedControl
                aria-label="按实验状态筛选"
                size="sm"
                options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={statusFilter}
                onValueChange={setStatusFilter}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={experiments.page}
            pageSize={experiments.pageSize}
            total={experiments.total}
            onPageChange={experiments.setPage}
          />
        }
      >
          <ResourceState
            resource={experiments}
            emptyIcon={LayoutTemplate}
            emptyTitle={submittedKeyword || statusFilter ? '没有匹配的实验' : '还没有实验'}
            emptyDescription={
              submittedKeyword || statusFilter
                ? '换个关键词或状态看看。'
                : '新建实验后配置环境与检查点,校验通过即可发布给学生。'
            }
            emptyAction={
              statusFilter ? undefined : (
                <Button
                  variant="primary"
                  leftIcon={Plus}
                  onClick={() => navigate('/teacher/experiments/new')}
                >
                  新建实验
                </Button>
              )
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):实验名一行、所属课程与检查点数一行
                mobileCard={(item) => ({
                  title: item.name,
                  meta: `${experimentCollabModeLabel(item.collab_mode)} · 检查点 ${item.components.checkpoints.length} · ${formatShortDateTime(item.updated_at)}`,
                  badge: <ExperimentStatusCell experiment={item} />,
                  action: (
                    <Menu>
                      <MenuTrigger asChild>
                        <IconButton variant="ghost" size="sm" icon={MoreVertical} aria-label={`${item.name} 的更多操作`} />
                      </MenuTrigger>
                      <MenuContent align="end">
                        <MenuItem icon={Pencil} onSelect={() => navigate(`/teacher/experiments/${item.id}`)}>
                          编排实验
                        </MenuItem>
                        <MenuItem
                          icon={ClipboardCheck}
                          onSelect={() => navigate(`/teacher/experiments/${item.id}/reports`)}
                        >
                          报告与小组
                        </MenuItem>
                        <MenuSeparator />
                        {item.status === ExperimentStatus.PUBLISHED ? (
                          <MenuItem icon={Undo2} onSelect={() => void unpublishExperiment(item)}>
                            下架实验
                          </MenuItem>
                        ) : (
                          <MenuItem icon={CircleCheck} onSelect={() => void validateExperiment(item)}>
                            校验并发布
                          </MenuItem>
                        )}
                      </MenuContent>
                    </Menu>
                  ),
                })}
              />
            )}
          </ResourceState>
      </DataPanel>

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
                <ExperimentValidationIssues issues={validateResult.result.issues} />
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

      <Modal
        open={publishTarget !== undefined}
        onOpenChange={(open) => !open && setPublishTarget(undefined)}
      >
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
