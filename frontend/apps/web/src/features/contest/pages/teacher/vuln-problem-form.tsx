// 按服务端草稿恢复漏洞题编辑，暂存初始化、攻击、断言和运行环境。

import { useId } from 'react'
import { Bug, Plus, Trash2 } from 'lucide-react'
import {
  VulnRuntimeMode,
  VulnProblemStatus,
  type VulnProblem,
  type VulnSource,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Empty,
  FormField,
  IconButton,
  Input,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  SegmentedControl,
  Select,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useAsyncResource } from '../../../../hooks'
import { ResourceState } from '../../../../components/ResourceState'
import { UnsavedChangesGuard } from '../../../../components/UnsavedChangesGuard'
import { CompositionDeclarationFields } from '../../../sandbox/components/CompositionDeclarationFields'
import {
  vulnAssertOpLabel,
  vulnChainOpLabel,
  vulnLevelLabel,
  vulnRuntimeModeLabel,
} from '../../../../utils/labels/contest'
import { parseVulnJSONValue } from '../../vulnerabilityContract'
import { useVulnProblemDraft } from '../../useVulnProblemDraft'
import { VulnPrevalidateModal } from './vuln-prevalidate'
import { type ChainStep, type AssertionDraft, type VulnProblemFields, vulnProblemFields } from '../../vulnProblemDraft'
import {
  VULN_ASSERT_OPS,
  VULN_CHAIN_OPS,
  VULN_LEVELS,
  VULN_RUNTIME_MODES,
  type VulnAssertOp,
  type VulnChainOp,
} from '../../options'
export interface VulnProblemFormModalProps {
  problemId: string
  sources: VulnSource[]
  onClose: () => void
  onSaved: () => void
}

/** 先读取服务端草稿；加载失败时不以本机内容开启编辑。 */
export function VulnProblemFormModal(props: VulnProblemFormModalProps) {
  const resource = useAsyncResource(async () => {
    const problem = await api.contest.getVulnProblem(props.problemId)
    vulnProblemFields(problem)
    return problem
  }, [props.problemId])
  if (resource.status !== 'success' || !resource.data) return (
    <Drawer open onOpenChange={(open) => !open && props.onClose()}>
      <DrawerContent size="wide">
        <DrawerHeader><DrawerTitle>编辑漏洞题</DrawerTitle><DrawerDescription>继续编辑前需读取已保存的草稿。</DrawerDescription></DrawerHeader>
        <DrawerBody><ResourceState resource={resource} emptyIcon={Bug} emptyTitle="没有找到漏洞题" emptyDescription="请返回工坊重新选择。">{() => null}</ResourceState></DrawerBody>
      </DrawerContent>
    </Drawer>
  )
  if (resource.data.status !== VulnProblemStatus.DRAFT) return (
    <VulnPrevalidateModal problem={resource.data} onClose={props.onClose} onDone={props.onSaved} />
  )
  return <VulnProblemForm key={props.problemId} {...props} problem={resource.data} />
}

/** 编辑完整服务端草稿，半成品自动暂存，关闭前等待当前写回。 */
function VulnProblemForm({ problem, sources, onClose, onSaved }: VulnProblemFormModalProps & { problem: VulnProblem }) {
  const fieldId = useId()
  const draft = useVulnProblemDraft(problem)
  const { title, judgerCode, maxScore, sourceId, externalRef, level, runtimeMode, description, contractSource, forkBlock, initSteps, positiveSteps, assertions, declaration } = draft.fields
  // 同一编辑入口负责记录未同步输入，避免各字段分别维护保存逻辑。
  function change<K extends keyof VulnProblemFields>(key: K) {
    return (value: VulnProblemFields[K]) => draft.update({ [key]: value })
  }
  const setTitle = change('title'), setJudgerCode = change('judgerCode'), setMaxScore = change('maxScore')
  const setSourceId = change('sourceId'), setExternalRef = change('externalRef'), setLevel = change('level')
  const setRuntimeMode = change('runtimeMode'), setDescription = change('description'), setContractSource = change('contractSource')
  const setForkBlock = change('forkBlock'), setInitSteps = change('initSteps'), setPositiveSteps = change('positiveSteps'), setAssertions = change('assertions')
  const isForked = Number(runtimeMode) === VulnRuntimeMode.FORKED
  const errors: Record<string, string | null> = {
      title: title.trim() === '' ? '请输入漏洞题标题' : [...title.trim()].length > 255 ? '标题不能超过 255 个字' : null,
      judgerCode: judgerCode.trim() === '' ? '请输入平台已注册的判题器编码' : null,
      maxScore: !Number.isInteger(Number(maxScore)) || Number(maxScore) <= 0 ? '满分必须是正整数' : null,
      description: description.trim() === '' ? '请写下漏洞说明,学生要知道这道题在考什么' : null,
      forkBlock:
        isForked && (forkBlock.trim() === '' || !Number.isFinite(Number(forkBlock)))
          ? '主网分叉复现需要锁定一个区块高度'
          : null,
      assertions: assertions.length === 0 ? '至少保留一条判定断言'
        : assertions.some((item) => item.target.trim() === '' || assertionValueError(item))
          ? '请检查每条断言的查询目标和期望值' : null,
      steps: [...initSteps, ...positiveSteps].some((step) => !isValidStep(step))
        ? '有步骤的信息不完整或格式不正确，请检查查询目标和参数'
        : null,
  }

  // 手动保存与自动保存共用队列，失败时继续保留输入。
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (await draft.save()) toast.success('草稿已保存')
  }

  async function close() {
    if (await draft.save()) { onSaved(); onClose() }
  }

  return (
    <Drawer open onOpenChange={(open) => !open && void close()}>
      <UnsavedChangesGuard when={draft.dirty} description="尚未同步的编辑仅保留在本机，保存后可在其他设备继续。" onSave={draft.save} />
      <DrawerContent size="wide">
        <DrawerHeader>
          <DrawerTitle>编辑漏洞题</DrawerTitle>
          <DrawerDescription>
            编辑会自动保存，未填完也可暂存。修改后需要重新预验证，通过后才能固化到题库。
          </DrawerDescription>
        </DrawerHeader>
        <form onSubmit={submit} noValidate className="flex min-h-0 flex-1 flex-col">
          <DrawerBody className="flex flex-col gap-4">
            <p role="status" className="text-sm text-ink-sub">{draft.status === 'saved' ? '已保存到服务器' : draft.status === 'saving' ? '正在保存' : '还有编辑尚未同步到服务器'}</p>
            {draft.error ? <Callout tone="danger">{draft.error}</Callout> : null}
            {draft.cacheError ? <Callout tone="warning">{draft.cacheError}</Callout> : null}
            {draft.status === 'conflict' ? <Callout tone="warning" title="草稿已有更新">
              <p>当前输入已保留。覆盖将用当前输入替换服务器最新草稿；使用服务器内容将丢弃本机未同步的修改。</p>
              <div className="mt-3 flex flex-wrap gap-2">
                <Button variant="outline" onClick={() => void draft.resolveConflict(true)}>用当前输入覆盖服务器草稿</Button>
                <Button variant="outline" onClick={() => void draft.resolveConflict(false)}>使用服务器内容</Button>
              </div>
            </Callout> : null}
            <fieldset disabled={draft.status === 'conflict'} className="flex min-w-0 flex-col gap-4">
            <FormField label="标题" htmlFor={`${fieldId}-title`} required error={errors.title}>
              <Input
                id={`${fieldId}-title`}
                value={title}
                invalid={Boolean(errors.title)}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="判题器编码" htmlFor={`${fieldId}-judger`} required error={errors.judgerCode} helper="填写平台已注册的判题器编码">
                <Input id={`${fieldId}-judger`} value={judgerCode} invalid={Boolean(errors.judgerCode)} onChange={(event) => setJudgerCode(event.target.value)} />
              </FormField>
              <FormField label="题目满分" htmlFor={`${fieldId}-max-score`} required error={errors.maxScore}>
                <Input id={`${fieldId}-max-score`} type="number" min={1} step={1} value={maxScore} invalid={Boolean(errors.maxScore)} onChange={(event) => setMaxScore(event.target.value)} />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="归属来源" htmlFor={`${fieldId}-source`} helper="手工录入可以不选">
                <Select
                  id={`${fieldId}-source`}
                  options={[
                    { value: 'none', label: '不归属任何来源' },
                    ...sources.map((source) => ({ value: source.id, label: source.name })),
                  ]}
                  value={sourceId || 'none'}
                  placeholder="不归属任何来源"
                  onValueChange={(value) => setSourceId(value === 'none' ? '' : value)}
                />
              </FormField>
              <FormField
                label="案例编号"
                htmlFor={`${fieldId}-ref`}
                helper="外部漏洞编号,如 SWC-107 或 CVE 编号;用于去重"
              >
                <Input
                  id={`${fieldId}-ref`}
                  value={externalRef}
                  onChange={(event) => setExternalRef(event.target.value)}
                />
              </FormField>
            </div>

            <FormField label="可复现性分级" required helper="A 级可自动转链上题,C 级只作理论素材">
              <SegmentedControl
                aria-label="可复现性分级"
                options={VULN_LEVELS.map((item) => ({
                  value: String(item),
                  label: vulnLevelLabel(item),
                }))}
                value={level}
                onValueChange={setLevel}
              />
            </FormField>

            <FormField
              label="复现方式"
              required
              helper="干净测试链适合可本地重建的漏洞;主网分叉适合依赖历史链状态的真实事件"
            >
              <SegmentedControl
                aria-label="复现方式"
                options={VULN_RUNTIME_MODES.map((item) => ({
                  value: String(item),
                  label: vulnRuntimeModeLabel(item),
                }))}
                value={runtimeMode}
                onValueChange={setRuntimeMode}
              />
            </FormField>

            {isForked ? (
              <FormField
                label="锁定区块高度"
                htmlFor={`${fieldId}-fork`}
                required
                error={errors.forkBlock}
                helper="从这个高度派生隔离链,锁定后不依赖外部节点实时可用"
              >
                <Input
                  id={`${fieldId}-fork`}
                  type="number"
                  min="1"
                  value={forkBlock}
                  invalid={Boolean(errors.forkBlock)}
                  onChange={(event) => setForkBlock(event.target.value)}
                />
              </FormField>
            ) : null}

            <FormField
              label="漏洞说明"
              htmlFor={`${fieldId}-desc`}
              required
              error={errors.description}
              helper="学生看到的题面说明。写清背景与目标,不要写出攻击答案"
            >
              <Textarea
                id={`${fieldId}-desc`}
                value={description}
                rows={4}
                invalid={Boolean(errors.description)}
                onChange={(event) => setDescription(event.target.value)}
              />
            </FormField>

            <FormField
              label="合约源码"
              htmlFor={`${fieldId}-source-code`}
              helper="有漏洞的合约源码。随题固化保存,之后不依赖外部来源"
            >
              <Textarea
                id={`${fieldId}-source-code`}
                value={contractSource}
                rows={6}
                className="font-mono text-xs"
                onChange={(event) => setContractSource(event.target.value)}
              />
            </FormField>

            <ChainStepEditor
              title="初始化步骤"
              description="预验证开始时先执行这些步骤,把环境准备到漏洞可被触发的状态。正向与反向验证都会执行。"
              steps={initSteps}
              onChange={setInitSteps}
            />

            <ChainStepEditor
              title="攻击步骤(官方 PoC)"
              description="只在正向验证时执行。反向验证故意不执行这些步骤,以确认断言不会误判。"
              steps={positiveSteps}
              onChange={setPositiveSteps}
            />

            <AssertionEditor assertions={assertions} error={errors.assertions} onChange={setAssertions} />

            <CompositionDeclarationFields idPrefix={`${fieldId}-composition`} value={declaration} onChange={change('declaration')} toolsHelper="选择预验证需要的运行环境和工具，未选完也可暂存。" />

            {errors.steps ? <Callout tone="danger">{errors.steps}</Callout> : null}
            </fieldset>
          </DrawerBody>
          <DrawerFooter>
            <Button type="button" variant="outline" onClick={() => void close()}>
              保存并关闭
            </Button>
            <Button type="submit" variant="primary" loading={draft.status === 'saving'} disabled={draft.status === 'conflict'}>
              保存草稿
            </Button>
            {draft.dirty ? <Button type="button" variant="ghost" onClick={() => { onSaved(); onClose() }}>稍后继续</Button> : null}
          </DrawerFooter>
        </form>
      </DrawerContent>
    </Drawer>
  )
}

interface ChainStepEditorProps {
  title: string
  description: string
  steps: ChainStep[]
  onChange: (steps: ChainStep[]) => void
}

// 部署与交易参数由运行时适配器定义，保留 JSON 输入。
function ChainStepEditor({ title, description, steps, onChange }: ChainStepEditorProps) {
  const fieldId = useId()

  return (
    <div className="flex flex-col gap-3 rounded-md bg-surface-hover p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-base text-ink">{title}</p>
          <p className="text-sm text-ink-sub">{description}</p>
        </div>
        <Button
          variant="outline"
          size="sm"
          leftIcon={Plus}
          onClick={() => onChange([...steps, { runtimeInstance: '', op: VULN_CHAIN_OPS[0], payload: '{}', target: '' }])}
        >
          添加步骤
        </Button>
      </div>

      {steps.length === 0 ? (
        <p className="text-sm text-ink-sub">没有步骤。这一段可以留空。</p>
      ) : (
        <div className="flex flex-col divide-y divide-line">
          {steps.map((step, index) => (
            <div key={index} className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0">
              <div className="flex flex-wrap items-end gap-3">
                <Badge tone="neutral">第 {index + 1} 步</Badge>
                <FormField label="操作" htmlFor={`${fieldId}-op-${index}`} className="mb-0 flex-1">
                  <Select
                    id={`${fieldId}-op-${index}`}
                    options={VULN_CHAIN_OPS.map((op) => ({ value: op, label: vulnChainOpLabel(op) }))}
                    value={step.op}
                    onValueChange={(value) =>
                      onChange(steps.map((item, i) => (i === index ? { ...item, op: value as VulnChainOp } : item)))
                    }
                  />
                </FormField>
                <IconButton
                  variant="ghost"
                  size="sm"
                  icon={Trash2}
                  aria-label={`删除第 ${index + 1} 步`}
                  onClick={() => onChange(steps.filter((_, i) => i !== index))}
                />
              </div>
              <FormField label="目标链" htmlFor={`${fieldId}-runtime-${index}`} className="mb-0" helper="填写编排中的链实例名称；留空使用工作区所属链">
                <Input id={`${fieldId}-runtime-${index}`} value={step.runtimeInstance} onChange={(event) => onChange(steps.map((item, i) => i === index ? { ...item, runtimeInstance: event.target.value } : item))} />
              </FormField>
              {step.op === 'query' ? (
                <FormField label="查询目标" htmlFor={`${fieldId}-target-${index}`} required className="mb-0" helper="填写要查询的合约、账户或链上对象" error={step.target.trim() === '' ? '请填写查询目标' : null}>
                  <Input id={`${fieldId}-target-${index}`} value={step.target} onChange={(event) => onChange(steps.map((item, i) => i === index ? { ...item, target: event.target.value } : item))} />
                </FormField>
              ) : step.op === 'reset' ? (
                <p className="text-xs text-ink-sub">这个操作不需要参数。</p>
              ) : (
                <FormField
                  label="参数"
                  htmlFor={`${fieldId}-payload-${index}`}
                  className="mb-0"
                  helper="这一步需要传入的参数,格式由所选运行环境决定"
                  error={isValidStep(step) ? undefined : '参数格式不正确,请按示例填写'}
                >
                  <Textarea
                    id={`${fieldId}-payload-${index}`}
                    value={step.payload}
                    rows={3}
                    className="font-mono text-xs"
                    invalid={!isValidStep(step)}
                    onChange={(event) =>
                      onChange(
                        steps.map((item, i) => (i === index ? { ...item, payload: event.target.value } : item)),
                      )
                    }
                  />
                </FormField>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

interface AssertionEditorProps {
  assertions: AssertionDraft[]
  error?: string | null
  onChange: (assertions: AssertionDraft[]) => void
}

function AssertionEditor({ assertions, error, onChange }: AssertionEditorProps) {
  const fieldId = useId()

  return (
    <div className="flex flex-col gap-3 rounded-md bg-surface-hover p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-base text-ink">判定断言</p>
          <p className="text-sm text-ink-sub">
            至少一条。正向验证要求全部成立,反向验证要求全部不成立 —— 两者都满足才算这道题可判。
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          leftIcon={Plus}
          onClick={() => onChange([...assertions, emptyAssertion()])}
        >
          添加断言
        </Button>
      </div>

      {assertions.length === 0 ? (
        <Empty
          icon={Plus}
          title="还没有断言"
          description="没有断言无法判断漏洞是否被利用,预验证会直接失败。"
        />
      ) : (
        <div className="flex flex-col divide-y divide-line">
          {assertions.map((assertion, index) => (
            <div key={index} className="flex flex-col gap-3 py-3 first:pt-0 last:pb-0">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <Badge tone="neutral">断言 {index + 1}</Badge>
                <IconButton
                  variant="ghost"
                  size="sm"
                  icon={Trash2}
                  aria-label={`删除断言 ${index + 1}`}
                  onClick={() => onChange(assertions.filter((_, i) => i !== index))}
                />
              </div>

              <FormField label="目标链" htmlFor={`${fieldId}-runtime-${index}`} className="mb-0" helper="填写编排中的链实例名称；留空使用工作区所属链">
                <Input id={`${fieldId}-runtime-${index}`} value={assertion.runtimeInstance} onChange={(event) => patch(onChange, assertions, index, { runtimeInstance: event.target.value })} />
              </FormField>

              <div className="grid gap-3 sm:grid-cols-2">
                <FormField
                  label="断言名称"
                  htmlFor={`${fieldId}-label-${index}`}
                  className="mb-0"
                  helper="验证报告里显示这个名字"
                >
                  <Input
                    id={`${fieldId}-label-${index}`}
                    value={assertion.label}
                    placeholder="资金被转出"
                    onChange={(event) => patch(onChange, assertions, index, { label: event.target.value })}
                  />
                </FormField>
                <FormField
                  label="查询目标"
                  htmlFor={`${fieldId}-target-${index}`}
                  required
                  className="mb-0"
                  helper="要查询的链上对象,如合约地址或账户"
                >
                  <Input
                    id={`${fieldId}-target-${index}`}
                    value={assertion.target}
                    invalid={assertion.target.trim() === ''}
                    onChange={(event) => patch(onChange, assertions, index, { target: event.target.value })}
                  />
                </FormField>
              </div>

              <div className="grid gap-3 sm:grid-cols-3">
                <FormField
                  label="查询字段"
                  htmlFor={`${fieldId}-field-${index}`}
                  className="mb-0"
                  helper="留空则用查询目标同名字段"
                >
                  <Input
                    id={`${fieldId}-field-${index}`}
                    value={assertion.field}
                    placeholder="例如 balanceOf"
                    onChange={(event) => patch(onChange, assertions, index, { field: event.target.value })}
                  />
                </FormField>
                <FormField label="判定方式" htmlFor={`${fieldId}-op-${index}`} className="mb-0">
                  <Select
                    id={`${fieldId}-op-${index}`}
                    options={VULN_ASSERT_OPS.map((op) => ({ value: op, label: vulnAssertOpLabel(op) }))}
                    value={assertion.op}
                    onValueChange={(value) =>
                      patch(onChange, assertions, index, { op: value as VulnAssertOp })
                    }
                  />
                </FormField>
                <FormField
                  label="期望值"
                  htmlFor={`${fieldId}-value-${index}`}
                  className="mb-0"
                  helper={assertion.op === 'exists' ? '这种判定不需要期望值' : '按 JSON 填写，文本加双引号；也可填数字、true、null、数组或对象'}
                  error={assertionValueError(assertion)}
                >
                  <Textarea
                    id={`${fieldId}-value-${index}`}
                    value={assertion.value}
                    rows={2}
                    disabled={assertion.op === 'exists'}
                    onChange={(event) => patch(onChange, assertions, index, { value: event.target.value })}
                  />
                </FormField>
              </div>

              <div className="grid gap-3 sm:grid-cols-2">
                <FormField
                  label="期望说明"
                  htmlFor={`${fieldId}-expected-${index}`}
                  className="mb-0"
                  helper="学生答题时看到的期望描述,不暴露具体数值"
                >
                  <Input
                    id={`${fieldId}-expected-${index}`}
                    value={assertion.expectedLabel}
                    placeholder="合约余额被清空"
                    onChange={(event) =>
                      patch(onChange, assertions, index, { expectedLabel: event.target.value })
                    }
                  />
                </FormField>
                <FormField
                  label="失败提示"
                  htmlFor={`${fieldId}-hint-${index}`}
                  className="mb-0"
                  helper="断言不成立时给学生的提示方向"
                >
                  <Input
                    id={`${fieldId}-hint-${index}`}
                    value={assertion.hint}
                    onChange={(event) => patch(onChange, assertions, index, { hint: event.target.value })}
                  />
                </FormField>
              </div>
            </div>
          ))}
        </div>
      )}

      {error ? <Callout tone="danger">{error}</Callout> : null}
    </div>
  )
}

function emptyAssertion(): AssertionDraft {
  return {
    runtimeInstance: '',
    label: '',
    target: '',
    field: '',
    op: VULN_ASSERT_OPS[0],
    value: '',
    expectedLabel: '',
    hint: '',
  }
}

function patch(
  onChange: (assertions: AssertionDraft[]) => void,
  assertions: AssertionDraft[],
  index: number,
  next: Partial<AssertionDraft>,
): void {
  onChange(assertions.map((item, i) => (i === index ? { ...item, ...next } : item)))
}

function isValidStep(step: ChainStep): boolean {
  if (step.op === 'query') return step.target.trim() !== ''
  if (step.op === 'reset') return true
  try {
    const parsed = parseVulnJSONValue(step.payload)
    return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)
  } catch {
    return false
  }
}

function assertionValueError(assertion: AssertionDraft): string | null {
  if (assertion.op === 'exists') return null
  try {
    parseVulnJSONValue(assertion.value)
    return null
  } catch {
    return '请填写有效的期望值，文本需加双引号，数字不能超出范围'
  }
}
