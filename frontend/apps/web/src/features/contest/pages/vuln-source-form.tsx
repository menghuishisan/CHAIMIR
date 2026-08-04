// 漏洞源配置表单(教师漏洞题工坊页内 / 平台漏洞题源页内共用)。
//
// 后端 validateVulnSourceConfig 对 config 有明确要求:公网 HTTP(S) 接口地址、GET/POST、
// 1-60 秒超时、字段映射里 external_ref/title/draft_body 必填。故这里按这些键渲染结构化字段,
// 不给裸 JSON 文本域 —— 手写 JSON 的结果是保存时才发现键名拼错。
//
// 字段映射是「外部响应的哪个路径对应漏洞题的哪个字段」,后端按 JSON 路径取值。
//
// 租户源与平台全局源的表单完全相同,差别只在落到哪张归属上(POST /contest/vuln-sources
// 写本校源,POST /contest/platform/vuln-sources 写 tenant_id=0 的全局源),
// 故由调用方通过 global 显式声明,不复制第二份表单。

import { useCallback, useId, useState } from 'react'
import { VulnLevel, type VulnSource, type VulnSourceRequest } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Checkbox,
  FormField,
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
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import {
  VULN_LEVELS,
  VULN_SOURCE_CONFIG_FIELDS,
  VULN_SOURCE_MAPPING_FIELDS,
  VULN_SOURCE_METHODS,
  VULN_SOURCE_TYPES,
  vulnLevelLabel,
  vulnSourceTypeLabel,
} from '../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

/** 后端允许的超时区间(秒),与 validateVulnSourceConfig 一致。 */
const TIMEOUT_MIN = 1
const TIMEOUT_MAX = 60

export interface VulnSourceFormModalProps {
  /** 传入即为编辑模式;缺省为新建 */
  source?: VulnSource
  /**
   * 是否写平台全局源。
   * 全局源 tenant_id=0、对所有学校可见,只有平台管理员能维护(后端 UpsertPlatformVulnSource
   * 断言平台身份);租户源只在本校可见。由调用方显式声明,组件不判角色枚举。
   */
  global?: boolean
  onClose: () => void
  onSaved: () => void
}

/**
 * VulnSourceFormModal 承载漏洞源的创建与修改。
 */
export function VulnSourceFormModal({
  source,
  global = false,
  onClose,
  onSaved,
}: VulnSourceFormModalProps) {
  const fieldId = useId()
  const editing = source !== undefined

  const [name, setName] = useState(source?.name ?? '')
  const [type, setType] = useState(String(source?.type ?? VULN_SOURCE_TYPES[0]))
  const [defaultLevel, setDefaultLevel] = useState(String(source?.default_level ?? VulnLevel.B))
  const [enabled, setEnabled] = useState(source?.enabled ?? true)
  const [endpoint, setEndpoint] = useState(
    readString(source?.config, VULN_SOURCE_CONFIG_FIELDS.endpoint),
  )
  const [method, setMethod] = useState(
    readString(source?.config, VULN_SOURCE_CONFIG_FIELDS.method) || VULN_SOURCE_METHODS[0],
  )
  const [timeoutSeconds, setTimeoutSeconds] = useState(
    String(readNumber(source?.config, VULN_SOURCE_CONFIG_FIELDS.timeoutSeconds, 15)),
  )
  const [casesPath, setCasesPath] = useState(
    readString(source?.config, VULN_SOURCE_CONFIG_FIELDS.casesPath),
  )
  const [mapping, setMapping] = useState(() => readMapping(source))

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  /** validate 按后端 validateVulnSourceConfig 的要求校验必填与取值范围。 */
  const validate = useCallback((): boolean => {
    const timeout = Number(timeoutSeconds)
    const next: Record<string, string | null> = {
      name: name.trim() === '' ? '请输入来源名称' : null,
      endpoint: !isHttpUrl(endpoint) ? '请输入完整的接口地址,以 http:// 或 https:// 开头' : null,
      timeoutSeconds:
        !Number.isFinite(timeout) || timeout < TIMEOUT_MIN || timeout > TIMEOUT_MAX
          ? `超时需要在 ${TIMEOUT_MIN} 到 ${TIMEOUT_MAX} 秒之间`
          : null,
      externalRef: mapping.externalRef.trim() === '' ? '请填写案例编号所在的字段路径' : null,
      title: mapping.title.trim() === '' ? '请填写标题所在的字段路径' : null,
      draftBody: mapping.draftBody.trim() === '' ? '请填写题目正文所在的字段路径' : null,
    }
    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [endpoint, mapping, name, timeoutSeconds])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return

      // 配置按结构化字段组装:键名与后端读取的键一致,不接受用户手写 JSON
      const payload: VulnSourceRequest = {
        id: source?.id,
        type: Number(type),
        name: name.trim(),
        default_level: Number(defaultLevel) as VulnLevel,
        enabled,
        config: {
          [VULN_SOURCE_CONFIG_FIELDS.endpoint]: endpoint.trim(),
          [VULN_SOURCE_CONFIG_FIELDS.method]: method,
          [VULN_SOURCE_CONFIG_FIELDS.timeoutSeconds]: Number(timeoutSeconds),
          ...(casesPath.trim() !== ''
            ? { [VULN_SOURCE_CONFIG_FIELDS.casesPath]: casesPath.trim() }
            : {}),
          [VULN_SOURCE_CONFIG_FIELDS.mapping]: {
            [VULN_SOURCE_MAPPING_FIELDS.externalRef]: mapping.externalRef.trim(),
            [VULN_SOURCE_MAPPING_FIELDS.title]: mapping.title.trim(),
            [VULN_SOURCE_MAPPING_FIELDS.draftBody]: mapping.draftBody.trim(),
            ...(mapping.level.trim() !== ''
              ? { [VULN_SOURCE_MAPPING_FIELDS.level]: mapping.level.trim() }
              : {}),
            ...(mapping.runtimeMode.trim() !== ''
              ? { [VULN_SOURCE_MAPPING_FIELDS.runtimeMode]: mapping.runtimeMode.trim() }
              : {}),
          },
        },
      }

      setFormError(undefined)
      setSubmitting(true)
      try {
        if (global) await api.contest.upsertPlatformVulnSource(payload)
        else await api.contest.upsertVulnSource(payload)
        toast.success(editing ? '来源配置已更新' : '来源已添加')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请检查配置后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [
      casesPath,
      defaultLevel,
      editing,
      enabled,
      endpoint,
      global,
      mapping,
      method,
      name,
      onSaved,
      source?.id,
      timeoutSeconds,
      type,
      validate,
    ],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? '修改漏洞来源' : '添加漏洞来源'}</ModalTitle>
          <ModalDescription>
            来源是一个返回漏洞案例列表的公开接口。字段映射告诉系统该读响应里的哪些字段。
            {global ? '这里维护的是全平台共享的来源,所有学校都能从它同步案例。' : ''}
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="来源名称" htmlFor={`${fieldId}-name`} required error={errors.name}>
              <Input
                id={`${fieldId}-name`}
                value={name}
                invalid={Boolean(errors.name)}
                onChange={(event) => setName(event.target.value)}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="来源类型" htmlFor={`${fieldId}-type`} required>
                <Select
                  id={`${fieldId}-type`}
                  options={VULN_SOURCE_TYPES.map((item) => ({
                    value: String(item),
                    label: vulnSourceTypeLabel(item),
                  }))}
                  value={type}
                  onValueChange={setType}
                />
              </FormField>
              <FormField
                label="默认可复现性分级"
                htmlFor={`${fieldId}-level`}
                required
                helper="来源响应里没有分级时按这个填"
              >
                <Select
                  id={`${fieldId}-level`}
                  options={VULN_LEVELS.map((level) => ({
                    value: String(level),
                    label: vulnLevelLabel(level),
                  }))}
                  value={defaultLevel}
                  onValueChange={setDefaultLevel}
                />
              </FormField>
            </div>

            <FormField
              label="接口地址"
              htmlFor={`${fieldId}-endpoint`}
              required
              error={errors.endpoint}
              helper="必须是可公网访问的地址,内网地址会被拒绝"
            >
              <Input
                id={`${fieldId}-endpoint`}
                value={endpoint}
                placeholder="https://example.org/api/vulnerabilities"
                invalid={Boolean(errors.endpoint)}
                onChange={(event) => setEndpoint(event.target.value)}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-3">
              <FormField label="请求方式" required>
                <SegmentedControl
                  aria-label="请求方式"
                  size="sm"
                  options={VULN_SOURCE_METHODS.map((item) => ({ value: item, label: item }))}
                  value={method}
                  onValueChange={setMethod}
                />
              </FormField>
              <FormField
                label="超时(秒)"
                htmlFor={`${fieldId}-timeout`}
                required
                error={errors.timeoutSeconds}
              >
                <Input
                  id={`${fieldId}-timeout`}
                  type="number"
                  min={TIMEOUT_MIN}
                  max={TIMEOUT_MAX}
                  value={timeoutSeconds}
                  invalid={Boolean(errors.timeoutSeconds)}
                  onChange={(event) => setTimeoutSeconds(event.target.value)}
                />
              </FormField>
              <FormField
                label="案例列表路径"
                htmlFor={`${fieldId}-cases`}
                helper="响应里案例数组的位置,如 data.items;响应本身就是数组则留空"
              >
                <Input
                  id={`${fieldId}-cases`}
                  value={casesPath}
                  placeholder="data.items"
                  onChange={(event) => setCasesPath(event.target.value)}
                />
              </FormField>
            </div>

            <div className="flex flex-col gap-4 rounded-md border border-line bg-surface-sunken p-4">
              <div>
                <p className="text-base text-ink">字段映射</p>
                <p className="text-sm text-ink-sub">
                  填写外部响应里对应字段的路径。嵌套字段用点号连接,如 <span className="font-mono">detail.body</span>。
                </p>
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  label="案例编号"
                  htmlFor={`${fieldId}-map-ref`}
                  required
                  error={errors.externalRef}
                  helper="用于去重,同一编号再同步会更新而不是新增"
                >
                  <Input
                    id={`${fieldId}-map-ref`}
                    value={mapping.externalRef}
                    placeholder="id"
                    invalid={Boolean(errors.externalRef)}
                    onChange={(event) => patchMapping(setMapping, { externalRef: event.target.value })}
                  />
                </FormField>
                <FormField label="标题" htmlFor={`${fieldId}-map-title`} required error={errors.title}>
                  <Input
                    id={`${fieldId}-map-title`}
                    value={mapping.title}
                    placeholder="title"
                    invalid={Boolean(errors.title)}
                    onChange={(event) => patchMapping(setMapping, { title: event.target.value })}
                  />
                </FormField>
              </div>

              <FormField
                label="题目正文"
                htmlFor={`${fieldId}-map-body`}
                required
                error={errors.draftBody}
                helper="承载合约源码、初始化交易、断言与 PoC 的那个对象"
              >
                <Input
                  id={`${fieldId}-map-body`}
                  value={mapping.draftBody}
                  placeholder="detail"
                  invalid={Boolean(errors.draftBody)}
                  onChange={(event) => patchMapping(setMapping, { draftBody: event.target.value })}
                />
              </FormField>

              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  label="可复现性分级"
                  htmlFor={`${fieldId}-map-level`}
                  helper="留空则用上面的默认分级"
                >
                  <Input
                    id={`${fieldId}-map-level`}
                    value={mapping.level}
                    placeholder="severity"
                    onChange={(event) => patchMapping(setMapping, { level: event.target.value })}
                  />
                </FormField>
                <FormField
                  label="复现方式"
                  htmlFor={`${fieldId}-map-runtime`}
                  helper="留空则按干净测试链复现"
                >
                  <Input
                    id={`${fieldId}-map-runtime`}
                    value={mapping.runtimeMode}
                    placeholder="runtime_mode"
                    onChange={(event) => patchMapping(setMapping, { runtimeMode: event.target.value })}
                  />
                </FormField>
              </div>
            </div>

            <Checkbox
              checked={enabled}
              label="启用这个来源(启用后才能同步案例)"
              onCheckedChange={(checked) => setEnabled(checked === true)}
            />

            {editing ? (
              <Callout tone="info">
                密钥类配置由后端加密保存、不回显。留空表示保持原值不变。
              </Callout>
            ) : null}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={submitting}>
              {editing ? '保存配置' : '添加来源'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** MappingState 是字段映射五个键的编辑态。 */
interface MappingState {
  externalRef: string
  title: string
  level: string
  runtimeMode: string
  draftBody: string
}

/** readMapping 从来源配置里取出字段映射;缺失或类型不符回空串。 */
function readMapping(source: VulnSource | undefined): MappingState {
  const raw = source?.config?.[VULN_SOURCE_CONFIG_FIELDS.mapping]
  const mapping = typeof raw === 'object' && raw !== null ? (raw as Record<string, unknown>) : {}
  return {
    externalRef: readString(mapping, VULN_SOURCE_MAPPING_FIELDS.externalRef),
    title: readString(mapping, VULN_SOURCE_MAPPING_FIELDS.title),
    level: readString(mapping, VULN_SOURCE_MAPPING_FIELDS.level),
    runtimeMode: readString(mapping, VULN_SOURCE_MAPPING_FIELDS.runtimeMode),
    draftBody: readString(mapping, VULN_SOURCE_MAPPING_FIELDS.draftBody),
  }
}

/** patchMapping 局部更新映射编辑态。 */
function patchMapping(
  setState: React.Dispatch<React.SetStateAction<MappingState>>,
  patch: Partial<MappingState>,
): void {
  setState((current) => ({ ...current, ...patch }))
}

/** readString 从开放对象里读字符串字段;被脱敏的字段回空串,不把掩码值当输入回填。 */
function readString(source: Record<string, unknown> | undefined, key: string): string {
  const value = source?.[key]
  return typeof value === 'string' ? value : ''
}

/** readNumber 从开放对象里读数字字段;非数字回默认值。 */
function readNumber(source: Record<string, unknown> | undefined, key: string, fallback: number): number {
  const value = source?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

/** isHttpUrl 前端先挡明显非法地址;公网可达性由后端最终判定。 */
function isHttpUrl(value: string): boolean {
  const trimmed = value.trim()
  return trimmed.startsWith('http://') || trimmed.startsWith('https://')
}
