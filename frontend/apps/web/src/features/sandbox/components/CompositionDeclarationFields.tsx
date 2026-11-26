// 编排表单复用目录和连接预览；保存时仍须通过 M2 完整组合编译。

import { Plus, Server, Trash2 } from 'lucide-react'
import { Button, Callout, Checkbox, FormField, Input, Select, Skeleton } from '@chaimir/ui'
import type { SandboxCatalogTool } from '@chaimir/api-client'
import { ResourceState } from '../../../components/ResourceState'
import { sandboxToolKindLabel } from '../../../utils/labels/sandbox'
import { useOrchestrationCatalog } from '../useOrchestrationCatalog'
import {
  compositionConnectionKey,
  compositionSelectionError,
  compositionSelectionKey,
  reconcileRuntimeNames,
  type CompositionDeclaration,
} from '../composition'
import { currentConnectionCheck } from '../compositionCheck'
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
 * CompositionDeclarationFields 渲染运行时、组件和连接配置。
 * 运行时变化会清空依赖它的镜像与参数,最终兼容性由 M2 编译器确认。
 */
export function CompositionDeclarationFields({
  idPrefix,
  value,
  onChange: commit,
  toolsHelper,
}: CompositionDeclarationFieldsProps) {
  const catalog = useOrchestrationCatalog()
  // 选择变化后只保留仍匹配当前声明的预览证明。
  const onChange = (next: CompositionDeclaration) => {
    const { runtimes, renames } = reconcileRuntimeNames(value.runtimes, next.runtimes)
    next = { ...next, runtimes,
      workspaceRuntimeInstance: renames.get(next.workspaceRuntimeInstance) ?? next.workspaceRuntimeInstance,
      links: next.links.map((link) => ({ ...link,
        source_component: renames.get(link.source_component) ?? link.source_component,
        target_component: renames.get(link.target_component) ?? link.target_component,
      })),
    }
    commit({
      ...next,
      connectionCheck: currentConnectionCheck(next.connectionCheck, compositionConnectionKey(next)),
    })
  }
  const selectionError = compositionSelectionError(value)

  return (
    <ResourceState
      resource={catalog.resource}
      emptyIcon={Server}
      emptyTitle="平台还没有可用运行时"
      emptyDescription="请联系平台管理员完成运行时检查并开放使用。"
      skeleton={<Skeleton variant="line" lines={2} />}
    >
      {() => (
        <div className="flex flex-col gap-4">
          <FormField
            label="运行时实例"
            required
            helper="每个实例需要唯一别名,跨链连接和链操作都按别名定位。"
          >
            <div className="flex flex-col gap-3">
              {value.runtimes.map((runtime, index) => {
                const imageOptions = catalog.imageOptions(runtime.runtimeCode)
                return (
                  <div
                    key={runtime.clientId}
                    className="grid gap-3 rounded-md bg-surface-hover p-3 sm:grid-cols-4 sm:items-end"
                  >
                    <FormField
                      label="实例别名"
                      htmlFor={`${idPrefix}-runtime-${index}-instance`}
                      required
                    >
                      <Input
                        id={`${idPrefix}-runtime-${index}-instance`}
                        value={runtime.instanceCode}
                        placeholder="source-chain"
                        onChange={(event) =>
                          onChange({
                            ...value,
                            runtimes: value.runtimes.map((item, itemIndex) =>
                              itemIndex === index
                                ? { ...item, instanceCode: event.target.value }
                                : item
                            ),
                          })
                        }
                      />
                    </FormField>
                    <FormField
                      label="运行时"
                      htmlFor={`${idPrefix}-runtime-${index}-code`}
                      required
                    >
                      <Select
                        id={`${idPrefix}-runtime-${index}-code`}
                        options={catalog.runtimeOptions}
                        value={runtime.runtimeCode}
                        placeholder="选择运行时"
                        onValueChange={(runtimeCode) =>
                          onChange({
                            ...value,
                            runtimes: value.runtimes.map((item, itemIndex) =>
                              itemIndex === index
                                ? { ...item, runtimeCode, imageVersion: '', params: undefined }
                                : item
                            ),
                          })
                        }
                      />
                    </FormField>
                    <FormField
                      label="镜像版本"
                      htmlFor={`${idPrefix}-runtime-${index}-image`}
                      required
                    >
                      <Select
                        id={`${idPrefix}-runtime-${index}-image`}
                        options={imageOptions}
                        value={runtime.imageVersion}
                        placeholder={
                          runtime.runtimeCode === ''
                            ? '请先选择运行时'
                            : imageOptions.length > 0
                              ? '选择版本'
                              : '该运行时暂无可用版本'
                        }
                        disabled={imageOptions.length === 0}
                        onValueChange={(imageVersion) =>
                          onChange({
                            ...value,
                            runtimes: value.runtimes.map((item, itemIndex) =>
                              itemIndex === index ? { ...item, imageVersion } : item
                            ),
                          })
                        }
                      />
                    </FormField>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      aria-label={`移除运行时实例 ${runtime.instanceCode || index + 1}`}
                      title="移除运行时实例"
                      disabled={value.runtimes.length <= 1 || runtime.initializationBound === true}
                      onClick={() => {
                        const runtimes = value.runtimes.filter(
                          (_, itemIndex) => itemIndex !== index
                        )
                        onChange({
                          ...value,
                          runtimes,
                          workspaceRuntimeInstance:
                            runtime.instanceCode === value.workspaceRuntimeInstance
                              ? ''
                              : value.workspaceRuntimeInstance,
                          links: value.links.filter(
                            (link) =>
                              link.source_component !== runtime.instanceCode &&
                              link.target_component !== runtime.instanceCode
                          ),
                        })
                      }}
                    >
                      <Trash2 size={16} aria-hidden="true" />
                    </Button>
                    {runtime.initializationBound ? (
                      <p className="text-xs text-ink-sub sm:col-span-4">
                        初始化仍关联此实例，调整关联后才能移除；更名会同步保留关联。
                      </p>
                    ) : null}
                    <div className="sm:col-span-4">
                      <CompositionParameterFields
                        idPrefix={`${idPrefix}-runtime-${index}`}
                        schema={
                          catalog.resource.data?.runtimes.find(
                            (item) => item.code === runtime.runtimeCode
                          )?.capabilities.config_schema
                        }
                        value={runtime.params}
                        onChange={(params) =>
                          onChange({
                            ...value,
                            runtimes: value.runtimes.map((item, itemIndex) =>
                              itemIndex === index ? { ...item, params } : item
                            ),
                          })
                        }
                      />
                    </div>
                  </div>
                )
              })}
              <Button
                type="button"
                variant="outline"
                size="sm"
                leftIcon={Plus}
                onClick={() => {
                  let number = 1
                  while (
                    value.runtimes.some((runtime) => runtime.instanceCode === `chain-${number}`)
                  )
                    number++
                  onChange({
                    ...value,
                    runtimes: [
                      ...value.runtimes,
                      { clientId: crypto.randomUUID(), instanceCode: `chain-${number}`, runtimeCode: '', imageVersion: '' },
                    ],
                  })
                }}
              >
                添加运行时实例
              </Button>
            </div>
          </FormField>

          <FormField
            label="工作区运行时"
            required
            helper="文件、终端和初始化由这个实例的工作区能力承载。"
          >
            <Select
              id={`${idPrefix}-workspace-runtime`}
              options={value.runtimes
                .filter((runtime) => runtime.instanceCode.trim() !== '')
                .map((runtime) => ({ label: runtime.instanceCode, value: runtime.instanceCode }))}
              value={value.workspaceRuntimeInstance}
              placeholder="选择工作区运行时"
              onValueChange={(workspaceRuntimeInstance) =>
                onChange({ ...value, workspaceRuntimeInstance })
              }
            />
          </FormField>

          <FormField label="工具" helper={toolsHelper}>
            <ComponentChecklist
              items={catalog.tools}
              selected={value.toolCodes}
              params={value.componentParams}
              idPrefix={`${idPrefix}-tool`}
              onParamsChange={(code, params) =>
                onChange({
                  ...value,
                  componentParams: { ...value.componentParams, [code]: params },
                })
              }
              emptyHint="平台还没有可用工具,可只使用运行时自身能力。"
              onToggle={(toolCodes) =>
                onChange({
                  ...value,
                  toolCodes,
                  links: value.links.filter(
                    (link) =>
                      !value.toolCodes.some(
                        (code) =>
                          !toolCodes.includes(code) &&
                          (link.source_component === code || link.target_component === code)
                      )
                  ),
                })
              }
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
              onParamsChange={(code, params) =>
                onChange({
                  ...value,
                  componentParams: { ...value.componentParams, [code]: params },
                })
              }
              emptyHint="平台还没有可声明的基础设施组件。"
              onToggle={(infraCodes) =>
                onChange({
                  ...value,
                  infraCodes,
                  links: value.links.filter(
                    (link) =>
                      !value.infraCodes.some(
                        (code) =>
                          !infraCodes.includes(code) &&
                          (link.source_component === code || link.target_component === code)
                      )
                  ),
                })
              }
            />
          </FormField>

          {selectionError === undefined ? (
            <FormField label="组件连接" helper="平台会检查依赖和可用目标;多个目标时请明确选择。">
              <CompositionConnectionsFields
                key={compositionSelectionKey(value)}
                idPrefix={idPrefix}
                value={value}
                onChange={commit}
              />
            </FormField>
          ) : (
            <Callout tone="info">{selectionError}</Callout>
          )}
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
function ComponentChecklist({
  items,
  selected,
  emptyHint,
  onToggle,
  idPrefix,
  params,
  onParamsChange,
}: ComponentChecklistProps) {
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
                  : selected.filter((code) => code !== item.code)
              )
            }
          />
          {selected.includes(item.code) ? (
            <CompositionParameterFields
              idPrefix={`${idPrefix}-${item.code}`}
              schema={item.capabilities.config_schema}
              value={params?.[item.code]}
              onChange={(next) => onParamsChange(item.code, next)}
            />
          ) : null}
        </div>
      ))}
    </div>
  )
}
