// CompositionDeclarationFields 统一编排页里的组合声明表单:主运行时、镜像版本、学生工具与基础设施。
//
// 四个编排入口(实验环境、题库实操题、漏洞题预验证、判题私有环境)共用本组件,
// 不各写一遍取数、下拉与勾选逻辑。
//
// 目录只回真正可调度的项(运行时已通过平台自检、镜像已预拉取成功且内置创世),
// 状态门禁在服务端做,故这里不按状态二次过滤,也不在前端算运行时与工具的兼容性 ——
// 兼容性由服务端编译器判定并在保存时回报(对齐清单 §6.3)。

import { Server } from 'lucide-react'
import { Callout, Checkbox, FormField, Select, Skeleton } from '@chaimir/ui'
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
  const imageOptions = catalog.imageOptions(value.runtimeCode)

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
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="运行时" htmlFor={`${idPrefix}-runtime`} required>
              <Select
                id={`${idPrefix}-runtime`}
                options={catalog.runtimeOptions}
                value={value.runtimeCode}
                placeholder="选择运行时"
                onValueChange={(runtimeCode) =>
                  onChange({ runtimeCode, imageVersion: '', toolCodes: [], infraCodes: [] })
                }
              />
            </FormField>
            <FormField
              label="镜像版本"
              htmlFor={`${idPrefix}-image`}
              required
              helper="发布后按这个版本固定,平台换镜像不会改变已发布内容"
            >
              <Select
                id={`${idPrefix}-image`}
                options={imageOptions}
                value={value.imageVersion}
                placeholder={
                  value.runtimeCode === ''
                    ? '请先选择运行时'
                    : imageOptions.length > 0
                      ? '选择版本'
                      : '该运行时暂无可用版本'
                }
                disabled={imageOptions.length === 0}
                onValueChange={(imageVersion) => onChange({ ...value, imageVersion })}
              />
            </FormField>
          </div>

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
