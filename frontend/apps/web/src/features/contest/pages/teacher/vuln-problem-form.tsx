// 漏洞题录入表单(漏洞题工坊页内)。
//
// draft_body 的键不是自由的:后端预验证按 init_steps(初始化)、positive_steps(攻击 PoC)、
// assertions(判定断言)三个数组执行链上操作,故表单按这三段渲染结构化编辑器,
// 题面部分(说明 / 合约源码 / 分叉区块)随固化原样进 M5 题目正文。
//
// 断言至少要有一条:后端 checkVulnAssertions 在断言为空时直接判失败
// (没有断言就无法判断漏洞是否真的被利用),故这里也要求至少一条。

import { useCallback, useId, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import {
  VulnLevel,
  VulnRuntimeMode,
  type VulnProblemImportRequest,
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
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import {
  VULN_ASSERTION_FIELDS,
  VULN_ASSERT_OPS,
  VULN_CHAIN_OPS,
  VULN_CHAIN_STEP_FIELDS,
  VULN_DRAFT_BODY_FIELDS,
  VULN_LEVELS,
  VULN_RUNTIME_MODES,
  vulnAssertOpLabel,
  vulnChainOpLabel,
  vulnLevelLabel,
  vulnRuntimeModeLabel,
  type VulnAssertOp,
  type VulnChainOp,
} from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** ChainStep 是一条链上步骤的编辑态。 */
interface ChainStep {
  op: VulnChainOp
  payload: string
}

/** AssertionDraft 是一条断言的编辑态。 */
interface AssertionDraft {
  label: string
  target: string
  field: string
  op: VulnAssertOp
  value: string
  expectedLabel: string
  hint: string
}

export interface VulnProblemFormModalProps {
  sources: VulnSource[]
  onClose: () => void
  onSaved: () => void
}

/**
 * VulnProblemFormModal 手工录入一条漏洞题草稿。
 */
export function VulnProblemFormModal({ sources, onClose, onSaved }: VulnProblemFormModalProps) {
  const fieldId = useId()

  const [title, setTitle] = useState('')
  const [sourceId, setSourceId] = useState<string>('')
  const [externalRef, setExternalRef] = useState('')
  const [level, setLevel] = useState(String(VulnLevel.A))
  const [runtimeMode, setRuntimeMode] = useState(String(VulnRuntimeMode.ISOLATED))
  const [description, setDescription] = useState('')
  const [contractSource, setContractSource] = useState('')
  const [forkBlock, setForkBlock] = useState('')
  const [initSteps, setInitSteps] = useState<ChainStep[]>([])
  const [positiveSteps, setPositiveSteps] = useState<ChainStep[]>([])
  const [assertions, setAssertions] = useState<AssertionDraft[]>([emptyAssertion()])

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const isForked = Number(runtimeMode) === VulnRuntimeMode.FORKED

  /** validate 校验必填项与 JSON 参数可解析性。 */
  const validate = useCallback((): boolean => {
    const next: Record<string, string | null> = {
      title: title.trim() === '' ? '请输入漏洞题标题' : title.trim().length > 255 ? '标题不能超过 255 个字' : null,
      description: description.trim() === '' ? '请写下漏洞说明,学生要知道这道题在考什么' : null,
      forkBlock:
        isForked && (forkBlock.trim() === '' || !Number.isFinite(Number(forkBlock)))
          ? '主网分叉复现需要锁定一个区块高度'
          : null,
      assertions: assertions.some((item) => item.target.trim() === '')
        ? '每条断言都要填查询目标'
        : null,
      steps: [...initSteps, ...positiveSteps].some((step) => !isValidPayload(step))
        ? '有步骤的参数格式不正确,请检查内容后重填'
        : null,
    }
    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [assertions, description, forkBlock, initSteps, isForked, positiveSteps, title])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return

      // draft_body 按后端读取的键组装:三段链上流程 + 题面部分
      const payload: VulnProblemImportRequest = {
        source_id: sourceId || undefined,
        external_ref: externalRef.trim() || undefined,
        title: title.trim(),
        level: Number(level) as VulnLevel,
        runtime_mode: Number(runtimeMode) as VulnRuntimeMode,
        draft_body: {
          [VULN_DRAFT_BODY_FIELDS.description]: description.trim(),
          ...(contractSource.trim() !== ''
            ? { [VULN_DRAFT_BODY_FIELDS.contractSource]: contractSource }
            : {}),
          ...(isForked ? { [VULN_DRAFT_BODY_FIELDS.forkBlock]: Number(forkBlock) } : {}),
          [VULN_DRAFT_BODY_FIELDS.initSteps]: initSteps.map(stepToPayload),
          [VULN_DRAFT_BODY_FIELDS.positiveSteps]: positiveSteps.map(stepToPayload),
          [VULN_DRAFT_BODY_FIELDS.assertions]: assertions.map(assertionToPayload),
        },
      }

      setFormError(undefined)
      setSubmitting(true)
      try {
        await api.contest.importVulnProblem(payload)
        toast.success('漏洞题草稿已录入,接下来做预验证')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '录入没有成功,请检查填写内容后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [
      assertions,
      contractSource,
      description,
      externalRef,
      forkBlock,
      initSteps,
      isForked,
      level,
      onSaved,
      positiveSteps,
      runtimeMode,
      sourceId,
      title,
      validate,
    ],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>录入漏洞题</ModalTitle>
          <ModalDescription>
            填好初始化、攻击步骤与判定断言后做预验证:正向要打通、反向要不误判,双向通过才能固化进题库。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="标题" htmlFor={`${fieldId}-title`} required error={errors.title}>
              <Input
                id={`${fieldId}-title`}
                value={title}
                invalid={Boolean(errors.title)}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="归属来源" htmlFor={`${fieldId}-source`} helper="手工录入可以不选">
                <Select
                  id={`${fieldId}-source`}
                  options={[
                    { value: '', label: '不归属任何来源' },
                    ...sources.map((source) => ({ value: source.id, label: source.name })),
                  ]}
                  value={sourceId}
                  placeholder="不归属任何来源"
                  onValueChange={setSourceId}
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

            {errors.steps ? <Callout tone="danger">{errors.steps}</Callout> : null}
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={submitting}>
              保存草稿
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface ChainStepEditorProps {
  title: string
  description: string
  steps: ChainStep[]
  onChange: (steps: ChainStep[]) => void
}

/**
 * ChainStepEditor 编辑一组链上步骤。
 * 参数是链能力的入参(合约字节码、调用数据等),形状由运行时适配器决定,
 * 故这里保留 JSON 参数输入 —— 但操作类型是封闭枚举,从下拉里选。
 */
function ChainStepEditor({ title, description, steps, onChange }: ChainStepEditorProps) {
  const fieldId = useId()

  return (
    <div className="flex flex-col gap-3 rounded-md border border-line bg-surface-sunken p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-base text-ink">{title}</p>
          <p className="text-sm text-ink-sub">{description}</p>
        </div>
        <Button
          variant="outline"
          size="sm"
          leftIcon={Plus}
          onClick={() => onChange([...steps, { op: VULN_CHAIN_OPS[0], payload: '{}' }])}
        >
          添加步骤
        </Button>
      </div>

      {steps.length === 0 ? (
        <p className="text-sm text-ink-sub">没有步骤。这一段可以留空。</p>
      ) : (
        <div className="flex flex-col gap-3">
          {steps.map((step, index) => (
            <div key={index} className="flex flex-col gap-2 rounded-md border border-line bg-surface p-3">
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
              {step.op === 'reset' || step.op === 'query' ? (
                <p className="text-xs text-ink-sub">这个操作不需要参数。</p>
              ) : (
                <FormField
                  label="参数"
                  htmlFor={`${fieldId}-payload-${index}`}
                  className="mb-0"
                  helper="这一步需要传入的参数,格式由所选运行环境决定"
                  error={isValidPayload(step) ? undefined : '参数格式不正确,请按示例填写'}
                >
                  <Textarea
                    id={`${fieldId}-payload-${index}`}
                    value={step.payload}
                    rows={3}
                    className="font-mono text-xs"
                    invalid={!isValidPayload(step)}
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

/**
 * AssertionEditor 编辑判定断言。
 * 断言决定"漏洞是否真的被利用":正向验证要全部通过,反向验证要全部不通过。
 */
function AssertionEditor({ assertions, error, onChange }: AssertionEditorProps) {
  const fieldId = useId()

  return (
    <div className="flex flex-col gap-3 rounded-md border border-line bg-surface-sunken p-4">
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
        <div className="flex flex-col gap-3">
          {assertions.map((assertion, index) => (
            <div key={index} className="flex flex-col gap-3 rounded-md border border-line bg-surface p-3">
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
                  helper={assertion.op === 'exists' ? '这种判定不需要期望值' : '与查询结果比对的值'}
                >
                  <Input
                    id={`${fieldId}-value-${index}`}
                    value={assertion.value}
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

/** emptyAssertion 给出一条空断言的初始编辑态。 */
function emptyAssertion(): AssertionDraft {
  return {
    label: '',
    target: '',
    field: '',
    op: VULN_ASSERT_OPS[0],
    value: '',
    expectedLabel: '',
    hint: '',
  }
}

/** patch 局部更新第 index 条断言。 */
function patch(
  onChange: (assertions: AssertionDraft[]) => void,
  assertions: AssertionDraft[],
  index: number,
  next: Partial<AssertionDraft>,
): void {
  onChange(assertions.map((item, i) => (i === index ? { ...item, ...next } : item)))
}

/** isValidPayload 判断步骤参数是否是合法 JSON 对象;无参数的操作恒为真。 */
function isValidPayload(step: ChainStep): boolean {
  if (step.op === 'reset' || step.op === 'query') return true
  try {
    const parsed: unknown = JSON.parse(step.payload)
    return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)
  } catch {
    return false
  }
}

/** stepToPayload 把步骤编辑态转成后端读取的形状。 */
function stepToPayload(step: ChainStep): Record<string, unknown> {
  if (step.op === 'reset' || step.op === 'query') {
    return { [VULN_CHAIN_STEP_FIELDS.op]: step.op }
  }
  return {
    [VULN_CHAIN_STEP_FIELDS.op]: step.op,
    [VULN_CHAIN_STEP_FIELDS.payload]: JSON.parse(step.payload) as Record<string, unknown>,
  }
}

/** assertionToPayload 把断言编辑态转成后端 chainassert 读取的形状。 */
function assertionToPayload(assertion: AssertionDraft): Record<string, unknown> {
  return {
    [VULN_ASSERTION_FIELDS.label]: assertion.label.trim() || assertion.target.trim(),
    [VULN_ASSERTION_FIELDS.target]: assertion.target.trim(),
    ...(assertion.field.trim() !== '' ? { [VULN_ASSERTION_FIELDS.field]: assertion.field.trim() } : {}),
    [VULN_ASSERTION_FIELDS.op]: assertion.op,
    ...(assertion.op === 'exists' ? {} : { [VULN_ASSERTION_FIELDS.value]: assertion.value }),
    ...(assertion.expectedLabel.trim() !== ''
      ? { [VULN_ASSERTION_FIELDS.expectedLabel]: assertion.expectedLabel.trim() }
      : {}),
    ...(assertion.hint.trim() !== '' ? { [VULN_ASSERTION_FIELDS.hint]: assertion.hint.trim() } : {}),
  }
}
