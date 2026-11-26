import { useId, useState } from 'react'
import {
  SandboxToolKind,
  SANDBOX_BUILTIN_ENDPOINT_TEMPLATE_PREFIX,
  type SandboxToolResourceSpec,
  ToolStatus,
  type SandboxToolDefinition,
  type SandboxToolRequest,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
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
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { sandboxToolKindLabel, toolStatusLabel } from '../../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { SANDBOX_TOOL_KINDS, TOOL_STATUSES } from '../../options'
import { toolDraft, toolEditBlocker, toolResourceSpec, type ToolDraft } from '../../toolDraft'
import { validCommand } from '../../dynamicSpec'
import { CommandArgvFields } from '../../components/CommandArgvFields'

/** 工具编码规则,与后端 codePattern 一致。 */
const CODE_PATTERN = /^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/

const validPort = (text: string) => {
  const port = Number(text)
  return Number.isInteger(port) && port > 0 && port <= 65535
}

export interface ToolFormModalProps {
  /** 已有工具:抽屉同时是它的详情与修改;不传为登记新工具 */
  tool?: SandboxToolDefinition
  onClose: () => void
  onSaved: () => void
}

export function ToolFormModal({ tool, onClose, onSaved }: ToolFormModalProps) {
  const fieldId = useId()
  const editing = tool !== undefined
  // 表单表达不了的工具只能查看(基础设施、目录同步写入的停用组件、复杂网页工具)
  const blocker = tool ? toolEditBlocker(tool) : undefined
  const [draft, setDraft] = useState(() =>
    toolDraft(tool, SANDBOX_BUILTIN_ENDPOINT_TEMPLATE_PREFIX)
  )
  const {
    code,
    name,
    kind,
    ecoTags,
    status,
    builtinEndpoint,
    allowedArgv,
    defaultTimeout,
    maxTimeout,
    componentName,
    componentImage,
    componentCommand,
    webComponentName,
    webComponentImage,
    webPortName,
    webContainerPort,
    webServicePort,
    webServiceName,
    webRoutePrefix,
    prepullCommand,
  } = draft

  const setField = <K extends keyof ToolDraft>(field: K, value: ToolDraft[K]) =>
    setDraft((previous) => ({ ...previous, [field]: value }))

  const setAllowedArgv = (update: (rows: string[][]) => string[][]) =>
    setDraft((previous) => ({ ...previous, allowedArgv: update(previous.allowedArgv) }))

  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const kindValue = Number(kind) as SandboxToolKind

  const { errors, validate } = useFormValidation(() => {
    const next: Record<string, string | null> = {
      code: CODE_PATTERN.test(code.trim())
        ? null
        : '以小写字母或数字开头,只含小写字母、数字与连字符,长度 3 到 64 位',
      name: name.trim() === '' ? '请输入工具名称' : null,
      builtinEndpoint: null,
      allowedArgv: null,
      timeout: null,
      components: null,
      command: null,
      prepullCommand: null,
      webSpec: null,
    }

    if (kindValue === SandboxToolKind.COMMAND || kindValue === SandboxToolKind.WEB_EMBED) {
      if (!validCommand(prepullCommand)) {
        next.prepullCommand = '请逐项填写镜像自检命令及其参数'
      }
    }

    if (kindValue === SandboxToolKind.BUILTIN) {
      const endpoint = builtinEndpoint.trim()
      next.builtinEndpoint =
        endpoint.startsWith(SANDBOX_BUILTIN_ENDPOINT_TEMPLATE_PREFIX) && !endpoint.includes('://')
          ? null
          : `入口模板必须以 ${SANDBOX_BUILTIN_ENDPOINT_TEMPLATE_PREFIX} 开头,并且不能是完整网址`
    }

    if (kindValue === SandboxToolKind.COMMAND) {
      const defaultValue = Number(defaultTimeout)
      const maxValue = Number(maxTimeout)
      const argvList = allowedArgv.map((argv) => argv.map((item) => item.trim()))
      const keys = argvList.map((argv) => argv.join('\u0000'))
      next.allowedArgv =
        argvList.length === 0
          ? '请至少填写一条允许执行的完整命令'
          : allowedArgv.some((argv) => !validCommand(argv))
            ? '请补齐每条命令的参数，参数不能留空或包含控制字符'
            : new Set(keys).size !== keys.length
              ? '命令重复，请移除重复项'
              : null
      next.timeout =
        !Number.isInteger(defaultValue) ||
        !Number.isInteger(maxValue) ||
        defaultValue <= 0 ||
        maxValue <= 0
          ? '两个超时都要是大于 0 的整数秒'
          : defaultValue > maxValue
            ? '默认超时不能大于最长超时'
            : null
      next.components =
        componentName.trim() === '' || componentImage.trim() === ''
          ? '请填写执行环境名称和不可变镜像'
          : null
      next.command = validCommand(componentCommand) ? null : '请填写完整的启动命令参数'
    }

    if (kindValue === SandboxToolKind.WEB_EMBED) {
      next.webSpec =
        webComponentName.trim() === '' ||
        webComponentImage.trim() === '' ||
        webPortName.trim() === '' ||
        !validPort(webContainerPort) ||
        !validPort(webServicePort) ||
        webServiceName.trim() === '' ||
        !webRoutePrefix.startsWith('/')
          ? '请完整填写组件、端口、服务和代理路由'
          : null
    }

    return next
  })

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (submitting || blocker || !validate()) return
    setFormError(undefined)
    setSubmitting(true)
    try {
      const payload: SandboxToolRequest = {
        code: code.trim(),
        name: name.trim(),
        kind: kindValue,
        eco_tags: ecoTags
          .split(/[\s,，]+/)
          .map((item) => item.trim())
          .filter((item) => item !== ''),
        resource_spec: toolResourceSpec(
          draft,
          tool?.kind === kindValue ? (tool.resource_spec as Record<string, unknown>) : undefined
        ) as SandboxToolResourceSpec,
        status: Number(status) as ToolStatus,
      }
      await api.sandbox.registerTool(payload)
      toast.success(editing ? `已保存「${name.trim()}」` : '工具已登记')
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '登记没有成功,请检查声明后重试。'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Drawer open onOpenChange={(open) => !open && !submitting && onClose()}>
      <DrawerContent size="wide">
        <DrawerHeader>
          <DrawerTitle>{editing ? tool.name : '登记工具'}</DrawerTitle>
          <DrawerDescription>
            {editing
              ? `${sandboxToolKindLabel(tool.kind)} · 改动对之后创建的环境生效,已在运行的不受影响。不再使用的工具改成停用,不删除,历史环境还引用着它。`
              : '短名在平台内唯一。改动对之后创建的环境生效。'}
          </DrawerDescription>
        </DrawerHeader>
        <form onSubmit={submit} noValidate className="flex min-h-0 flex-1 flex-col">
          <DrawerBody className="flex flex-col gap-4">
            {blocker ? <Callout tone="info">{blocker}。</Callout> : null}
            {tool?.resource_spec.disabled_reason ? (
              <Callout tone="warning" title="当前不可部署">
                {tool.resource_spec.disabled_reason}
              </Callout>
            ) : null}
            <fieldset disabled={Boolean(blocker)} className="flex min-w-0 flex-col gap-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  label="工具短名"
                  htmlFor={`${fieldId}-code`}
                  required
                  error={errors.code}
                  helper="平台内唯一短名,例如 code-server"
                >
                  <Input
                    id={`${fieldId}-code`}
                    value={code}
                    disabled={editing}
                    placeholder="code-server"
                    invalid={Boolean(errors.code)}
                    onBlur={() => validate(['code'])}
                    onChange={(event) => setField('code', event.target.value)}
                  />
                </FormField>
                <FormField
                  label="工具名称"
                  htmlFor={`${fieldId}-name`}
                  required
                  error={errors.name}
                >
                  <Input
                    id={`${fieldId}-name`}
                    value={name}
                    placeholder="在线代码编辑器"
                    invalid={Boolean(errors.name)}
                    onBlur={() => validate(['name'])}
                    onChange={(event) => setField('name', event.target.value)}
                  />
                </FormField>
              </div>

              <FormField
                label="工具类型"
                htmlFor={`${fieldId}-kind`}
                required
                helper={KIND_HINTS[kindValue]}
              >
                <Select
                  id={`${fieldId}-kind`}
                  disabled={editing}
                  options={SANDBOX_TOOL_KINDS.map((item) => ({
                    value: String(item),
                    label: sandboxToolKindLabel(item),
                  }))}
                  value={kind}
                  onValueChange={(value) => setField('kind', value)}
                />
              </FormField>

              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  label="适用生态"
                  htmlFor={`${fieldId}-eco`}
                  helper="留空表示不限生态。多个用逗号或空格分隔"
                >
                  <Input
                    id={`${fieldId}-eco`}
                    value={ecoTags}
                    placeholder="ethereum, fabric"
                    onChange={(event) => setField('ecoTags', event.target.value)}
                  />
                </FormField>
                <FormField label="登记状态" required helper="停用的工具不会出现在学生工作台">
                  <SegmentedControl
                    aria-label="登记状态"
                    options={TOOL_STATUSES.map((item) => ({
                      value: String(item),
                      label: toolStatusLabel(item),
                    }))}
                    value={status}
                    onValueChange={(value) => setField('status', value)}
                  />
                </FormField>
              </div>

              {kindValue === SandboxToolKind.BUILTIN ? (
                <FormField
                  label="入口地址模板"
                  htmlFor={`${fieldId}-endpoint`}
                  required
                  error={errors.builtinEndpoint}
                  helper="平台会自动替换学生实验环境编号并生成真实入口"
                >
                  <Input
                    id={`${fieldId}-endpoint`}
                    className="font-mono text-sm"
                    value={builtinEndpoint}
                    invalid={Boolean(errors.builtinEndpoint)}
                    onBlur={() => validate(['builtinEndpoint'])}
                    onChange={(event) => setField('builtinEndpoint', event.target.value)}
                  />
                </FormField>
              ) : null}

              {kindValue === SandboxToolKind.TERMINAL ? (
                <Callout tone="info" title="终端工具不需要额外声明">
                  终端直连运行时容器,用的是运行时声明里的终端命令。填了容器或命令策略反而会被拒绝。
                </Callout>
              ) : null}

              {kindValue === SandboxToolKind.COMMAND ? (
                <>
                  <fieldset
                    className="flex flex-col gap-2"
                    aria-describedby={
                      errors.allowedArgv
                        ? `${fieldId}-allowed-argv-error`
                        : `${fieldId}-allowed-argv-help`
                    }
                    aria-invalid={errors.allowedArgv ? true : undefined}
                  >
                    <legend className="text-sm font-medium text-ink">
                      允许执行的命令
                      <span aria-hidden="true" className="text-seal">
                        {' '}
                        *
                      </span>
                      <span className="sr-only">必填</span>
                    </legend>
                    {!errors.allowedArgv ? (
                      <p id={`${fieldId}-allowed-argv-help`} className="text-xs text-ink-sub">
                        每个参数独立填写;平台只会放行与这些完整参数序列完全一致的命令。
                      </p>
                    ) : null}
                    <div className="flex flex-col gap-2">
                      {allowedArgv.map((argv, rowIndex) => (
                        <div key={rowIndex} className="flex flex-wrap items-center gap-2">
                          {argv.map((item, argIndex) => (
                            <Input
                              key={argIndex}
                              aria-label={`第 ${rowIndex + 1} 条命令的第 ${argIndex + 1} 个参数`}
                              className="min-w-28 flex-1 font-mono text-sm"
                              value={item}
                              placeholder={argIndex === 0 ? '命令' : '参数'}
                              onChange={(event) =>
                                setAllowedArgv((rows) =>
                                  rows.map((row, index) =>
                                    index === rowIndex
                                      ? row.map((value, position) =>
                                          position === argIndex ? event.target.value : value
                                        )
                                      : row
                                  )
                                )
                              }
                            />
                          ))}
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() =>
                              setAllowedArgv((rows) =>
                                rows.map((row, index) => (index === rowIndex ? [...row, ''] : row))
                              )
                            }
                          >
                            添加参数
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            disabled={argv.length <= 1}
                            onClick={() =>
                              setAllowedArgv((rows) =>
                                rows.map((row, index) =>
                                  index === rowIndex ? row.slice(0, -1) : row
                                )
                              )
                            }
                          >
                            移除参数
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            disabled={allowedArgv.length <= 1}
                            onClick={() =>
                              setAllowedArgv((rows) =>
                                rows.filter((_, index) => index !== rowIndex)
                              )
                            }
                          >
                            移除此命令
                          </Button>
                        </div>
                      ))}
                      <div>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => setAllowedArgv((rows) => [...rows, ['']])}
                        >
                          添加允许命令
                        </Button>
                      </div>
                    </div>
                    {errors.allowedArgv ? (
                      <p
                        id={`${fieldId}-allowed-argv-error`}
                        role="alert"
                        className="text-xs text-danger"
                      >
                        {errors.allowedArgv}
                      </p>
                    ) : null}
                  </fieldset>

                  <div className="grid gap-4 sm:grid-cols-2">
                    <FormField
                      label="默认超时(秒)"
                      htmlFor={`${fieldId}-default-timeout`}
                      required
                      error={errors.timeout}
                      helper="调用方没有指定超时时用这个值"
                    >
                      <Input
                        id={`${fieldId}-default-timeout`}
                        type="number"
                        min={1}
                        value={defaultTimeout}
                        invalid={Boolean(errors.timeout)}
                        onBlur={() => validate(['timeout'])}
                        onChange={(event) => setField('defaultTimeout', event.target.value)}
                      />
                    </FormField>
                    <FormField
                      label="最长超时(秒)"
                      htmlFor={`${fieldId}-max-timeout`}
                      required
                      helper="调用方指定的超时不能超过这个值"
                    >
                      <Input
                        id={`${fieldId}-max-timeout`}
                        type="number"
                        min={1}
                        value={maxTimeout}
                        invalid={Boolean(errors.timeout)}
                        onBlur={() => validate(['timeout'])}
                        onChange={(event) => setField('maxTimeout', event.target.value)}
                      />
                    </FormField>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-2">
                    <FormField
                      label="执行组件名称"
                      htmlFor={`${fieldId}-component-name`}
                      required
                      error={errors.components}
                      helper="命令工具只能有一个不开放端口的执行组件。"
                    >
                      <Input
                        id={`${fieldId}-component-name`}
                        value={componentName}
                        invalid={Boolean(errors.components)}
                        onBlur={() => validate(['components'])}
                        onChange={(event) => setField('componentName', event.target.value)}
                      />
                    </FormField>
                    <FormField label="执行镜像" htmlFor={`${fieldId}-component-image`} required>
                      <Input
                        id={`${fieldId}-component-image`}
                        className="font-mono text-sm"
                        value={componentImage}
                        placeholder="registry.chaimir.io/tool/...@sha256:..."
                        onChange={(event) => setField('componentImage', event.target.value)}
                      />
                    </FormField>
                  </div>
                  <CommandArgvFields
                    required
                    label="启动命令"
                    helper="填写保持执行组件运行的命令，每个参数单独填写。"
                    value={componentCommand}
                    error={errors.command}
                    onChange={(value) => setField('componentCommand', value)}
                  />
                </>
              ) : null}

              {kindValue === SandboxToolKind.WEB_EMBED ? (
                <>
                  <div className="grid gap-4 sm:grid-cols-2">
                    <FormField
                      label="网页组件名称"
                      htmlFor={`${fieldId}-web-component`}
                      required
                      error={errors.webSpec}
                    >
                      <Input
                        id={`${fieldId}-web-component`}
                        value={webComponentName}
                        invalid={Boolean(errors.webSpec)}
                        onBlur={() => validate(['webSpec'])}
                        onChange={(event) => setField('webComponentName', event.target.value)}
                      />
                    </FormField>
                    <FormField label="网页组件镜像" htmlFor={`${fieldId}-web-image`} required>
                      <Input
                        id={`${fieldId}-web-image`}
                        className="font-mono text-sm"
                        value={webComponentImage}
                        placeholder="registry.chaimir.io/tool/...@sha256:..."
                        onChange={(event) => setField('webComponentImage', event.target.value)}
                      />
                    </FormField>
                    <FormField label="组件端口名称" htmlFor={`${fieldId}-web-port`} required>
                      <Input
                        id={`${fieldId}-web-port`}
                        value={webPortName}
                        onChange={(event) => setField('webPortName', event.target.value)}
                      />
                    </FormField>
                    <FormField label="组件端口" htmlFor={`${fieldId}-web-container-port`} required>
                      <Input
                        id={`${fieldId}-web-container-port`}
                        type="number"
                        min={1}
                        max={65535}
                        value={webContainerPort}
                        onChange={(event) => setField('webContainerPort', event.target.value)}
                      />
                    </FormField>
                    <FormField label="服务名称" htmlFor={`${fieldId}-web-service`} required>
                      <Input
                        id={`${fieldId}-web-service`}
                        value={webServiceName}
                        onChange={(event) => setField('webServiceName', event.target.value)}
                      />
                    </FormField>
                    <FormField label="服务端口" htmlFor={`${fieldId}-web-service-port`} required>
                      <Input
                        id={`${fieldId}-web-service-port`}
                        type="number"
                        min={1}
                        max={65535}
                        value={webServicePort}
                        onChange={(event) => setField('webServicePort', event.target.value)}
                      />
                    </FormField>
                  </div>
                  <FormField
                    label="平台代理路径"
                    htmlFor={`${fieldId}-web-route`}
                    required
                    helper="必须以 / 开头,由平台在沙箱工具路径下代理。"
                  >
                    <Input
                      id={`${fieldId}-web-route`}
                      className="font-mono text-sm"
                      value={webRoutePrefix}
                      onChange={(event) => setField('webRoutePrefix', event.target.value)}
                    />
                  </FormField>
                </>
              ) : null}

              <Callout tone="info">
                镜像必须来自平台私有仓库并通过签名与漏洞扫描,登记时后端会校验。
              </Callout>
              {kindValue === SandboxToolKind.COMMAND || kindValue === SandboxToolKind.WEB_EMBED ? (
                <CommandArgvFields
                  required
                  label="镜像自检命令"
                  helper="填写能在镜像中独立执行并退出的检查命令，每个参数单独填写。"
                  value={prepullCommand}
                  error={errors.prepullCommand}
                  onChange={(value) => setField('prepullCommand', value)}
                />
              ) : null}
            </fieldset>
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </DrawerBody>
          <DrawerFooter>
            <Button type="button" variant="outline" disabled={submitting} onClick={onClose}>
              {blocker ? '关闭' : '取消'}
            </Button>
            {blocker ? null : (
              <Button type="submit" variant="primary" loading={submitting}>
                {editing ? '保存' : '登记工具'}
              </Button>
            )}
          </DrawerFooter>
        </form>
      </DrawerContent>
    </Drawer>
  )
}

const KIND_HINTS: Record<SandboxToolKind, string> = {
  [SandboxToolKind.BUILTIN]: '平台自带的面板,只需要入口地址模板',
  [SandboxToolKind.TERMINAL]: '直接连接运行时环境的终端,不需要任何额外声明',
  [SandboxToolKind.WEB_EMBED]: '独立环境提供网页界面,要声明组件、服务与代理路由',
  [SandboxToolKind.COMMAND]: '在白名单内执行命令,要声明一个执行环境与允许的命令',
  [SandboxToolKind.INFRA]: '只参与组合编排的基础设施,不提供学生工具入口',
}
