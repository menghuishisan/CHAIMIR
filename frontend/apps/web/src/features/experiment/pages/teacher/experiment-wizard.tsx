// 实验编排向导页(教师深页,/teacher/experiments/:experimentId 与 /teacher/experiments/new)。设计 §7.4:
// 页头为实验名 + 状态 + 保存状态;下方步骤条(已完成的步骤可点回去,当前步骤写进地址 ?step=);
// 主体左边是当前步骤的表单,≥1280px 时右边是「编排摘要」(各步已配置的内容,点一步跳过去);
// 底部固定操作条「上一步 / 保存并进入下一步」,最后一步为「校验并发布」。离开时有未保存内容要拦一下。
//
// FE-7:进入先拉服务端草稿(新建时先建草稿再进入),每步「下一步」即整体保存并推进
// wizard_step;刷新或换设备回到上次所在步骤(地址里没有 step 时以服务端 wizard_step 为准)。
// 步进用 Steps 组件,不做成手填数字输入(规范 §6.6)。

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { ArrowLeft, ArrowRight, CircleCheck, LayoutTemplate, Send } from 'lucide-react'
import {
  ExperimentCollabMode,
  ExperimentStatus,
  PAGINATION_MAX_SIZE,
  type Course,
  type Experiment,
  type ValidationResult,
} from '@chaimir/api-client'
import {
  Autosave,
  Button,
  Callout,
  DescriptionList,
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
  Steps,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { collectAllPages } from '../../../../utils/allPages'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePageLocation, locationLabel } from '../../../../hooks'
import { experimentStatusLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { UnsavedChangesGuard } from '../../../../components/UnsavedChangesGuard'
import { ExperimentValidationIssues } from '../../components/ExperimentValidationIssues'
import { experimentStatusTone } from '../../statusPresentation'
import { WizardBasicStep } from './wizard-basic'
import { WizardCheckpointsStep } from './wizard-checkpoints'
import { WizardComponentsStep } from './wizard-components'
import { WizardStagesStep } from './wizard-stages'
import {
  WIZARD_STEPS,
  WIZARD_STEP_COUNT,
  draftFromExperiment,
  emptyComponents,
  stageAssignmentError,
  useWizardPersistence,
  type ExperimentDraft,
} from './wizard-state'

/** 新建时的初始草稿:名称留空由教师填,其余给可用的最小配置。 */
function initialDraft(): ExperimentDraft {
  return {
    template_ref: '',
    template_version: '',
    name: '未命名实验',
    description: '',
    components: emptyComponents(),
    collab_mode: ExperimentCollabMode.SOLO,
    group_config: { size: 2, roles: [] },
    require_report: false,
    wizard_step: 1,
  }
}

/**
 * TeacherExperimentWizardPage 承载实验编排向导。
 * 新建路径先在服务端建草稿再跳到编辑路径:草稿有编号后才能按步保存(FE-7)。
 */
export default function TeacherExperimentWizardPage() {
  const { experimentId } = useParams<{ experimentId: string }>()
  const isNew = experimentId === undefined || experimentId === 'new'

  if (isNew) return <NewExperimentBootstrap />
  return <WizardLoader experimentId={experimentId} />
}

/**
 * NewExperimentBootstrap 在服务端创建草稿后跳转到编辑路径。
 * 不在本地攒完整份再一次性创建:那样刷新即丢,违反 FE-7。
 */
function NewExperimentBootstrap() {
  const navigate = useNavigate()
  const [error, setError] = useState<string>()
  const [creating, setCreating] = useState(false)

  const createDraft = useCallback(async () => {
    setCreating(true)
    setError(undefined)
    try {
      const created = await api.experiment.createExperiment(initialDraft())
      navigate(`/teacher/experiments/${created.id}`, { replace: true })
    } catch (createError) {
      setError(userFacingErrorMessage(createError, '创建实验草稿失败,请稍后重试。'))
    } finally {
      setCreating(false)
    }
  }, [navigate])

  usePageLocation([{ label: '新建实验' }])

  return (
    <PageScaffold>
      <PageHeader
        title="新建实验"
        description="创建草稿后按四步完成编排。每一步都会保存到服务器,中途离开也不会丢。"
      />
      <PageSection>
        <div className="flex flex-col gap-4">
          {error ? <Callout tone="danger">{error}</Callout> : null}
          <Callout tone="info" title="编排分四步">
            基础信息 → 环境与仿真 → 实验阶段 → 检查点。全部配置完成后校验并发布。
          </Callout>
          <Button variant="primary" loading={creating} onClick={() => void createDraft()}>
            开始编排
          </Button>
        </div>
      </PageSection>
    </PageScaffold>
  )
}

/**
 * WizardLoader 读取服务端草稿后进入向导。
 * 实验定义走单读、课程清单一次取齐(基础信息步要选所属课程),两者并行。
 */
function WizardLoader({ experimentId }: { experimentId: string }) {
  const view = useAsyncResource(
    () =>
      Promise.all([
        api.experiment.getExperiment(experimentId),
        collectAllPages((params) => api.teaching.getCourses({ role: 'teacher', ...params }), PAGINATION_MAX_SIZE),
      ]).then(([experiment, courses]) => ({ experiment, courses })),
    [experimentId],
    () => false
  )

  usePageLocation([{ label: locationLabel(view, (data) => data.experiment.name, '未命名实验') }])

  return (
    <PageScaffold>
      <ResourceState
        resource={view}
        emptyIcon={LayoutTemplate}
        emptyTitle="实验不存在"
        emptyDescription="这个实验可能已被删除,请回到实验编排查看。"
      >
        {(data) => <WizardContent experiment={data.experiment} courses={data.courses} />}
      </ResourceState>
    </PageScaffold>
  )
}

interface WizardContentProps {
  experiment: Experiment
  courses: Course[]
}

/**
 * WizardContent 渲染步骤指示、当前步表单与步进动作。
 */
function WizardContent({ experiment, courses }: WizardContentProps) {
  const navigate = useNavigate()
  const [draft, setDraft] = useState<ExperimentDraft>(() => draftFromExperiment(experiment))
  const [validateResult, setValidateResult] = useState<ValidationResult>()
  const [publishing, setPublishing] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const [searchParams, setSearchParams] = useSearchParams()
  const urlStep = Number(searchParams.get('step'))
  // 地址里的 step 从 1 起;没有或不合法时用服务端记录的 wizard_step。
  // 地址只能回到服务端记录过的步骤之内,不能靠改地址跳过未完成的步骤
  const serverStep = Math.max(experiment.wizard_step, 1)
  const persistence = useWizardPersistence(
    experiment.id,
    Number.isInteger(urlStep) && urlStep >= 1 && urlStep <= serverStep ? urlStep : serverStep,
    serverStep
  )
  // 当前步骤写进地址(替换历史记录):刷新、分享链接都回到同一步
  useEffect(() => {
    const step = String(persistence.stepIndex + 1)
    if (searchParams.get('step') === step) return
    setSearchParams(
      (previous) => {
        const params = new URLSearchParams(previous)
        params.set('step', step)
        return params
      },
      { replace: true },
    )
  }, [persistence.stepIndex, searchParams, setSearchParams])

  // 未保存改动:与最近一次保存(或进入时)的草稿快照比对
  const [savedSnapshot, setSavedSnapshot] = useState(() => JSON.stringify(draftFromExperiment(experiment)))
  const dirty = useMemo(() => JSON.stringify(draft) !== savedSnapshot, [draft, savedSnapshot])
  /** markSaved 保存成功后把当前草稿记为已保存 */
  const markSaved = useCallback((saved: ExperimentDraft) => {
    setSavedSnapshot(JSON.stringify(saved))
  }, [])

  const patchDraft = useCallback((patch: Partial<ExperimentDraft>) => {
    setDraft((current) => ({ ...current, ...patch }))
  }, [])

  /** validateStep 校验当前步的必填项,返回是否可以进入下一步。 */
  const { errors, validate: validateStep } = useFormValidation(useCallback(() => {
    const stepKey = WIZARD_STEPS[persistence.stepIndex].key
    const next: Record<string, string | null> = {}

    if (stepKey === 'basic') {
      next.name = draft.name.trim() === '' ? '请输入实验名称' : null
      next.description =
        draft.description.trim() === '' ? '请填写实验说明,学生需要知道要做什么' : null
      next.groupSize =
        draft.collab_mode === ExperimentCollabMode.GROUP && draft.group_config.size < 2
          ? '小组实验每组至少 2 人'
          : null
    }

    if (stepKey === 'components') {
      next.components =
        draft.components.envs.length === 0 && draft.components.sims.length === 0
          ? '至少配置一个代码环境或仿真场景,否则学生进入实验后没有内容'
          : null
    }

    if (stepKey === 'stages') {
      next.stages = stageAssignmentError(draft.components)
    }

    if (stepKey === 'checkpoints') {
      next.checkpoints =
        draft.components.checkpoints.length === 0 && !draft.require_report
          ? '实验没有检查点也不要求报告,学生完成后无法产生成绩。请添加检查点或在第 1 步勾选需要报告'
          : null
    }

    return next
  }, [draft, persistence.stepIndex]), persistence.stepIndex)

  const goNext = useCallback(async () => {
    if (!validateStep()) return
    const target = Math.min(persistence.stepIndex + 1, WIZARD_STEP_COUNT - 1)
    if (await persistence.saveStep(draft, target)) markSaved(draft)
  }, [draft, markSaved, persistence, validateStep])

  const saveCurrent = useCallback(async (): Promise<boolean> => {
    // 保存草稿用于持久化尚未完成的编辑态;当前步骤完整性只约束继续和发布校验。
    const saved = await persistence.saveStep(draft, persistence.stepIndex)
    if (saved) markSaved(draft)
    return saved
  }, [draft, markSaved, persistence])

  /** validateAndPublish 先保存当前草稿,再走后端发布前校验。 */
  const validateAndPublish = useCallback(async () => {
    if (!validateStep()) return
    const saved = await persistence.saveStep(draft, persistence.stepIndex)
    if (!saved) return
    markSaved(draft)
    setActionError(undefined)
    try {
      const result = await api.experiment.validateExperiment(experiment.id)
      setValidateResult(result)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '校验没有完成,请稍后重试。'))
    }
  }, [draft, experiment.id, markSaved, persistence, validateStep])

  const publish = useCallback(async () => {
    setPublishing(true)
    setActionError(undefined)
    try {
      await api.experiment.publishExperiment(experiment.id)
      toast.success('实验已发布')
      setValidateResult(undefined)
      navigate('/teacher/experiments')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '发布没有成功,请稍后重试。'))
    } finally {
      setPublishing(false)
    }
  }, [experiment.id, navigate])

  const stepKey = WIZARD_STEPS[persistence.stepIndex].key
  const isLastStep = persistence.stepIndex === WIZARD_STEP_COUNT - 1

  const steps = useMemo(
    () =>
      WIZARD_STEPS.map((step) => ({
        key: step.key,
        label: step.label,
        description: step.description,
      })),
    []
  )

  // 已发布实验的组件快照已经进入学生实例链路,教师只能查看不能再改写。
  // 下架后重新进入草稿编辑链路,由服务端状态门槛与这里的页面门槛共同约束。
  if (experiment.status === ExperimentStatus.PUBLISHED) {
    return <PublishedExperimentView experiment={experiment} />
  }

  return (
    <>
      <PageHeader
        title={draft.name || '未命名实验'}
        status={<StatusIndicator tone={experimentStatusTone(experiment.status)} label={experimentStatusLabel(experiment.status)} />}
        meta={[{ label: '课程', value: courses.find((course) => course.id === draft.course_id)?.name }]}
        actions={
          <div className="flex items-center gap-2">
            {dirty && !persistence.saving ? <span className="text-sm text-ink-sub">有改动未保存</span> : null}
            <Autosave
              state={
                persistence.saving
                  ? 'saving'
                  : persistence.saveError
                    ? 'error'
                    : persistence.savedAt
                      ? 'saved'
                      : 'idle'
              }
              savedAt={persistence.savedAt}
              onRetry={() => void saveCurrent()}
            />
          </div>
        }
      />

      <div className="mb-6">
        <Steps steps={steps} current={persistence.stepIndex} onSelect={persistence.goTo} />
      </div>

      <div className="flex flex-col gap-6 xl:flex-row xl:items-start">
        <div className="flex min-w-0 flex-1 flex-col gap-4">
          {persistence.saveError ? <Callout tone="danger">{persistence.saveError}</Callout> : null}
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          {stepKey === 'basic' ? (
            <WizardBasicStep
              draft={draft}
              courses={courses}
              errors={errors}
              onChange={patchDraft}
            />
          ) : null}
          {stepKey === 'components' ? (
            <WizardComponentsStep draft={draft} errors={errors} onChange={patchDraft} />
          ) : null}
          {stepKey === 'stages' ? (
            <WizardStagesStep draft={draft} errors={errors} onChange={patchDraft} />
          ) : null}
          {stepKey === 'checkpoints' ? (
            <WizardCheckpointsStep draft={draft} errors={errors} onChange={patchDraft} />
          ) : null}
        </div>
        <WizardSummary
          draft={draft}
          current={persistence.stepIndex}
          reached={persistence.reachedIndex}
          onJump={persistence.goTo}
        />
      </div>

      <UnsavedChangesGuard
        when={dirty}
        description="不保存就离开,这一步里改动的内容会丢失。"
        onSave={saveCurrent}
      />

      <div className="sticky bottom-4 z-sticky mt-6 rounded-lg bg-surface px-4 py-3 shadow-md">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <Button
            variant="outline"
            leftIcon={ArrowLeft}
            disabled={persistence.stepIndex === 0}
            onClick={persistence.goBack}
          >
            上一步
          </Button>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="ghost" loading={persistence.saving} onClick={() => void saveCurrent()}>
              保存草稿
            </Button>
            {isLastStep ? (
              <Button
                variant="seal"
                leftIcon={CircleCheck}
                loading={persistence.saving}
                onClick={() => void validateAndPublish()}
              >
                校验并发布
              </Button>
            ) : (
              <Button
                variant="primary"
                leftIcon={ArrowRight}
                loading={persistence.saving}
                onClick={() => void goNext()}
              >
                保存并进入下一步
              </Button>
            )}
          </div>
        </div>
      </div>

      <Modal
        open={validateResult !== undefined}
        onOpenChange={(open) => !open && setValidateResult(undefined)}
      >
        <ModalContent size="lg">
          <ModalHeader>
            <ModalTitle>发布前校验结果</ModalTitle>
            <ModalDescription>
              校验会检查运行时是否可用、题目版本是否锁定、检查点与阶段引用是否完整。
            </ModalDescription>
          </ModalHeader>
          <ModalBody className="flex flex-col gap-3">
            {validateResult?.ok ? (
              <Callout tone="success" title="校验通过">
                依赖完整,可以发布给学生。
              </Callout>
            ) : (
              <Callout tone="warning" title="有需要处理的问题">
                修正后再发布,否则学生进入实验时可能无法准备环境。
              </Callout>
            )}
            <ExperimentValidationIssues issues={validateResult?.issues ?? []} />
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setValidateResult(undefined)}>
              继续修改
            </Button>
            <Button
              variant="seal"
              leftIcon={Send}
              disabled={!validateResult?.ok}
              loading={publishing}
              onClick={() => void publish()}
            >
              确认发布
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </>
  )
}

interface WizardSummaryProps {
  draft: ExperimentDraft
  current: number
  /** 走到过的最远一步;之后的步骤只展示不可点 */
  reached: number
  onJump: (index: number) => void
}

/**
 * WizardSummary ≥1280px 时的「编排摘要」:各步已配置的内容,点一步跳过去(只到走到过的步骤)。
 * 只读当前草稿,不另取数据;当前步高亮。
 */
function WizardSummary({ draft, current, reached, onJump }: WizardSummaryProps) {
  const { envs, sims, stages, checkpoints } = draft.components
  const score = checkpoints.reduce((sum, item) => sum + item.score, 0)
  const lines = [
    draft.collab_mode === ExperimentCollabMode.GROUP ? `小组 ${draft.group_config.size} 人` : '个人完成',
    `${envs.length} 个代码环境 · ${sims.length} 个仿真场景`,
    stages.length > 0 ? `${stages.length} 个阶段` : '不分阶段',
    checkpoints.length > 0 ? `${checkpoints.length} 个检查点 · 共 ${score} 分` : draft.require_report ? '没有检查点,按实验报告计分' : '还没有检查点',
  ]
  return (
    <aside aria-label="编排摘要" className="hidden xl:sticky xl:top-6 xl:block xl:w-72 xl:shrink-0">
      <div className="rounded-lg bg-surface p-4 shadow-xs">
        <p className="mb-2 text-sm font-semibold text-ink">编排摘要</p>
        <ol className="flex flex-col gap-1">
          {WIZARD_STEPS.map((step, index) => (
            <li key={step.key}>
              <button
                type="button"
                onClick={() => onJump(index)}
                disabled={index > reached}
                aria-current={index === current ? 'step' : undefined}
                className={
                  'w-full rounded-md px-2 py-2 text-left transition-colors duration-fast focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent disabled:cursor-default disabled:opacity-60 ' +
                  (index === current ? 'bg-primary-soft' : 'enabled:hover:bg-surface-hover')
                }
              >
                <span className="block text-sm font-medium text-ink">
                  {index + 1}. {step.label}
                </span>
                <span className="block text-xs text-ink-sub">{lines[index]}</span>
              </button>
            </li>
          ))}
        </ol>
      </div>
    </aside>
  )
}

/** PublishedExperimentView 展示已发布实验的只读摘要,避免把向导伪装成可编辑页面。 */
function PublishedExperimentView({ experiment }: { experiment: Experiment }) {
  const components = experiment.components
  const checkpointScore = components.checkpoints.reduce((sum, item) => sum + item.score, 0)

  return (
    <>
      <PageHeader
        title={experiment.name}
        description="实验已经发布,当前页面只展示已冻结的配置摘要。"
        actions={
          <StatusIndicator tone={experimentStatusTone(experiment.status)} label="已发布 · 只读" />
        }
      />
      <PageSection>
        <Callout tone="info" title="发布后配置已锁定">
          学生实例使用发布时冻结的环境、题面和判分配置。需要调整时请先下架,完成修改后重新校验发布。
        </Callout>
      </PageSection>
      <PageSection title="实验说明" description="学生进入实验时看到的说明。">
        <div className="rounded-lg bg-surface p-5 shadow-xs">
          <p className="whitespace-pre-wrap text-sm leading-7 text-ink">
            {experiment.description || '未填写实验说明。'}
          </p>
        </div>
      </PageSection>
      <PageSection title="配置摘要" description="发布快照的结构摘要,不提供修改入口。">
        <div className="rounded-lg bg-surface p-5 shadow-xs">
          <DescriptionList
            dense
            columns={2}
            items={[
              { term: '代码环境', description: `${components.envs.length} 个` },
              { term: '仿真场景', description: `${components.sims.length} 个` },
              { term: '实验阶段', description: `${components.stages.length} 个` },
              { term: '检查点', description: `${components.checkpoints.length} 个` },
              { term: '检查点总分', description: `${checkpointScore} 分` },
              { term: '实验报告', description: experiment.require_report ? '需要提交' : '不需要' },
            ]}
          />
        </div>
      </PageSection>
    </>
  )
}
