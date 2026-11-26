// CompositionConnectionsFields 展示服务端编译出的连接候选并保存选择证明。

import { useEffect } from 'react'
import { Link2 } from 'lucide-react'
import { Callout, Checkbox, FormField, Select, Skeleton } from '@chaimir/ui'
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

function linkKey(link: CompositionLink): string {
  return JSON.stringify([
    link.source_component,
    link.source_endpoint,
    link.target_component,
    link.target_endpoint,
    link.protocol,
    link.required_at_start,
    link.access_scope,
    link.config_binding,
  ])
}

export function CompositionConnectionsFields({
  idPrefix,
  value,
  onChange,
}: CompositionConnectionsFieldsProps) {
  const requestKey = compositionConnectionKey(value)
  const resource = useAsyncResource(
    () => api.sandbox.previewComposition(compositionPreviewRequest(value)),
    [requestKey]
  )
  const preview = resource.status === 'success' ? resource.data : null

  useEffect(() => {
    if (preview === null) return
    const next = { ...value, links: preview.links }
    const connectionCheck = {
      key: compositionConnectionKey(next),
      complete: preview.complete,
    }
    if (
      value.connectionCheck?.key === connectionCheck.key &&
      value.connectionCheck.complete === connectionCheck.complete
    )
      return
    onChange({ ...next, connectionCheck })
  }, [preview, value, onChange])

  const retry = () => {
    onChange({ ...value, connectionCheck: undefined })
    resource.reload()
  }

  return (
    <ResourceState
      resource={{ ...resource, reload: retry }}
      emptyIcon={Link2}
      emptyTitle="暂时没有可选连接"
      emptyDescription="请调整运行时或组件后重试。"
      skeleton={
        <div aria-live="polite">
          <p className="mb-2 text-sm text-ink-sub">正在检查组件连接，请稍候。</p>
          <Skeleton variant="line" lines={2} />
        </div>
      }
    >
      {(result) => {
        const automaticInfra = result.infra.filter(
          (item) => item.selection === COMPOSITION_SELECTION.AUTO
        )
        return (
          <div className="flex flex-col gap-3">
            {automaticInfra.length > 0 ? (
              <Callout tone="info" title="平台将自动补齐支撑组件">
                {automaticInfra.map((item) => item.code).join('、')}
              </Callout>
            ) : null}
            {result.bindings.length === 0 ? (
              <p className="text-sm text-ink-sub">当前组件无需额外连接。</p>
            ) : (
              result.bindings.map((binding, index) => {
                const matchesBinding = (link: CompositionLink) =>
                  link.source_component === binding.source_component &&
                  link.config_binding === binding.config_binding
                const inputId = `${idPrefix}-connection-${index}`
                const label = `${binding.source_name} · ${binding.title || `连接 ${index + 1}`}`
                const helper = binding.optional
                  ? '按需选择目标；不选时保持未连接。'
                  : binding.options.length === 0
                    ? '当前没有可用目标，请调整运行时或组件。'
                    : binding.multiple
                      ? '至少选择一个目标，可以多选。'
                      : '只有一个目标时会自动选好。'
                if (binding.multiple) {
                  const selectedKeys = new Set(value.links.filter(matchesBinding).map(linkKey))
                  return (
                    <fieldset
                      key={`${binding.source_component}:${binding.config_binding}`}
                      className="flex flex-col gap-2"
                      aria-describedby={`${inputId}-help`}
                    >
                      <legend className="text-sm font-medium text-ink">{label}</legend>
                      <p id={`${inputId}-help`} className="text-xs text-ink-sub">
                        {helper}
                      </p>
                      {binding.options.map((option) => {
                        const key = linkKey(option.link)
                        return (
                          <Checkbox
                            key={key}
                            label={option.label}
                            checked={selectedKeys.has(key)}
                            onCheckedChange={(checked) => {
                              onChange({
                                ...value,
                                links: [
                                  ...value.links.filter((link) => linkKey(link) !== key),
                                  ...(checked === true ? [option.link] : []),
                                ],
                                connectionCheck: undefined,
                              })
                            }}
                          />
                        )
                      })}
                    </fieldset>
                  )
                }
                const selected = value.links.find(matchesBinding)
                const selectedIndex =
                  selected === undefined
                    ? -1
                    : binding.options.findIndex(
                        (option) => linkKey(option.link) === linkKey(selected)
                      )
                return (
                  <FormField
                    key={`${binding.source_component}:${binding.config_binding}`}
                    label={label}
                    htmlFor={inputId}
                    required={!binding.optional}
                    helper={helper}
                  >
                    <Select
                      id={inputId}
                      options={[
                        ...(binding.optional ? [{ label: '不连接', value: 'none' }] : []),
                        ...binding.options.map((option, optionIndex) => ({
                          label: option.label,
                          value: String(optionIndex),
                        })),
                      ]}
                      value={
                        selectedIndex < 0 ? (binding.optional ? 'none' : '') : String(selectedIndex)
                      }
                      placeholder="选择连接目标"
                      disabled={binding.options.length === 0}
                      onValueChange={(optionIndex) => {
                        const option = binding.options[Number(optionIndex)]
                        if (option === undefined && !(binding.optional && optionIndex === 'none'))
                          return
                        onChange({
                          ...value,
                          links: [
                            ...value.links.filter((link) => !matchesBinding(link)),
                            ...(option === undefined ? [] : [option.link]),
                          ],
                          connectionCheck: undefined,
                        })
                      }}
                    />
                  </FormField>
                )
              })
            )}
            {!result.complete ? <Callout tone="warning">完成连接选择后才能保存。</Callout> : null}
          </div>
        )
      }}
    </ResourceState>
  )
}
