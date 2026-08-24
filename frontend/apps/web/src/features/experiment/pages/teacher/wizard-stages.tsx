// 实验编排向导:实验阶段步(第 3 步)。
//
// 阶段把实验拆成有先后的几段:每段启用哪些组件、进入条件是什么、以及参数如何从
// 上一段的检查点结果流到下一段(param_bindings)。
// 阶段号由列表顺序决定,不让教师手填 —— 顺序即语义。

import { useCallback, useState } from 'react'
import { ArrowDown, ArrowUp, Layers, Plus, Trash2 } from 'lucide-react'
import type { ParamBinding, StageConfig, UnlockCondition } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  Empty,
  FormField,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  SegmentedControl,
  Select,
  Textarea,
} from '@chaimir/ui'
import type { ExperimentDraft } from './wizard-state'

export interface WizardStagesStepProps {
  draft: ExperimentDraft
  errors: Record<string, string | null>
  onChange: (patch: Partial<ExperimentDraft>) => void
}

/**
 * WizardStagesStep 管理实验阶段。
 * 不分阶段也是合法配置:此时全部组件在同一个环境内同时可用。
 */
export function WizardStagesStep({ draft, errors, onChange }: WizardStagesStepProps) {
  const [modal, setModal] = useState<{ index?: number } | undefined>()

  /** writeStages 写回阶段列表并按顺序重排阶段号:顺序即语义,不留空号。 */
  const writeStages = useCallback(
    (stages: StageConfig[]) => {
      onChange({
        components: {
          ...draft.components,
          stages: stages.map((stage, index) => ({ ...stage, stage: index + 1 })),
        },
      })
    },
    [draft.components, onChange],
  )

  const move = useCallback(
    (index: number, delta: number) => {
      const target = index + delta
      if (target < 0 || target >= draft.components.stages.length) return
      const next = [...draft.components.stages]
      const [moved] = next.splice(index, 1)
      next.splice(target, 0, moved)
      writeStages(next)
    },
    [draft.components.stages, writeStages],
  )

  return (
    <div className="flex flex-col gap-4">
      {errors.stages ? <Callout tone="danger">{errors.stages}</Callout> : null}

      <Card>
        <CardHeader
          title="实验阶段"
          description="按顺序推进的几段实验。前一段的检查点通过后解锁下一段。"
          actions={
            <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setModal({})}>
              添加阶段
            </Button>
          }
        />
        <CardBody>
          {draft.components.stages.length === 0 ? (
            <Empty
              icon={Layers}
              title="不分阶段"
              description="不分阶段时,配置的全部环境与场景在实验开始就一起可用。需要循序推进时再添加阶段。"
              action={
                <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setModal({})}>
                  添加阶段
                </Button>
              }
            />
          ) : (
            <div className="flex flex-col gap-2">
              {draft.components.stages.map((stage, index) => (
                <div key={stage.stage} className="flex flex-col gap-2 well p-3">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge tone="jade">第 {index + 1} 阶段</Badge>
                        <span className="min-w-0 truncate text-base text-ink">{stage.title}</span>
                      </div>
                      {stage.description ? (
                        <p className="mt-1 line-clamp-2 text-xs text-ink-sub">{stage.description}</p>
                      ) : null}
                      <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                        {(stage.components.envs ?? []).map((envId, envIndex) => (
                          <Badge key={envId} tone="neutral">
                            代码环境 {envIndex + 1}
                          </Badge>
                        ))}
                        {(stage.components.sims ?? []).map((simId, simIndex) => (
                          <Badge key={simId} tone="neutral">
                            仿真场景 {simIndex + 1}
                          </Badge>
                        ))}
                        <Badge tone={stage.unlock_condition ? 'info' : 'neutral'}>
                          {unlockSummary(stage.unlock_condition)}
                        </Badge>
                        {(stage.param_bindings?.length ?? 0) > 0 ? (
                          <Badge tone="info">参数传递 {stage.param_bindings?.length}</Badge>
                        ) : null}
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      <IconButton
                        variant="ghost"
                        size="sm"
                        icon={ArrowUp}
                        aria-label={`把第 ${index + 1} 阶段往前移`}
                        disabled={index === 0}
                        onClick={() => move(index, -1)}
                      />
                      <IconButton
                        variant="ghost"
                        size="sm"
                        icon={ArrowDown}
                        aria-label={`把第 ${index + 1} 阶段往后移`}
                        disabled={index === draft.components.stages.length - 1}
                        onClick={() => move(index, 1)}
                      />
                      <Button variant="ghost" size="sm" onClick={() => setModal({ index })}>
                        编辑
                      </Button>
                      <IconButton
                        variant="ghost"
                        size="sm"
                        icon={Trash2}
                        aria-label={`删除第 ${index + 1} 阶段`}
                        onClick={() =>
                          writeStages(draft.components.stages.filter((_, i) => i !== index))
                        }
                      />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardBody>
      </Card>

      {modal ? (
        <StageFormModal
          stage={modal.index !== undefined ? draft.components.stages[modal.index] : undefined}
          stageNumber={modal.index !== undefined ? modal.index + 1 : draft.components.stages.length + 1}
          draft={draft}
          onClose={() => setModal(undefined)}
          onSave={(stage) => {
            const stages =
              modal.index !== undefined
                ? draft.components.stages.map((item, i) => (i === modal.index ? stage : item))
                : [...draft.components.stages, stage]
            writeStages(stages)
            setModal(undefined)
          }}
        />
      ) : null}
    </div>
  )
}

/** unlockSummary 用一句话说明阶段的进入条件。 */
function unlockSummary(condition: UnlockCondition | undefined): string {
  if (!condition) return '直接进入'
  if (condition.type === 'manual') return '由老师开放'
  return condition.min_score
    ? `检查点达到 ${condition.min_score} 分后进入`
    : '通过指定检查点后进入'
}

interface StageFormModalProps {
  stage?: StageConfig
  stageNumber: number
  draft: ExperimentDraft
  onClose: () => void
  onSave: (stage: StageConfig) => void
}

/**
 * StageFormModal 配置单个阶段:启用组件、进入条件与参数传递。
 */
function StageFormModal({ stage, stageNumber, draft, onClose, onSave }: StageFormModalProps) {
  const editing = stage !== undefined
  const [title, setTitle] = useState(stage?.title ?? '')
  const [description, setDescription] = useState(stage?.description ?? '')
  const [envIds, setEnvIds] = useState<string[]>(stage?.components.envs ?? [])
  const [simIds, setSimIds] = useState<string[]>(stage?.components.sims ?? [])
  const [unlockType, setUnlockType] = useState<'none' | 'checkpoint' | 'manual'>(
    stage?.unlock_condition ? stage.unlock_condition.type : 'none',
  )
  const [unlockCheckpoint, setUnlockCheckpoint] = useState(
    stage?.unlock_condition?.checkpoint_id ?? '',
  )
  const [minScore, setMinScore] = useState(String(stage?.unlock_condition?.min_score ?? ''))
  const [bindings, setBindings] = useState<ParamBinding[]>(stage?.param_bindings ?? [])
  const [formError, setFormError] = useState<string>()

  const submit = useCallback(() => {
    if (title.trim() === '') {
      setFormError('请输入阶段名称')
      return
    }
    if (envIds.length === 0 && simIds.length === 0) {
      setFormError('请至少启用一个环境或场景,否则这个阶段没有内容')
      return
    }
    if (unlockType === 'checkpoint' && unlockCheckpoint === '') {
      setFormError('请选择作为进入条件的检查点')
      return
    }
    setFormError(undefined)

    const unlock: UnlockCondition | undefined =
      unlockType === 'none'
        ? undefined
        : unlockType === 'manual'
          ? { type: 'manual' }
          : {
              type: 'checkpoint',
              checkpoint_id: unlockCheckpoint,
              min_score: minScore === '' ? undefined : Number(minScore),
            }

    onSave({
      stage: stageNumber,
      title: title.trim(),
      description: description.trim() === '' ? undefined : description.trim(),
      components: { envs: envIds, sims: simIds },
      unlock_condition: unlock,
      param_bindings: bindings.length > 0 ? bindings : undefined,
    })
  }, [
    bindings,
    description,
    envIds,
    minScore,
    onSave,
    simIds,
    stageNumber,
    title,
    unlockCheckpoint,
    unlockType,
  ])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? `编辑第 ${stageNumber} 阶段` : `添加第 ${stageNumber} 阶段`}</ModalTitle>
          <ModalDescription>
            选择这个阶段启用哪些组件、什么条件下解锁,以及是否把上一阶段的判分结果作为参数传入。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField label="阶段名称" htmlFor="stage-title" required>
            <Input
              id="stage-title"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
            />
          </FormField>

          <FormField label="阶段说明" htmlFor="stage-description" helper="学生在工作台左栏看到的本阶段目标">
            <Textarea
              id="stage-description"
              value={description}
              rows={3}
              onChange={(event) => setDescription(event.target.value)}
            />
          </FormField>

          <FormField label="启用的代码环境" helper="不勾选表示这个阶段不用代码环境">
            {draft.components.envs.length === 0 ? (
              <Callout tone="info">上一步还没有配置代码环境。</Callout>
            ) : (
              <div className="flex flex-col gap-2">
                {draft.components.envs.map((env, index) => (
                  <Checkbox
                    key={env.id}
                    checked={envIds.includes(env.id)}
                    label={`代码环境 ${index + 1} · ${env.primary_runtime.runtime_code}`}
                    onCheckedChange={(checked) =>
                      setEnvIds((current) =>
                        checked === true ? [...current, env.id] : current.filter((id) => id !== env.id),
                      )
                    }
                  />
                ))}
              </div>
            )}
          </FormField>

          <FormField label="启用的仿真场景" helper="不勾选表示这个阶段不用仿真">
            {draft.components.sims.length === 0 ? (
              <Callout tone="info">上一步还没有配置仿真场景。</Callout>
            ) : (
              <div className="flex flex-col gap-2">
                {draft.components.sims.map((sim, index) => (
                  <Checkbox
                    key={sim.id}
                    checked={simIds.includes(sim.id)}
                    label={`仿真场景 ${index + 1} · ${sim.package_code}`}
                    onCheckedChange={(checked) =>
                      setSimIds((current) =>
                        checked === true ? [...current, sim.id] : current.filter((id) => id !== sim.id),
                      )
                    }
                  />
                ))}
              </div>
            )}
          </FormField>

          <FormField label="进入条件" required>
            <SegmentedControl
              aria-label="阶段进入条件"
              options={[
                { value: 'none', label: '直接进入' },
                { value: 'checkpoint', label: '通过检查点后' },
                { value: 'manual', label: '由老师开放' },
              ]}
              value={unlockType}
              onValueChange={(value) => setUnlockType(value as 'none' | 'checkpoint' | 'manual')}
            />
          </FormField>

          {unlockType === 'checkpoint' ? (
            <div className="grid gap-4 well p-4 sm:grid-cols-2">
              <FormField label="作为条件的检查点" htmlFor="stage-unlock-checkpoint" required>
                {draft.components.checkpoints.length === 0 ? (
                  <Callout tone="warning">
                    还没有检查点。请先完成第 4 步的检查点配置,再回来设置进入条件。
                  </Callout>
                ) : (
                  <Select
                    id="stage-unlock-checkpoint"
                    options={draft.components.checkpoints.map((checkpoint, index) => ({
                      value: checkpoint.id,
                      label: `检查点 ${index + 1} · ${checkpoint.score} 分`,
                    }))}
                    value={unlockCheckpoint}
                    placeholder="选择检查点"
                    onValueChange={setUnlockCheckpoint}
                  />
                )}
              </FormField>
              <FormField
                label="最低得分"
                htmlFor="stage-unlock-score"
                helper="留空表示通过即可,不要求分数"
              >
                <Input
                  id="stage-unlock-score"
                  type="number"
                  min="0"
                  value={minScore}
                  onChange={(event) => setMinScore(event.target.value)}
                />
              </FormField>
            </div>
          ) : null}

          <ParamBindingsEditor draft={draft} bindings={bindings} onChange={setBindings} />

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" onClick={submit}>
            {editing ? '保存阶段' : '添加阶段'}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface ParamBindingsEditorProps {
  draft: ExperimentDraft
  bindings: ParamBinding[]
  onChange: (bindings: ParamBinding[]) => void
}

/**
 * ParamBindingsEditor 配置参数传递。
 * 把上一阶段检查点的判分输出接到本阶段某个组件的参数上,或直接给一个固定值 ——
 * 这让「用第一阶段部署的合约地址跑第二阶段的攻击」这类编排成为可能。
 */
function ParamBindingsEditor({ draft, bindings, onChange }: ParamBindingsEditorProps) {
  const componentOptions = [
    ...draft.components.envs.map((env, index) => ({ value: env.id, label: `代码环境 ${index + 1}` })),
    ...draft.components.sims.map((sim, index) => ({ value: sim.id, label: `仿真场景 ${index + 1}` })),
  ]

  const update = useCallback(
    (index: number, patch: Partial<ParamBinding>) => {
      onChange(bindings.map((item, i) => (i === index ? { ...item, ...patch } : item)))
    },
    [bindings, onChange],
  )

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="text-sm font-medium text-ink">参数传递(可选)</div>
          <p className="text-xs text-ink-sub">
            把上一阶段检查点的结果接到本阶段组件的参数上,例如把部署得到的合约地址传给下一阶段。
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          leftIcon={Plus}
          disabled={componentOptions.length === 0}
          onClick={() =>
            onChange([
              ...bindings,
              {
                target_component: componentOptions[0]?.value ?? '',
                target_param: '',
                source_type: 'checkpoint',
                source_ref: draft.components.checkpoints[0]?.id ?? '',
                source_path: '',
              },
            ])
          }
        >
          添加一条
        </Button>
      </div>

      {bindings.map((binding, index) => (
        <div key={index} className="flex flex-col gap-3 well p-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <FormField label="传给哪个组件" htmlFor={`binding-target-${index}`} required>
              <Select
                id={`binding-target-${index}`}
                size="sm"
                options={componentOptions}
                value={binding.target_component}
                onValueChange={(value) => update(index, { target_component: value })}
              />
            </FormField>
            <FormField
              label="配置项名称"
              htmlFor={`binding-param-${index}`}
              required
              helper="要配置的项目名称,可在仿真包或运行时文档中查阅"
            >
              <Input
                id={`binding-param-${index}`}
                value={binding.target_param}
                onChange={(event) => update(index, { target_param: event.target.value })}
              />
            </FormField>
          </div>

          <FormField label="取值来源" required>
            <SegmentedControl
              aria-label="取值来源"
              size="sm"
              options={[
                { value: 'checkpoint', label: '检查点结果' },
                { value: 'constant', label: '固定值' },
              ]}
              value={binding.source_type}
              onValueChange={(value) =>
                update(index, { source_type: value as 'checkpoint' | 'constant' })
              }
            />
          </FormField>

          {binding.source_type === 'checkpoint' ? (
            <div className="grid gap-3 sm:grid-cols-2">
              <FormField label="来源检查点" htmlFor={`binding-source-${index}`} required>
                {draft.components.checkpoints.length === 0 ? (
                  <Callout tone="warning">还没有检查点,请先完成第 4 步。</Callout>
                ) : (
                  <Select
                    id={`binding-source-${index}`}
                    size="sm"
                    options={draft.components.checkpoints.map((checkpoint, checkpointIndex) => ({
                      value: checkpoint.id,
                      label: `检查点 ${checkpointIndex + 1}`,
                    }))}
                    value={binding.source_ref ?? ''}
                    onValueChange={(value) => update(index, { source_ref: value })}
                  />
                )}
              </FormField>
              <FormField
                label="提取哪项数据"
                htmlFor={`binding-path-${index}`}
                helper="检查点输出中的数据项名称,例如 contract_address"
              >
                <Input
                  id={`binding-path-${index}`}
                  value={binding.source_path ?? ''}
                  onChange={(event) => update(index, { source_path: event.target.value })}
                />
              </FormField>
            </div>
          ) : (
            <FormField
              label="固定值"
              htmlFor={`binding-constant-${index}`}
              required
              helper="直接传给组件的值"
            >
              <Input
                id={`binding-constant-${index}`}
                value={typeof binding.constant_value === 'string' ? binding.constant_value : ''}
                onChange={(event) => update(index, { constant_value: event.target.value })}
              />
            </FormField>
          )}

          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onChange(bindings.filter((_, i) => i !== index))}
          >
            移除这条
          </Button>
        </div>
      ))}
    </div>
  )
}
