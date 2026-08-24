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
import type { CompositionDeclaration } from '../composition'

export interface CompositionDeclarationFieldsProps {
  /** idPrefix 保证同页多个实例的表单控件 id 不撞车 */
  idPrefix: string
  value: CompositionDeclaration
  onChange: (next: CompositionDeclaration) => void
  /** toolsHelper 让各场景说明工具在这里意味着什么(学生打开 / 验证时使用 / 判题执行) */
  toolsHelper: string
  /** 已由编译器按依赖补齐的基础设施编码,只读提示,不参与勾选 */
  derivedInfraCodes?: readonly string[]
}

/**
 * CompositionDeclarationFields 渲染组合声明的四个字段。
 * 换运行时会清空镜像与组件:上一个运行时的版本号与组件对新运行时没有意义。
 */
export function CompositionDeclarationFields({
  idPrefix,
  value,
  onChange,
  toolsHelper,
  derivedInfraCodes = [],
}: CompositionDeclarationFieldsProps) {
  const catalog = useOrchestrationCatalog()

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
                        })}
                      />
                    </FormField>
                    <FormField label="运行时" htmlFor={`${idPrefix}-runtime-${index}-code`} required>
                      <Select
                        id={`${idPrefix}-runtime-${index}-code`}
                        options={catalog.runtimeOptions}
                        value={runtime.runtimeCode}
                        placeholder="选择运行时"
                        onValueChange={(runtimeCode) => onChange({ ...value, runtimes: value.runtimes.map((item, itemIndex) => itemIndex === index ? { ...item, runtimeCode, imageVersion: '' } : item) })}
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
                    <Button type="button" variant="ghost" size="sm" aria-label={`移除运行时实例 ${runtime.instanceCode || index + 1}`} title="移除运行时实例" disabled={value.runtimes.length <= 1} onClick={() => {
                      const runtimes = value.runtimes.filter((_, itemIndex) => itemIndex !== index)
                      onChange({ ...value, runtimes, workspaceRuntimeInstance: runtime.instanceCode === value.workspaceRuntimeInstance ? '' : value.workspaceRuntimeInstance })
                    }}>
                      <Trash2 size={16} aria-hidden="true" />
                    </Button>
                  </div>
                )
              })}
              <Button type="button" variant="outline" size="sm" leftIcon={Plus} onClick={() => onChange({ ...value, runtimes: [...value.runtimes, { instanceCode: `chain-${value.runtimes.length + 1}`, runtimeCode: '', imageVersion: '' }] })}>
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
              emptyHint="平台还没有可用工具,可只使用运行时自身能力。"
              onToggle={(toolCodes) => onChange({ ...value, toolCodes })}
            />
          </FormField>

          <FormField
            label="基础设施组件"
            helper="链浏览器数据库、索引服务这类不由使用者直接打开的支撑组件;不确定时留空,平台会按依赖自动补齐"
          >
            <ComponentChecklist
              items={catalog.infra}
              selected={value.infraCodes}
              emptyHint="平台还没有可声明的基础设施组件。"
              onToggle={(infraCodes) => onChange({ ...value, infraCodes })}
            />
          </FormField>

          {derivedInfraCodes.length > 0 ? (
            <Callout tone="info" title="平台已自动补齐的支撑组件">
              {derivedInfraCodes.join('、')}——
              这些是上次保存时按组件依赖补上的,保存后会重新计算。
            </Callout>
          ) : null}
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
}

/** ComponentChecklist 是工具与基础设施共用的勾选列表,两处口径一致。 */
function ComponentChecklist({ items, selected, emptyHint, onToggle }: ComponentChecklistProps) {
  if (items.length === 0) {
    return <p className="text-sm text-ink-sub">{emptyHint}</p>
  }
  return (
    <div className="flex flex-col gap-2">
      {items.map((item) => (
        <Checkbox
          key={item.code}
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
      ))}
    </div>
  )
}
