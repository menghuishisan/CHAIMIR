// CompositionConnectionsFields 复用M2只读预览配置连接,请求失败或过期时保留选择并阻止提交。
import { useEffect, useRef } from 'react'
import { Link2 } from 'lucide-react'
import { Callout, FormField, Select, Skeleton } from '@chaimir/ui'
import { COMPOSITION_SELECTION, type CompositionLink } from '@chaimir/api-client'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { useAsyncResource } from '../../../hooks'
import {
  compositionConnectionKey,
  compositionPreviewRequest,
  type CompositionDeclaration,
} from '../composition'

interface CompositionConnectionsFieldsProps {
  idPrefix: string
  value: CompositionDeclaration
  onChange: (next: CompositionDeclaration) => void
}

/** linkKey 忽略JSON属性顺序,按唯一连接契约识别服务端候选。 */
function linkKey(link: CompositionLink): string {
  return JSON.stringify([link.source_component, link.source_endpoint, link.target_component,
    link.target_endpoint, link.protocol, link.required_at_start, link.access_scope, link.config_binding])
}

/** CompositionConnectionsFields 在每次选择变化后重新挂载,旧请求不覆盖新组合。 */
export function CompositionConnectionsFields({ idPrefix, value, onChange }: CompositionConnectionsFieldsProps) {
  const current = useRef({ value, onChange })
  current.current = { value, onChange }
  const applied = useRef<unknown>(null)
  const requestedKey = useRef('')
  const resource = useAsyncResource(
    () => {
      requestedKey.current = compositionConnectionKey(current.current.value)
      return api.sandbox.previewComposition(compositionPreviewRequest(current.current.value))
    },
    [],
  )

  useEffect(() => {
    if (resource.status !== 'success' || resource.data === null || applied.current === resource.data) return
    if (requestedKey.current !== compositionConnectionKey(current.current.value)) return
    applied.current = resource.data
    const next = { ...current.current.value, links: resource.data.links }
    const checked = { ...next, connectionCheck: {
      key: compositionConnectionKey(next), complete: resource.data.complete,
    } }
    current.current.value = checked
    current.current.onChange(checked)
  }, [resource.data, resource.status])

  // 重试和明确选目标都先撤销旧证明;重读成功前不能提交旧成功状态。
  const retry = () => {
    current.current.onChange({ ...current.current.value, connectionCheck: undefined })
    resource.reload()
  }

  return (
    <ResourceState
      resource={{ ...resource, reload: retry }}
      emptyIcon={Link2}
      emptyTitle="暂时没有可选连接"
      emptyDescription="请调整运行时或组件后重试。"
      skeleton={<div aria-live="polite"><p className="mb-2 text-sm text-ink-sub">正在检查组件连接,请稍候。</p><Skeleton variant="line" lines={2} /></div>}
    >
      {(preview) => (
        <div className="flex flex-col gap-3">
          {preview.infra.filter((item) => item.selection === COMPOSITION_SELECTION.AUTO).length > 0 ? (
            <Callout tone="info" title="平台将自动补齐支撑组件">
              {preview.infra.filter((item) => item.selection === COMPOSITION_SELECTION.AUTO).map((item) => item.code).join('、')}
            </Callout>
          ) : null}
          {preview.bindings.length === 0 ? (
            <p className="text-sm text-ink-sub">当前组件无需额外连接。</p>
          ) : preview.bindings.map((binding, index) => {
            const selected = value.links.find((link) => link.source_component === binding.source_component && link.config_binding === binding.config_binding)
            const selectedIndex = binding.options.findIndex((option) => selected !== undefined && linkKey(option.link) === linkKey(selected))
            return (
              <FormField
                key={`${binding.source_component}:${binding.config_binding}`}
                label={`${binding.source_name} · 连接 ${index + 1}`}
                htmlFor={`${idPrefix}-connection-${index}`}
                required
                helper={binding.options.length === 0 ? '当前没有可用目标,请调整运行时或组件。' : '选择此组件使用的服务;只有一个目标时会自动选好。'}
              >
                <Select
                  id={`${idPrefix}-connection-${index}`}
                  options={binding.options.map((option, optionIndex) => ({ label: option.label, value: String(optionIndex) }))}
                  value={selectedIndex < 0 ? '' : String(selectedIndex)}
                  placeholder="选择连接目标"
                  disabled={binding.options.length === 0}
                  onValueChange={(optionIndex) => {
                    const option = binding.options[Number(optionIndex)]
                    if (option === undefined) return
                    const latest = current.current.value
                    const next = { ...latest, links: [
                      ...latest.links.filter((link) => link.source_component !== binding.source_component || link.config_binding !== binding.config_binding),
                      option.link,
                    ], connectionCheck: undefined }
                    current.current.value = next
                    current.current.onChange(next)
                    resource.reload()
                  }}
                />
              </FormField>
            )
          })}
          {!preview.complete ? <Callout tone="warning">完成全部连接选择后才能保存;没有可用目标时请调整组件。</Callout> : null}
        </div>
      )}
    </ResourceState>
  )
}
