// 沙箱工具登记表单(沙箱工具页内)。
//
// 四类工具的声明形态互斥,后端 validateToolResourceSpecShape 对每类都有必填与禁填:
//   平台内置:只要入口地址模板,不能带组件、命令策略或网络规则;
//   终端:什么都不要,带任何声明都会被拒;
//   网页工具:组件、服务、代理路由三者都必须有,不能带命令策略;
//   命令工具:恰好一个组件 + 完整 argv 白名单,不能有服务、路由或网络规则。
// 故表单按类型分叉,只渲染该类型允许的字段 —— 把四类字段并排摊出来,填错的概率高于填对。
//
// 容器声明本身是不定长的编排清单(镜像、端口、探针、挂载),键不可枚举,
// 命令白名单、组件、端口、服务与路由使用字段级控件编辑，保存时由表单状态组装受控声明。

import { useCallback, useId, useState } from 'react'
import {
  SandboxToolKind,
  SANDBOX_BUILTIN_ENDPOINT_TEMPLATE_PREFIX,
  ToolStatus,
  type SandboxToolRequest,
  type SandboxToolResourceSpec,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
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
import { api } from '../../../../app/api'
import { sandboxToolKindLabel, toolStatusLabel } from '../../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { SANDBOX_TOOL_KINDS, TOOL_STATUSES } from '../../options'

/** 工具编码规则,与后端 codePattern 一致。 */
const CODE_PATTERN = /^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/

export interface ToolFormModalProps {
  onClose: () => void
  onSaved: () => void
}

/**
 * ToolFormModal 承载沙箱工具的登记。
 * 后端是同一条 POST 做创建与覆盖(按编码 upsert),故这里只有一个登记入口,
 * 修改已有工具就是用同一个编码再登记一次 —— 表单里明确说明这一点。
 */
export function ToolFormModal({ onClose, onSaved }: ToolFormModalProps) {
  const fieldId = useId()

  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [kind, setKind] = useState(String(SandboxToolKind.COMMAND))
  const [ecoTags, setEcoTags] = useState('')
  const [status, setStatus] = useState(String(ToolStatus.AVAILABLE))

  const [builtinEndpoint, setBuiltinEndpoint] = useState(SANDBOX_BUILTIN_ENDPOINT_TEMPLATE_PREFIX)
  const [allowedArgv, setAllowedArgv] = useState<string[][]>([['']])
  const [defaultTimeout, setDefaultTimeout] = useState('30')
  const [maxTimeout, setMaxTimeout] = useState('120')
  const [componentName, setComponentName] = useState('compiler')
  const [componentImage, setComponentImage] = useState('')
  const [componentCommand, setComponentCommand] = useState<string[]>(['sleep'])
  const [webComponentName, setWebComponentName] = useState('editor')
  const [webComponentImage, setWebComponentImage] = useState('')
  const [webPortName, setWebPortName] = useState('http')
  const [webContainerPort, setWebContainerPort] = useState('8080')
  const [webServicePort, setWebServicePort] = useState('8080')
  const [webServiceName, setWebServiceName] = useState('editor')
  const [webRoutePrefix, setWebRoutePrefix] = useState('/')

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const kindValue = Number(kind) as SandboxToolKind

  /** buildResourceSpec 按类型只组装该类型允许的字段,其余一律不带。 */
  const buildResourceSpec = useCallback((): SandboxToolResourceSpec => {
    switch (kindValue) {
      case SandboxToolKind.BUILTIN:
        return { builtin_endpoint: builtinEndpoint.trim() }
      case SandboxToolKind.COMMAND:
        return {
          components: [
            {
              name: componentName.trim(),
              image_url: componentImage.trim(),
              command: componentCommand.map((item) => item.trim()).filter(Boolean),
              mount_workspace: true,
            },
          ],
          command_policy: {
            allowed_argv: allowedArgv.map((argv) =>
              argv.map((item) => item.trim()).filter(Boolean)
            ),
            default_timeout_seconds: Number(defaultTimeout),
            max_timeout_seconds: Number(maxTimeout),
          },
        }
      case SandboxToolKind.WEB_EMBED:
        return {
          components: [
            {
              name: webComponentName.trim(),
              image_url: webComponentImage.trim(),
              ports: [
                {
                  name: webPortName.trim(),
                  container_port: Number(webContainerPort),
                  service_port: Number(webServicePort),
                  protocol: 'TCP',
                },
              ],
              mount_workspace: true,
            },
          ],
          services: [
            {
              name: webServiceName.trim(),
              component: webComponentName.trim(),
              ports: [
                {
                  name: webPortName.trim(),
                  port: Number(webServicePort),
                  target_port: webPortName.trim(),
                  protocol: 'TCP',
                },
              ],
            },
          ],
          routes: [
            {
              path_prefix: webRoutePrefix.trim(),
              service: webServiceName.trim(),
              port: webPortName.trim(),
            },
          ],
        }
      default:
        // 终端工具不带任何声明:后端对它禁填组件、入口模板、命令策略与网络规则
        return {}
    }
  }, [
    builtinEndpoint,
    allowedArgv,
    componentCommand,
    componentImage,
    componentName,
    defaultTimeout,
    kindValue,
    maxTimeout,
    webComponentImage,
    webComponentName,
    webContainerPort,
    webPortName,
    webRoutePrefix,
    webServiceName,
    webServicePort,
  ])

  /** validate 按类型逐项校验,校验口径与后端 validateToolResourceSpecShape 对齐。 */
  const validate = useCallback((): boolean => {
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
      webSpec: null,
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
      const argvList = allowedArgv.map((argv) => argv.map((item) => item.trim()).filter(Boolean))
      const keys = argvList.map((argv) => argv.join('\u0000'))
      next.allowedArgv =
        argvList.length === 0
          ? '至少写一个完整的 argv 白名单'
          : argvList.some((argv) => argv.length === 0)
            ? '每条白名单至少要有一个命令参数'
            : argvList.some((argv) => argv.some((item) => item.includes('\u0000')))
              ? 'argv 参数不能包含控制字符'
              : new Set(keys).size !== keys.length
                ? '有重复的 argv 白名单,去掉重复项'
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
      next.command =
        componentCommand.length === 0 || componentCommand.some((item) => item.trim() === '')
          ? '请填写完整的启动命令参数'
          : null
    }

    if (kindValue === SandboxToolKind.WEB_EMBED) {
      next.webSpec =
        webComponentName.trim() === '' ||
        webComponentImage.trim() === '' ||
        webPortName.trim() === '' ||
        Number(webContainerPort) <= 0 ||
        Number(webServicePort) <= 0 ||
        webServiceName.trim() === '' ||
        !webRoutePrefix.startsWith('/')
          ? '请完整填写组件、端口、服务和代理路由'
          : null
    }

    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [
    builtinEndpoint,
    code,
    allowedArgv,
    componentCommand,
    componentImage,
    componentName,
    defaultTimeout,
    kindValue,
    maxTimeout,
    name,
    webComponentImage,
    webComponentName,
    webContainerPort,
    webPortName,
    webRoutePrefix,
    webServiceName,
    webServicePort,
  ])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) {
        setFormError('有几项还不能提交,按提示改一下。')
        return
      }
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
          resource_spec: buildResourceSpec(),
          status: Number(status) as ToolStatus,
        }
        await api.sandbox.registerTool(payload)
        toast.success('工具已登记')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '登记没有成功,请检查声明后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [buildResourceSpec, code, ecoTags, kindValue, name, onSaved, status, validate]
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>登记沙箱工具</ModalTitle>
          <ModalDescription>
            用已存在的短名再登记一次就是覆盖原定义。改动对之后创建的环境生效。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
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
                  placeholder="code-server"
                  invalid={Boolean(errors.code)}
                  onChange={(event) => setCode(event.target.value)}
                />
              </FormField>
              <FormField label="工具名称" htmlFor={`${fieldId}-name`} required error={errors.name}>
                <Input
                  id={`${fieldId}-name`}
                  value={name}
                  placeholder="在线代码编辑器"
                  invalid={Boolean(errors.name)}
                  onChange={(event) => setName(event.target.value)}
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
                options={SANDBOX_TOOL_KINDS.map((item) => ({
                  value: String(item),
                  label: sandboxToolKindLabel(item),
                }))}
                value={kind}
                onValueChange={setKind}
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
                  onChange={(event) => setEcoTags(event.target.value)}
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
                  onValueChange={setStatus}
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
                  onChange={(event) => setBuiltinEndpoint(event.target.value)}
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
                            setAllowedArgv((rows) => rows.filter((_, index) => index !== rowIndex))
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
                      onChange={(event) => setDefaultTimeout(event.target.value)}
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
                      onChange={(event) => setMaxTimeout(event.target.value)}
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
                      onChange={(event) => setComponentName(event.target.value)}
                    />
                  </FormField>
                  <FormField label="执行镜像" htmlFor={`${fieldId}-component-image`} required>
                    <Input
                      id={`${fieldId}-component-image`}
                      className="font-mono text-sm"
                      value={componentImage}
                      placeholder="registry.chaimir.io/tool/...@sha256:..."
                      onChange={(event) => setComponentImage(event.target.value)}
                    />
                  </FormField>
                </div>
                <CommandArgvEditor
                  value={componentCommand}
                  error={errors.command}
                  onChange={setComponentCommand}
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
                      onChange={(event) => setWebComponentName(event.target.value)}
                    />
                  </FormField>
                  <FormField label="网页组件镜像" htmlFor={`${fieldId}-web-image`} required>
                    <Input
                      id={`${fieldId}-web-image`}
                      className="font-mono text-sm"
                      value={webComponentImage}
                      placeholder="registry.chaimir.io/tool/...@sha256:..."
                      onChange={(event) => setWebComponentImage(event.target.value)}
                    />
                  </FormField>
                  <FormField label="组件端口名称" htmlFor={`${fieldId}-web-port`} required>
                    <Input
                      id={`${fieldId}-web-port`}
                      value={webPortName}
                      onChange={(event) => setWebPortName(event.target.value)}
                    />
                  </FormField>
                  <FormField label="组件端口" htmlFor={`${fieldId}-web-container-port`} required>
                    <Input
                      id={`${fieldId}-web-container-port`}
                      type="number"
                      min={1}
                      max={65535}
                      value={webContainerPort}
                      onChange={(event) => setWebContainerPort(event.target.value)}
                    />
                  </FormField>
                  <FormField label="服务名称" htmlFor={`${fieldId}-web-service`} required>
                    <Input
                      id={`${fieldId}-web-service`}
                      value={webServiceName}
                      onChange={(event) => setWebServiceName(event.target.value)}
                    />
                  </FormField>
                  <FormField label="服务端口" htmlFor={`${fieldId}-web-service-port`} required>
                    <Input
                      id={`${fieldId}-web-service-port`}
                      type="number"
                      min={1}
                      max={65535}
                      value={webServicePort}
                      onChange={(event) => setWebServicePort(event.target.value)}
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
                    onChange={(event) => setWebRoutePrefix(event.target.value)}
                  />
                </FormField>
              </>
            ) : null}

            <Callout tone="info">
              镜像必须来自平台私有仓库并通过签名与漏洞扫描,登记时后端会校验。
            </Callout>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={submitting}>
              登记工具
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** 各类工具的选型说明,与列表页同源口径。 */
const KIND_HINTS: Record<SandboxToolKind, string> = {
  [SandboxToolKind.BUILTIN]: '平台自带的面板,只需要入口地址模板',
  [SandboxToolKind.TERMINAL]: '直接连接运行时环境的终端,不需要任何额外声明',
  [SandboxToolKind.WEB_EMBED]: '独立环境提供网页界面,要声明组件、服务与代理路由',
  [SandboxToolKind.COMMAND]: '在白名单内执行命令,要声明一个执行环境与允许的命令',
}

function CommandArgvEditor({
  value,
  error,
  onChange,
}: {
  value: string[]
  error?: string | null
  onChange: (value: string[]) => void
}) {
  const fieldId = useId()
  const helperId = `${fieldId}-helper`
  const errorId = `${fieldId}-error`

  return (
    <fieldset
      className="flex flex-col gap-2"
      aria-describedby={error ? errorId : helperId}
      aria-invalid={error ? true : undefined}
    >
      <legend className="text-sm font-medium text-ink">
        启动命令
        <span aria-hidden="true" className="text-seal">
          {' '}
          *
        </span>
        <span className="sr-only">必填</span>
      </legend>
      {error ? (
        <p id={errorId} role="alert" className="text-xs text-danger">
          {error}
        </p>
      ) : (
        <p id={helperId} className="text-xs text-ink-sub">
          每个参数独立填写,不经过 shell。
        </p>
      )}
      {value.map((item, index) => (
        <div key={`component-argv-${index}`} className="flex items-center gap-2">
          <Input
            aria-label={`启动命令第 ${index + 1} 个参数`}
            className="min-w-0 flex-1 font-mono text-sm"
            value={item}
            placeholder={index === 0 ? '命令' : '参数'}
            onChange={(event) =>
              onChange(
                value.map((part, partIndex) => (partIndex === index ? event.target.value : part))
              )
            }
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={value.length <= 1}
            onClick={() => onChange(value.filter((_, partIndex) => partIndex !== index))}
          >
            移除
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={() => onChange([...value, ''])}>
        添加参数
      </Button>
    </fieldset>
  )
}
