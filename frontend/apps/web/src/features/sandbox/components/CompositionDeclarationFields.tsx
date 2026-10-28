// CompositionDeclarationFields 统一编排页里的组合声明表单:命名 runtime 实例、镜像版本、学生工具与基础设施。
//
// 四个编排入口(实验环境、题库实操题、漏洞题预验证、判题私有环境)共用本组件,
// 不各写一遍取数、下拉与勾选逻辑。
//
// 目录只回真正可调度的项(运行时已通过平台自检、镜像已预拉取成功且内置创世),
// 状态门禁在服务端做,故这里不按状态二次过滤,也不在前端算运行时与工具的兼容性 ——
// 兼容性由服务端编译器判定并在保存时回报(对齐清单 §6.3)。

import { Plus, Server, Trash2 } from 'lucide-react'
import { Button, Callout, Checkbox, FormField, Input, Select, Skeleton } from '@chaimir/ui'
import type { SandboxCatalogTool } from '@chaimir/api-client'
import { ResourceState } from '../../../components/ResourceState'
import { sandboxToolKindLabel } from '../../../utils/labels/sandbox'
import { useOrchestrationCatalog } from '../useOrchestrationCatalog'
import { compositionSelectionError, compositionSelectionKey, type CompositionDeclaration } from '../composition'
import { CompositionConnectionsFields } from './CompositionConnectionsFields'
import { CompositionParameterFields } from './CompositionParameterFields'

export interface CompositionDeclarationFieldsProps {
  /** idPrefix 保证同页多个实例的表单控件 id 不撞车 */
  idPrefix: string
  value: CompositionDeclaration
  onChange: (next: CompositionDeclaration) => void
  /** toolsHelper 让各场景说明工具在这里意味着什么(学生打开 / 验证时使用 / 判题执行) */
  toolsHelper: string
}

/**
 * CompositionDeclarationFields 渲染选择和连接配置。
 * 换运行时清空旧镜像与参数,组件兼容性和连接由M2重新检查。
 */
export function CompositionDeclarationFields({
  idPrefix,
  value,
  onChange: commit,
  toolsHelper,
}: CompositionDeclarationFieldsProps) {
  const catalog = useOrchestrationCatalog()
  // 任何选择变化立即撤销预览证明;保留仍存在的连接,由M2重算依赖和候选。
  const onChange = (next: CompositionDeclaration) => commit({ ...next, connectionCheck: undefined })
  const selectionError = compositionSelectionError(value)

  return (
    <ResourceState
      resource={catalog.resource}
      emptyIcon={Server}
      emptyTitle="平台还没有可用运行时"
      emptyDescription="运行时要先通过平台自检、镜像预拉取成功才会出现在这里。请联系平台管理员。"
      skeleton={<Skeleton variant="line" lines={2} />}
    >
      {() => (
        <div className="flex flex-col gap-4">
          <FormField label="运行时实例" required helper="每个实例需要唯一别名,跨链连接和链操作都按别名定位。">
            <div className="flex flex-col gap-3">
              {value.runtimes.map((runtime, index) => {
                const imageOptions = catalog.imageOptions(runtime.runtimeCode)
                return (
                  <div key={`${idPrefix}-runtime-${index}`} className="grid gap-3 rounded border border-line p-3 sm:grid-cols-4 sm:items-end">
                    <FormField label="实例别名" htmlFor={`${idPrefix}-runtime-${index}-instance`} required>
                      <Input
                        id={`${idPrefix}-runtime-${index}-instance`}
                        value={runtime.instanceCode}
                        placeholder="source-chain"
                        onChange={(event) => onChange({
                          ...value,
                          runtimes: value.runtimes.map((item, itemIndex) => itemIndex === index ? { ...item, instanceCode: event.target.value } : item),
                          workspaceRuntimeInstance: runtime.instanceCode === value.workspaceRuntimeInstance ? event.target.value : value.workspaceRuntimeInstance,
                          links: value.links.map((link) => ({ ...link,
                            source_component: link.source_component === runtime.instanceCode ? event.target.value : link.source_component,
                            target_component: link.target_component === runtime.instanceCode ? event.target.value : link.target_component,
                          })),
                        })}
                      />
                    </FormField>
                    <FormField label="运行时" htmlFor={`${idPrefix}-runtime-${index}-code`} required>
                      <Select
                        id={`${idPrefix}-runtime-${index}-code`}
                        options={catalog.runtimeOptions}
                        value={runtime.runtimeCode}
                        placeholder="选择运行时"
                        onValueChange={(runtimeCode) => onChange({ ...value, runtimes: value.runtimes.map((item, itemIndex) => itemIndex === index ? { ...item, runtimeCode, imageVersion: '', params: undefined } : item) })}
                      />
                    </FormField>
                    <FormField label="镜像版本" htmlFor={`${idPrefix}-runtime-${index}-image`} required>
                      <Select
                        id={`${idPrefix}-runtime-${index}-image`}
                        options={imageOptions}
                        value={runtime.imageVersion}
                        placeholder={runtime.runtimeCode === '' ? '请先选择运行时' : imageOptions.length > 0 ? '选择版本' : '该运行时暂无可用版本'}
                        disabled={imageOptions.length === 0}
                        onValueChange={(imageVersion) => onChange({ ...value, runtimes: value.runtimes.map((item, itemIndex) => itemIndex === index ? { ...item, imageVersion } : item) })}
                      />
                    </FormField>
                    <Button type="button" variant="ghost" size="sm" aria-label={`移除运行时实例 ${runtime.instanceCode || index + 1}`} title="移除运行时实例" disabled={value.runtimes.length <= 1 || runtime.initAssetBound === true} onClick={() => {
                      const runtimes = value.runtimes.filter((_, itemIndex) => itemIndex !== index)
                      onChange({ ...value, runtimes, workspaceRuntimeInstance: runtime.instanceCode === value.workspaceRuntimeInstance ? '' : value.workspaceRuntimeInstance,
                        links: value.links.filter((link) => link.source_component !== runtime.instanceCode && link.target_component !== runtime.instanceCode) })
                    }}>
                      <Trash2 size={16} aria-hidden="true" />
                    </Button>
                    {runtime.initAssetBound ? <p className="text-xs text-ink-sub sm:col-span-4">初始化材料仍关联此实例,调整材料关联后才能移除;更名会同步保留关联。</p> : null}
                    <div className="sm:col-span-4">
                      <CompositionParameterFields
                        idPrefix={`${idPrefix}-runtime-${index}`}
                        schema={catalog.resource.data?.runtimes.find((item) => item.code === runtime.runtimeCode)?.capabilities.config_schema}
                        value={runtime.params}
                        onChange={(params) => onChange({ ...value, runtimes: value.runtimes.map((item, itemIndex) => itemIndex === index ? { ...item, params } : item) })}
                      />
                    </div>
                  </div>
                )
              })}
              <Button type="button" variant="outline" size="sm" leftIcon={Plus} onClick={() => {
                let number = 1
                while (value.runtimes.some((runtime) => runtime.instanceCode === `chain-${number}`)) number++
                onChange({ ...value, runtimes: [...value.runtimes, { instanceCode: `chain-${number}`, runtimeCode: '', imageVersion: '' }] })
              }}>
                添加运行时实例
              </Button>
            </div>
          </FormField>

          <FormField label="工作区运行时" required helper="文件、终端和初始化由这个实例的工作区能力承载。">
            <Select
              id={`${idPrefix}-workspace-runtime`}
              options={value.runtimes.filter((runtime) => runtime.instanceCode.trim() !== '').map((runtime) => ({ label: runtime.instanceCode, value: runtime.instanceCode }))}
              value={value.workspaceRuntimeInstance}
              placeholder="选择工作区运行时"
              onValueChange={(workspaceRuntimeInstance) => onChange({ ...value, workspaceRuntimeInstance })}
            />
          </FormField>

          <FormField label="工具" helper={toolsHelper}>
            <ComponentChecklist
              items={catalog.tools}
              selected={value.toolCodes}
              params={value.componentParams}
              idPrefix={`${idPrefix}-tool`}
              onParamsChange={(code, params) => onChange({ ...value, componentParams: { ...value.componentParams, [code]: params } })}
              emptyHint="平台还没有可用工具,可只使用运行时自身能力。"
              onToggle={(toolCodes) => onChange({ ...value, toolCodes, links: value.links.filter((link) => !value.toolCodes.some((code) => !toolCodes.includes(code) && (link.source_component === code || link.target_component === code))) })}
            />
          </FormField>

          <FormField
            label="基础设施组件"
            helper="链浏览器数据库、索引服务这类不由使用者直接打开的支撑组件;不确定时留空,平台会按依赖自动补齐"
          >
            <ComponentChecklist
              items={catalog.infra}
              selected={value.infraCodes}
              params={value.componentParams}
              idPrefix={`${idPrefix}-infra`}
              onParamsChange={(code, params) => onChange({ ...value, componentParams: { ...value.componentParams, [code]: params } })}
              emptyHint="平台还没有可声明的基础设施组件。"
              onToggle={(infraCodes) => onChange({ ...value, infraCodes, links: value.links.filter((link) => !value.infraCodes.some((code) => !infraCodes.includes(code) && (link.source_component === code || link.target_component === code))) })}
            />
          </FormField>

          {selectionError === undefined ? (
            <FormField label="组件连接" helper="平台会检查依赖和可用目标;多个目标时请明确选择。">
              <CompositionConnectionsFields key={compositionSelectionKey(value)} idPrefix={idPrefix} value={value} onChange={commit} />
            </FormField>
          ) : <Callout tone="info">{selectionError}</Callout>}
        </div>
      )}
    </ResourceState>
  )
}

interface ComponentChecklistProps {
  items: SandboxCatalogTool[]
  selected: readonly string[]
  emptyHint: string
  onToggle: (next: string[]) => void
  idPrefix: string
  params?: Record<string, Record<string, unknown>>
  onParamsChange: (code: string, params: Record<string, unknown>) => void
}

/** ComponentChecklist 是工具与基础设施共用的勾选列表,两处口径一致。 */
function ComponentChecklist({ items, selected, emptyHint, onToggle, idPrefix, params, onParamsChange }: ComponentChecklistProps) {
  if (items.length === 0) {
    return <p className="text-sm text-ink-sub">{emptyHint}</p>
  }
  return (
    <div className="flex flex-col gap-2">
      {items.map((item) => (
        <div key={item.code} className="flex flex-col gap-3">
          <Checkbox
            checked={selected.includes(item.code)}
            label={`${item.name} · ${sandboxToolKindLabel(item.kind)}`}
            onCheckedChange={(checked) =>
              onToggle(
                checked === true
                  ? [...selected, item.code]
                  : selected.filter((code) => code !== item.code),
              )
            }
          />
          {selected.includes(item.code) ? (
            <CompositionParameterFields idPrefix={`${idPrefix}-${item.code}`} schema={item.capabilities.config_schema} value={params?.[item.code]} onChange={(next) => onParamsChange(item.code, next)} />
          ) : null}
        </div>
      ))}
    </div>
  )
}
