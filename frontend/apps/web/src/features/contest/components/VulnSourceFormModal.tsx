// 教师与平台共用漏洞源配置表单，写入范围由调用方显式指定。

import { useId, useState } from 'react'
import {
  VulnLevel,
  type VulnSource,
  type VulnSourceRequest,
  type VulnSourceType,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
  Checkbox,
  FormField,
  Input,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  SegmentedControl,
  Select,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { useFormValidation } from '../../../hooks/useFormValidation'
import { vulnLevelLabel, vulnSourceTypeLabel } from '../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { isHttpUrl } from '../../../utils/validators'
import { VULN_LEVELS, VULN_SOURCE_METHODS, VULN_SOURCE_TYPES } from '../options'
import { readString } from '../jsonReaders'
import {
  VULN_SOURCE_CONFIG_FIELDS,
  VULN_SOURCE_MAPPING_FIELDS,
  VULN_SOURCE_TIMEOUT_MIN,
  VULN_SOURCE_TIMEOUT_MAX,
  parseVulnSourceRequestFields,
} from '../vulnerabilityContract'

export interface VulnSourceFormModalProps {
  /** 传入即为编辑模式;缺省为新建 */
  source?: VulnSource
  /** 写入平台共享来源，服务端仍校验平台管理权限。 */
  global?: boolean
  onClose: () => void
  onSaved: () => void
}

/** VulnSourceFormModal 编辑学校或平台来源，密钥仅在本次编辑期间留在内存。 */
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
    readString(source?.config, VULN_SOURCE_CONFIG_FIELDS.endpoint)
  )
  const [method, setMethod] = useState(
    readString(source?.config, VULN_SOURCE_CONFIG_FIELDS.method).trim().toUpperCase() ||
      VULN_SOURCE_METHODS[0]
  )
  const [timeoutSeconds, setTimeoutSeconds] = useState(
    source?.config.timeout_seconds === undefined ? '' : String(source.config.timeout_seconds)
  )
  const [casesPath, setCasesPath] = useState(
    readString(source?.config, VULN_SOURCE_CONFIG_FIELDS.casesPath)
  )
  const [mapping, setMapping] = useState(() => readMapping(source))
  const [headers, setHeaders] = useState(() =>
    source?.config.headers === undefined ? '' : JSON.stringify(source.config.headers, null, 2)
  )
  const [body, setBody] = useState(() =>
    source?.config.body === undefined ? '' : JSON.stringify(source.config.body, null, 2)
  )

  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const requestFields = parseVulnSourceRequestFields(headers, body, timeoutSeconds)
  const { errors, validate } = useFormValidation(() => {
    const next: Record<string, string | null> = {
      ...requestFields.errors,
      name: name.trim() === '' ? '请输入来源名称' : [...name.trim()].length > 128 ? '来源名称不能超过 128 个字' : null,
      endpoint: !isHttpUrl(endpoint) ? '请输入完整的接口地址,以 http:// 或 https:// 开头' : null,
      externalRef: mapping.externalRef.trim() === '' ? '请填写案例编号所在的字段路径' : null,
      title: mapping.title.trim() === '' ? '请填写标题所在的字段路径' : null,
      draftBody: mapping.draftBody.trim() === '' ? '请填写题目正文所在的字段路径' : null,
    }
    return next
  })

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!validate()) return

    const payload: VulnSourceRequest = {
      id: source?.id,
      type: Number(type) as VulnSourceType,
      name: name.trim(),
      default_level: Number(defaultLevel) as VulnLevel,
      enabled,
      config: {
        ...requestFields.config,
        [VULN_SOURCE_CONFIG_FIELDS.endpoint]: endpoint.trim(),
        [VULN_SOURCE_CONFIG_FIELDS.method]: method,
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
  }

  return (
    <Drawer open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent size="wide">
        <DrawerHeader>
          <DrawerTitle>{editing ? '修改漏洞来源' : '添加漏洞来源'}</DrawerTitle>
          <DrawerDescription>
            来源是一个返回漏洞案例列表的接口，可按来源要求填写访问凭证。字段映射指定要读取的信息。
            {global ? '这里维护的是全平台共享的来源,所有学校都能从它同步案例。' : ''}
          </DrawerDescription>
        </DrawerHeader>
        <form onSubmit={submit} noValidate className="flex min-h-0 flex-1 flex-col">
          <DrawerBody className="flex flex-col gap-4">
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
              label="来源地址"
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
              <FormField label="获取方式" required>
                <SegmentedControl
                  aria-label="获取方式"
                  size="sm"
                  options={VULN_SOURCE_METHODS.map((item) => ({ value: item, label: item }))}
                  value={method}
                  onValueChange={setMethod}
                />
              </FormField>
              <FormField
                label="等待时间(秒)"
                htmlFor={`${fieldId}-timeout`}
                error={errors.timeoutSeconds}
                helper="留空使用平台设置，填写后仅对当前来源生效"
              >
                <Input
                  id={`${fieldId}-timeout`}
                  type="number"
                  min={VULN_SOURCE_TIMEOUT_MIN}
                  max={VULN_SOURCE_TIMEOUT_MAX}
                  step={1}
                  value={timeoutSeconds}
                  invalid={Boolean(errors.timeoutSeconds)}
                  onChange={(event) => setTimeoutSeconds(event.target.value)}
                />
              </FormField>
              <FormField
                label="案例列表位置"
                htmlFor={`${fieldId}-cases`}
                helper="案例列表在数据中的位置,例如 data.items;数据本身就是列表时留空"
              >
                <Input
                  id={`${fieldId}-cases`}
                  value={casesPath}
                  placeholder="data.items"
                  onChange={(event) => setCasesPath(event.target.value)}
                />
              </FormField>
            </div>

            <FormField
              label="请求头"
              htmlFor={`${fieldId}-headers`}
              error={errors.headers}
              helper={'按来源要求填写，例如 {"Authorization":"Bearer 访问凭证"}；不需要时留空'}
            >
              <Textarea
                id={`${fieldId}-headers`}
                value={headers}
                invalid={Boolean(errors.headers)}
                onChange={(event) => setHeaders(event.target.value)}
                autoComplete="off"
                spellCheck={false}
                className="font-mono"
              />
            </FormField>
            <FormField
              label="请求内容"
              htmlFor={`${fieldId}-body`}
              error={errors.body}
              helper="使用 POST 获取时发送，格式按来源要求填写；切换获取方式会保留这里的内容"
            >
              <Textarea
                id={`${fieldId}-body`}
                value={body}
                invalid={Boolean(errors.body)}
                onChange={(event) => setBody(event.target.value)}
                autoComplete="off"
                spellCheck={false}
                className="font-mono"
              />
            </FormField>

            <div className="flex flex-col gap-4 rounded-md bg-surface-hover p-4">
              <div>
                <p className="text-base text-ink">数据对应关系</p>
                <p className="text-sm text-ink-sub">
                  填写外部数据中对应信息的位置。嵌套内容用点号连接,如{' '}
                  <span className="font-mono">detail.body</span>。
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
                    placeholder="填写案例编号所在字段"
                    invalid={Boolean(errors.externalRef)}
                    onChange={(event) =>
                      patchMapping(setMapping, { externalRef: event.target.value })
                    }
                  />
                </FormField>
                <FormField
                  label="标题"
                  htmlFor={`${fieldId}-map-title`}
                  required
                  error={errors.title}
                >
                  <Input
                    id={`${fieldId}-map-title`}
                    value={mapping.title}
                    placeholder="填写标题所在字段"
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
                  placeholder="填写题目正文所在字段"
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
                    placeholder="填写分级所在字段"
                    onChange={(event) => patchMapping(setMapping, { level: event.target.value })}
                  />
                </FormField>
                <FormField
                  label="复现方式"
                  htmlFor={`${fieldId}-map-runtime`}
                  helper="留空则按干净测试链复现;填写来源响应中复现方式所在字段"
                >
                  <Input
                    id={`${fieldId}-map-runtime`}
                    value={mapping.runtimeMode}
                    placeholder="填写复现方式所在字段"
                    onChange={(event) =>
                      patchMapping(setMapping, { runtimeMode: event.target.value })
                    }
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
                访问凭证不会回显。保留“已配置”可继续使用原凭证，填写新值可替换，删除对应字段或填空值可清除。
              </Callout>
            ) : null}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </DrawerBody>
          <DrawerFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={submitting}>
              {editing ? '保存配置' : '添加来源'}
            </Button>
          </DrawerFooter>
        </form>
      </DrawerContent>
    </Drawer>
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
  patch: Partial<MappingState>
): void {
  setState((current) => ({ ...current, ...patch }))
}
