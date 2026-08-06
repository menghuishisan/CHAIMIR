// 仿真工作台的操作、结论与执行追踪面板。
//
// 两种执行位置共用它:平台内置场景在浏览器 Worker 内运行,本校自建场景在服务端隔离容器内运行,
// 但面板拿到的都是同形的纯数据快照(见 docs/04-仿真可视化引擎/02-架构设计.md §8),
// 故渲染只有一条路径 —— 页面不为「跑在哪」写第二套操作面板与检查器。
//
// 交互参数控件一律用设计系统组件的深色语境变体(Input underline / Select onDark / Checkbox onDark),
// 不在页面里手拼深色输入框:那会让沉浸态的控件质感、焦点环与无障碍语义各页一套(规范 FE-1、§7.1)。

import { useState } from 'react'
import { Badge, Button, Checkbox, CodeTracePanel, Input, Select } from '@chaimir/ui'
import type {
  CheckpointDescriptor,
  CheckpointResult,
  CodeTraceDef,
  InteractionDescriptor,
  JsonObject,
  JsonValue,
  TraceInfo,
} from '@chaimir/sim-sdk'

/** 交互标签的用户向说明:扰动与攻击类操作会破坏系统,提前说清。 */
const TAG_LABELS: Record<'normal' | 'perturb' | 'attack', string> = {
  normal: '常规操作',
  perturb: '扰动',
  attack: '攻击',
}

export interface InteractionPanelProps {
  interactions: InteractionDescriptor[]
  availability: Record<string, boolean>
  selectedElementId?: string
  onInteract: (interaction: InteractionDescriptor, payload: JsonObject, target?: string) => void
}

/**
 * InteractionPanel 渲染场景声明的可用操作。
 * 能不能点由运行时算出的可用性决定 —— 场景自己知道当前状态允许什么,页面不复制一份判断逻辑。
 * 选元素类操作在舞台上直接点,不在这里重复出现按钮。
 */
export function InteractionPanel({
  interactions,
  availability,
  selectedElementId,
  onInteract,
}: InteractionPanelProps) {
  const actionable = interactions.filter((item) => item.kind !== 'select-element')
  if (actionable.length === 0) return null

  return (
    <section className="flex flex-col gap-2 border-b border-dark-line p-4">
      <h2 className="text-sm font-medium text-on-dark">可用操作</h2>
      <ul className="flex flex-col gap-2">
        {actionable.map((interaction) => (
          <li key={interaction.id}>
            <InteractionItem
              interaction={interaction}
              enabled={availability[interaction.id] !== false}
              selectedElementId={selectedElementId}
              onInteract={onInteract}
            />
          </li>
        ))}
      </ul>
    </section>
  )
}

interface InteractionItemProps {
  interaction: InteractionDescriptor
  enabled: boolean
  selectedElementId?: string
  onInteract: (interaction: InteractionDescriptor, payload: JsonObject, target?: string) => void
}

/**
 * InteractionItem 渲染一个操作及其参数。
 * 参数按场景声明的字段类型渲染显式控件,默认值取声明里的 default ——
 * 这些字段是场景作者定义的教学变量,不是自由输入。
 */
function InteractionItem({
  interaction,
  enabled,
  selectedElementId,
  onInteract,
}: InteractionItemProps) {
  const fields = interaction.params ?? []
  const [values, setValues] = useState<JsonObject>(() =>
    Object.fromEntries(fields.map((field) => [field.name, field.default])),
  )

  const needsElement = interaction.target === 'element'
  const blocked = !enabled || (needsElement && !selectedElementId)

  return (
    <div className="flex flex-col gap-2 rounded-md border border-dark-line bg-dark-surface p-2">
      <div className="flex items-start justify-between gap-2">
        <span className="min-w-0 text-sm text-on-dark">{interaction.label}</span>
        {interaction.labelTag && interaction.labelTag !== 'normal' ? (
          <Badge tone={interaction.labelTag === 'attack' ? 'cinnabar' : 'warning'}>
            {TAG_LABELS[interaction.labelTag]}
          </Badge>
        ) : null}
      </div>
      {interaction.description ? (
        <p className="text-xs text-on-dark-sub">{interaction.description}</p>
      ) : null}

      {fields.map((field) => (
        <label key={field.name} className="flex flex-col gap-1">
          <span className="text-xs text-on-dark-sub">{field.label}</span>
          {field.type === 'boolean' ? (
            <Checkbox
              onDark
              checked={values[field.name] === true}
              onCheckedChange={(checked) =>
                setValues((current) => ({ ...current, [field.name]: checked === true }))
              }
            />
          ) : field.type === 'select' ? (
            <Select
              onDark
              size="sm"
              options={(field.options ?? []).map((option) => ({
                value: String(option.value),
                label: option.label,
              }))}
              value={String(values[field.name] ?? '')}
              onValueChange={(raw) =>
                setValues((current) => ({ ...current, [field.name]: optionValue(field.options, raw) }))
              }
            />
          ) : (
            <Input
              variant="underline"
              type={field.type === 'string' ? 'text' : 'number'}
              value={String(values[field.name] ?? '')}
              min={field.min}
              max={field.max}
              step={field.step}
              className="h-8 font-mono text-sm"
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  [field.name]:
                    field.type === 'string' ? event.target.value : Number(event.target.value),
                }))
              }
            />
          )}
        </label>
      ))}

      <div>
        <Button
          variant="seal"
          size="sm"
          disabled={blocked}
          onClick={() => onInteract(interaction, values, needsElement ? selectedElementId : undefined)}
        >
          执行
        </Button>
      </div>
      {needsElement && !selectedElementId ? (
        <p className="text-xs text-on-dark-faint">先在舞台上点一个元素,再执行这个操作。</p>
      ) : null}
    </div>
  )
}

export interface CheckpointPanelProps {
  checkpoints: CheckpointDescriptor[]
  results: Record<string, CheckpointResult>
}

/**
 * CheckpointPanel 展示场景自带的教学检查点结论。
 * 这些是场景作者写在包里的判定(达成/未达成 + 解释),不是平台判分 ——
 * 实验里的判分走检查点判分接口,两者不混。
 */
export function CheckpointPanel({ checkpoints, results }: CheckpointPanelProps) {
  if (checkpoints.length === 0) return null

  return (
    <section className="flex flex-col gap-2 border-b border-dark-line p-4">
      <h2 className="text-sm font-medium text-on-dark">观察结论</h2>
      <ul className="flex flex-col gap-2">
        {checkpoints.map((checkpoint) => {
          const result = results[checkpoint.id]
          return (
            <li
              key={checkpoint.id}
              className="flex flex-col gap-1 rounded-md border border-dark-line bg-dark-surface p-2"
            >
              <div className="flex items-start justify-between gap-2">
                <span className="min-w-0 text-sm text-on-dark">{checkpoint.label}</span>
                <Badge tone={result?.achieved ? 'success' : 'neutral'}>
                  {result?.achieved ? '已观察到' : '尚未出现'}
                </Badge>
              </div>
              {result?.explanation ? (
                <p className="text-xs text-on-dark-sub">{result.explanation}</p>
              ) : null}
            </li>
          )
        })}
      </ul>
    </section>
  )
}

export interface CodeTraceSectionProps {
  codeTrace?: CodeTraceDef
  trace?: TraceInfo
}

/**
 * CodeTraceSection 把当前状态对应到教学代码行,建立「看到的现象 ↔ 代码逻辑」的因果锚点。
 * 场景没有声明代码追踪时整段不出现,不留空标题。
 */
export function CodeTraceSection({ codeTrace, trace }: CodeTraceSectionProps) {
  if (!codeTrace) return null

  return (
    <section className="border-b border-dark-line p-4 last:border-b-0">
      <h2 className="mb-2 text-sm font-medium text-on-dark">执行追踪</h2>
      <CodeTracePanel codeTrace={codeTrace} trace={trace} />
    </section>
  )
}

/** optionValue 从声明的选项里取回原始值:select 的 DOM 值是字符串,不能直接当业务值用。 */
function optionValue(
  options: Array<{ label: string; value: JsonValue }> | undefined,
  raw: string,
): JsonValue {
  const matched = (options ?? []).find((option) => String(option.value) === raw)
  return matched ? matched.value : raw
}
