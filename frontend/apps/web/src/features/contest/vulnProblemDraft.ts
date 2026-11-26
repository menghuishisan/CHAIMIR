// 漏洞题编辑态与服务端正文换算；保留未在表单中编辑的扩展字段。
import { COMPOSITION_SELECTION, type VulnProblem, type VulnProblemRequest, type VulnLevel, type VulnRuntimeMode } from '@chaimir/api-client'
import { parseVulnJSONValue } from './vulnerabilityContract'
import { type VulnAssertOp, type VulnChainOp } from './options'
import { declarationFromSpec, scenarioNeutralSpecFromDeclaration, type CompositionDeclaration } from '../sandbox/composition'

export interface ChainStep {
  runtimeInstance: string
  op: VulnChainOp
  payload: string
  target: string
  extra?: Record<string, unknown>
}

export interface AssertionDraft {
  runtimeInstance: string
  label: string
  target: string
  field: string
  op: VulnAssertOp
  value: string
  expectedLabel: string
  hint: string
  extra?: Record<string, unknown>
}

export interface VulnProblemFields {
  title: string
  judgerCode: string
  maxScore: string
  sourceId: string
  externalRef: string
  level: string
  runtimeMode: string
  description: string
  contractSource: string
  forkBlock: string
  initSteps: ChainStep[]
  positiveSteps: ChainStep[]
  assertions: AssertionDraft[]
  declaration: CompositionDeclaration
}

// 已存字段形状异常时停止编辑，不能把读不懂的内容当成空值覆盖。
function object(value: unknown): Record<string, unknown> {
  if (value === undefined) return {}
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid draft object')
  return value as Record<string, unknown>
}

function text(value: unknown): string {
  if (value === undefined) return ''
  if (typeof value !== 'string') throw new Error('invalid draft text')
  return value
}

function numberText(value: unknown): string {
  if (value === undefined) return ''
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new Error('invalid draft number')
  return String(value)
}

function items(value: unknown): Record<string, unknown>[] {
  if (value === undefined) return []
  if (!Array.isArray(value)) throw new Error('invalid draft list')
  return value.map((item) => { if (item === undefined) throw new Error('invalid draft item'); return object(item) })
}

/** 恢复可编辑字段；显式 null 与缺省值不混淆，组合允许尚未选完。 */
export function vulnProblemFields(problem: VulnProblem): VulnProblemFields {
  const body = problem.draft_body
  if (['init_steps', 'init_code_ref', 'init_script_ref'].some((key) => Object.prototype.hasOwnProperty.call(body, key))) throw new Error('duplicate draft initialization')
  const judge = object(body.judge_config)
  const composition = body.composition === undefined ? undefined : object(body.composition)
  const components = (value: unknown) => items(value).map((item) => {
    if (!text(item.code).trim() || (item.selection !== COMPOSITION_SELECTION.EXPLICIT && item.selection !== COMPOSITION_SELECTION.AUTO)) throw new Error('invalid component reference')
    if (item.params !== undefined) object(item.params)
    return item
  })
  const savedComposition = composition === undefined ? undefined : {
    ...composition,
    workspace_runtime_instance: text(composition.workspace_runtime_instance),
    runtimes: items(composition.runtimes).map((runtime) => {
      if (runtime.params !== undefined) object(runtime.params)
      return { ...runtime,
        instance_code: text(runtime.instance_code),
        runtime_code: text(runtime.runtime_code),
        image_version: text(runtime.image_version),
      }
    }),
    tools: components(composition.tools), infra: components(composition.infra),
    links: items(composition.links).map((link) => {
      if (link.required_at_start !== undefined && typeof link.required_at_start !== 'boolean') throw new Error('invalid link startup flag')
      return { ...link,
        source_component: text(link.source_component), source_endpoint: text(link.source_endpoint),
        target_component: text(link.target_component), target_endpoint: text(link.target_endpoint),
        protocol: text(link.protocol), access_scope: text(link.access_scope), config_binding: text(link.config_binding),
        required_at_start: link.required_at_start ?? false,
      }
    }),
  }
  const steps = (value: unknown): ChainStep[] => items(value).map(({ runtime_instance, op, payload, target, ...extra }) => ({
    runtimeInstance: text(runtime_instance), op: text(op) as VulnChainOp,
    payload: payload === undefined ? '' : JSON.stringify(payload, null, 2), target: text(target), extra,
  }))
  return {
    title: problem.title, judgerCode: text(judge.judger_code), maxScore: numberText(judge.max_score),
    sourceId: problem.source_id ?? '', externalRef: problem.external_ref ?? '',
    level: String(problem.level), runtimeMode: String(problem.runtime_mode),
    description: text(body.description), contractSource: text(body.contract_source), forkBlock: numberText(body.fork_block),
    initSteps: steps(composition?.init_chain_steps), positiveSteps: steps(body.positive_steps),
    assertions: items(body.assertions).map(({ runtime_instance, label, target, field, op, value, expected_label, hint, ...extra }) => ({
      runtimeInstance: text(runtime_instance), label: text(label), target: text(target), field: text(field), op: text(op) as VulnAssertOp,
      value: value === undefined ? '' : JSON.stringify(value, null, 2), expectedLabel: text(expected_label), hint: text(hint), extra,
    })),
    declaration: declarationFromSpec(savedComposition as Parameters<typeof declarationFromSpec>[0]),
  }
}

/** 将步骤转回唯一执行协议，未填完参数可暂存，错误 JSON 留在本机待修正。 */
export function stepToPayload(step: ChainStep): Record<string, unknown> {
  const base = { ...step.extra, op: step.op, ...(step.runtimeInstance.trim() ? { runtime_instance: step.runtimeInstance.trim() } : {}) }
  if (step.op === 'query') return { ...base, target: step.target.trim() }
  if (step.op === 'reset') return base
  return { ...base, ...(step.payload.trim() ? { payload: parseVulnJSONValue(step.payload) } : {}) }
}

/** 断言保持 JSON 值类型；尚未填写的值不伪装成 null。 */
export function assertionToPayload(item: AssertionDraft): Record<string, unknown> {
  return {
    ...item.extra,
    ...(item.runtimeInstance.trim() ? { runtime_instance: item.runtimeInstance.trim() } : {}),
    label: item.label.trim() || item.target.trim(), target: item.target.trim(), ...(item.field.trim() ? { field: item.field.trim() } : {}), op: item.op,
    ...(item.op !== 'exists' && item.value.trim() ? { value: parseVulnJSONValue(item.value) } : {}),
    ...(item.expectedLabel.trim() ? { expected_label: item.expectedLabel.trim() } : {}), ...(item.hint.trim() ? { hint: item.hint.trim() } : {}),
  }
}

/** 保存半成品时只检查存储边界和输入格式，不提前要求可执行。 */
export function vulnProblemRequest(fields: VulnProblemFields, base: VulnProblem): VulnProblemRequest {
  if (!fields.title.trim() || [...fields.title.trim()].length > 255 || [...fields.externalRef.trim()].length > 128) throw new Error('invalid draft title or reference')
  // 更名中间态不能清除已保存的实例标识，否则重开后无法恢复引用关系。
  if (fields.declaration.runtimes.some((runtime) =>
    (runtime.originalInstanceCode && !runtime.instanceCode.trim()) ||
    (runtime.pendingInstanceCode && runtime.pendingInstanceCode !== runtime.instanceCode.trim())
  )) throw new Error('unfinished runtime rename')
  const names = fields.declaration.runtimes.map((runtime) => runtime.instanceCode.trim()).filter(Boolean)
  if (new Set(names).size !== names.length) throw new Error('duplicate runtime names')
  const numeric = (value: string): number | undefined => {
    if (!value.trim()) return undefined
    const result = Number(value)
    if (!Number.isFinite(result)) throw new Error('invalid draft number')
    return result
  }
  const judge = object(base.draft_body.judge_config)
  return {
    title: fields.title, source_id: fields.sourceId || undefined, external_ref: fields.externalRef || undefined,
    level: Number(fields.level) as VulnLevel, runtime_mode: Number(fields.runtimeMode) as VulnRuntimeMode,
    draft_body: {
      ...base.draft_body, description: fields.description, contract_source: fields.contractSource,
      fork_block: Number(fields.runtimeMode) === 2 ? numeric(fields.forkBlock) : undefined,
      positive_steps: fields.positiveSteps.map(stepToPayload),
      assertions: fields.assertions.map(assertionToPayload),
      judge_config: { ...judge, judger_code: fields.judgerCode, max_score: numeric(fields.maxScore) },
      composition: {
        ...scenarioNeutralSpecFromDeclaration(`vuln-${base.id}`, fields.declaration, base.draft_body.composition as Parameters<typeof scenarioNeutralSpecFromDeclaration>[2]),
        init_chain_steps: fields.initSteps.map(stepToPayload),
      },
    },
  }
}
