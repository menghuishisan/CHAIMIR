// 实验编排向导页(教师深页,/teacher/experiments/:experimentId 与 /teacher/experiments/new)。
//
// FE-7:进入先拉服务端草稿(新建时先建草稿再进入),每步「下一步」即整体保存并推进
// wizard_step;刷新或换设备回到上次所在步骤。Autosave 显示保存状态。
// 步进用 Steps 组件,不做成手填数字输入(规范 §6.6)。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
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
  Steps,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { experimentStatusLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
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

  return (
    <PageScaffold>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[{ label: '实验编排', href: '/teacher/experiments' }]}
          />
        }
        title="新建实验"
        description="创建草稿后按四步完成编排。每一步都会保存到服务器,中途离开也不会丢。"
        icon={LayoutTemplate}
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
        api.teaching.getCourses({ role: 'teacher', page: 1, size: PAGINATION_MAX_SIZE }),
      ]).then(([experiment, courses]) => ({ experiment, courses: courses.list })),
    [experimentId],
    () => false
  )

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
  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [validateResult, setValidateResult] = useState<ValidationResult>()
  const [publishing, setPublishing] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const persistence = useWizardPersistence(experiment.id, experiment.wizard_step)

  const patchDraft = useCallback((patch: Partial<ExperimentDraft>) => {
    setDraft((current) => ({ ...current, ...patch }))
  }, [])

  /** validateStep 校验当前步的必填项,返回是否可以进入下一步。 */
  const validateStep = useCallback((): boolean => {
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
      // 不分阶段是合法的;分了阶段则每个阶段都必须有内容(阶段表单已校验,此处兜住整体)
      next.stages = draft.components.stages.some(
        (stage) =>
          (stage.components.envs?.length ?? 0) === 0 && (stage.components.sims?.length ?? 0) === 0
      )
        ? '有阶段没有启用任何组件,请补齐或删除该阶段'
        : null
    }

    if (stepKey === 'checkpoints') {
      next.checkpoints =
        draft.components.checkpoints.length === 0 && !draft.require_report
          ? '实验没有检查点也不要求报告,学生完成后无法产生成绩。请添加检查点或在第 1 步勾选需要报告'
          : null
    }

    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [draft, persistence.stepIndex])

  const goNext = useCallback(async () => {
    if (!validateStep()) return
    const target = Math.min(persistence.stepIndex + 1, WIZARD_STEP_COUNT - 1)
    await persistence.saveStep(draft, target)
  }, [draft, persistence, validateStep])

  const saveCurrent = useCallback(async () => {
    if (!validateStep()) return
    await persistence.saveStep(draft, persistence.stepIndex)
  }, [draft, persistence, validateStep])

  /** validateAndPublish 先保存当前草稿,再走后端发布前校验。 */
  const validateAndPublish = useCallback(async () => {
    if (!validateStep()) return
    const saved = await persistence.saveStep(draft, persistence.stepIndex)
    if (!saved) return
    setActionError(undefined)
    try {
      const result = await api.experiment.validateExperiment(experiment.id)
      setValidateResult(result)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '校验没有完成,请稍后重试。'))
    }
  }, [draft, experiment.id, persistence, validateStep])

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

  return (
    <>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '实践' },
              { label: '实验编排', href: '/teacher/experiments' },
            ]}
          />
        }
        title={draft.name || '未命名实验'}
        description="每一步的内容都会保存到服务器,中途离开或换设备都能接着编排。"
        icon={LayoutTemplate}
        actions={
          <div className="flex items-center gap-2">
            <StatusIndicator
              tone={experimentStatusTone(experiment.status)}
              label={experimentStatusLabel(experiment.status)}
            />
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

      <PageSection>
        <Steps steps={steps} current={persistence.stepIndex} />
      </PageSection>

      <PageSection>
        <div className="flex flex-col gap-4">
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
      </PageSection>

      <PageSection>
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
      </PageSection>

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
              disabled={!validateResult?.ok || experiment.status === ExperimentStatus.PUBLISHED}
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
