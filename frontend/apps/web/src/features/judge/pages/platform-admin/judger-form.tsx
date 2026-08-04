// 判题器配置表单(判题器页内)。
//
// 后端 parseJudgerResourceSpec 按类型分别要求不同字段:
//   测试用例 / 静态扫描:必须有链环境三件套 + 执行命令 + 执行目标 + 至少一个执行容器;
//   链上断言:必须有链环境三件套(命令由断言执行器内置);
//   flag 比对 / 仿真检查点:不强制链环境,勾了「需要链环境」才要;
//   人工评分:不能带执行命令(它根本不执行)。
// 故本表单按类型只渲染该类型真正用到的字段,并按同一口径先在前端校验。
//
// 链环境编码、镜像版本与工具编码都从已登记的资源里选,不让人手打 ——
// 打错的结果是判题任务在运行期才失败,那时学生已经交了作业。

import { useCallback, useId, useMemo, useState } from 'react'
import {
  JudgerStatus,
  JudgerType,
  type Judger,
  type JudgerRequest,
  type JudgerResourceSpec,
  type WorkloadComponent,
} from '@chaimir/api-client'
import {
  Badge,
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
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useAsyncResource } from '../../../../hooks'
import {
  JUDGER_STATUSES,
  JUDGER_TYPES,
  judgerStatusLabel,
  judgerTypeLabel,
} from '../../../../utils/labels/judge'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 判题器编码规则,与后端 codePattern 一致。 */
const CODE_PATTERN = /^[a-z][a-z0-9-]{0,30}[a-z0-9]$/

/** 执行目标规则:pod/container 两段,各自都是受控编码(后端 safeExecTarget)。 */
const EXEC_TARGET_PATTERN = /^[a-z][a-z0-9-]*[a-z0-9]\/[a-z][a-z0-9-]*[a-z0-9]$/

/** 必须绑定链环境的类型:后端对这三类强制要求链环境三件套。 */
const RUNTIME_BOUND_TYPES: readonly JudgerType[] = [
  JudgerType.TESTCASE,
  JudgerType.ONCHAIN_ASSERT,
  JudgerType.STATIC_SCAN,
]

/** 必须给出执行入口的类型:命令、执行目标与执行容器三者都不能缺。 */
const COMMAND_BOUND_TYPES: readonly JudgerType[] = [JudgerType.TESTCASE, JudgerType.STATIC_SCAN]

/** 执行容器清单骨架:判题在这些容器里跑,跑完即销毁。 */
const SIDECAR_TEMPLATE = `[
  {
    "name": "runner",
    "image_url": "",
    "command": ["sleep", "infinity"],
    "mount_workspace": true
  }
]`

export interface JudgerFormModalProps {
  /** 传入即为修改模式;缺省为新增。修改时编码不可变 */
  judger?: Judger
  onClose: () => void
  onSaved: () => void
}

/**
 * JudgerFormModal 承载判题器的登记与修改。
 */
export function JudgerFormModal({ judger, onClose, onSaved }: JudgerFormModalProps) {
  const fieldId = useId()
  const editing = judger !== undefined
  const spec = judger?.resource_spec

  const [code, setCode] = useState(judger?.code ?? '')
  const [name, setName] = useState(judger?.name ?? '')
  const [type, setType] = useState(String(judger?.type ?? JudgerType.TESTCASE))
  const [executorRef, setExecutorRef] = useState(judger?.executor_ref ?? '')
  const [runtimeRequired, setRuntimeRequired] = useState(judger?.runtime_required ?? false)
  const [defaultTimeout, setDefaultTimeout] = useState(String(judger?.default_timeout_sec ?? 120))
  const [status, setStatus] = useState(String(judger?.status ?? JudgerStatus.AVAILABLE))

  const [runtimeCode, setRuntimeCode] = useState(spec?.runtime_code ?? '')
  const [imageVersion, setImageVersion] = useState(spec?.runtime_image_version ?? '')
  const [genesisRef, setGenesisRef] = useState(spec?.genesis_ref ?? '')
  const [toolCodes, setToolCodes] = useState<string[]>(spec?.tool_codes ?? [])
  const [initScriptRef, setInitScriptRef] = useState(spec?.init_script_ref ?? '')
  const [command, setCommand] = useState((spec?.command ?? []).join(' '))
  const [execTarget, setExecTarget] = useState(spec?.exec_target ?? '')
  const [suiteArchiveName, setSuiteArchiveName] = useState(spec?.suite_archive_name ?? '')
  const [timeoutSec, setTimeoutSec] = useState(String(spec?.timeout_sec ?? 0))
  const [maxRetries, setMaxRetries] = useState(String(spec?.max_retries ?? 0))
  const [sidecarText, setSidecarText] = useState(() =>
    spec?.execution_sidecars && spec.execution_sidecars.length > 0
      ? JSON.stringify(spec.execution_sidecars, null, 2)
      : SIDECAR_TEMPLATE,
  )

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const typeValue = Number(type) as JudgerType
  const needsRuntime = runtimeRequired || RUNTIME_BOUND_TYPES.includes(typeValue)
  const needsCommand = COMMAND_BOUND_TYPES.includes(typeValue)
  const isManual = typeValue === JudgerType.MANUAL

  // 链环境与工具从已登记资源里选:选项来自平台自己的登记表,避免手打编码
  const runtimes = useAsyncResource(() => api.sandbox.listRuntimes(), [], () => false)
  const tools = useAsyncResource(() => api.sandbox.listTools(), [], () => false)

  const selectedRuntime = useMemo(
    () => (runtimes.data ?? []).find((item) => item.code === runtimeCode),
    [runtimeCode, runtimes.data],
  )

  const images = useAsyncResource(
    () =>
      selectedRuntime
        ? api.sandbox.listRuntimeImages(selectedRuntime.id)
        : Promise.resolve([]),
    [selectedRuntime?.id],
    () => false,
  )

  const commandList = useMemo(
    () =>
      command
        .split(/\s+/)
        .map((item) => item.trim())
        .filter((item) => item !== ''),
    [command],
  )

  const sidecarsParsed = useMemo(() => parseSidecars(sidecarText), [sidecarText])

  /** validate 按类型逐项校验,口径与后端 parseJudgerResourceSpec 一致。 */
  const validate = useCallback((): boolean => {
    const timeoutValue = Number(timeoutSec)
    const retriesValue = Number(maxRetries)
    const defaultTimeoutValue = Number(defaultTimeout)

    const next: Record<string, string | null> = {
      code: CODE_PATTERN.test(code.trim())
        ? null
        : '用小写字母开头,只含小写字母、数字与连字符,长度 2 到 32 位',
      name: name.trim() === '' ? '请输入判题器名称' : null,
      executorRef: executorRef.trim() === '' ? '请填写执行器标识' : null,
      defaultTimeout:
        !Number.isInteger(defaultTimeoutValue) || defaultTimeoutValue <= 0
          ? '默认超时要是大于 0 的整数秒'
          : null,
      timeoutSec:
        !Number.isInteger(timeoutValue) || timeoutValue < 0 ? '覆盖超时要是 0 或更大的整数秒' : null,
      maxRetries:
        !Number.isInteger(retriesValue) || retriesValue < 0 ? '重试次数要是 0 或更大的整数' : null,
      runtimeCode: null,
      imageVersion: null,
      genesisRef: null,
      command: null,
      execTarget: null,
      sidecars: null,
    }

    if (needsRuntime) {
      next.runtimeCode = runtimeCode.trim() === '' ? '请选择判题时使用的链环境' : null
      next.imageVersion = imageVersion.trim() === '' ? '请选择链环境的镜像版本' : null
      next.genesisRef = genesisRef.trim() === '' ? '请填写创世状态标识' : null
    }

    if (needsCommand) {
      next.command = commandList.length === 0 ? '请填写判题要执行的命令' : null
      next.execTarget = EXEC_TARGET_PATTERN.test(execTarget.trim())
        ? null
        : '执行目标写成「容器组/容器」两段,例如 runner/runner'
      next.sidecars = sidecarsParsed.error ?? null
    }

    if (isManual && commandList.length > 0) {
      next.command = '人工评分不执行命令,请清空这一栏'
    }

    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [
    code,
    commandList,
    defaultTimeout,
    execTarget,
    executorRef,
    genesisRef,
    imageVersion,
    isManual,
    maxRetries,
    name,
    needsCommand,
    needsRuntime,
    runtimeCode,
    sidecarsParsed.error,
    timeoutSec,
  ])

  /** buildResourceSpec 只带该类型真正用到的字段,空值一律不写进声明。 */
  const buildResourceSpec = useCallback((): JudgerResourceSpec => {
    const out: JudgerResourceSpec = {}
    if (needsRuntime) {
      out.runtime_code = runtimeCode.trim()
      out.runtime_image_version = imageVersion.trim()
      out.genesis_ref = genesisRef.trim()
    }
    if (toolCodes.length > 0) out.tool_codes = toolCodes
    if (initScriptRef.trim() !== '') out.init_script_ref = initScriptRef.trim()
    if (needsCommand) {
      out.command = commandList
      out.exec_target = execTarget.trim()
      out.execution_sidecars = sidecarsParsed.sidecars
    }
    if (suiteArchiveName.trim() !== '') out.suite_archive_name = suiteArchiveName.trim()
    if (Number(timeoutSec) > 0) out.timeout_sec = Number(timeoutSec)
    if (Number(maxRetries) > 0) out.max_retries = Number(maxRetries)
    // 自测样例由部署侧随判题器镜像提供,表单不改动;修改时原样带回,避免保存一次就丢
    if (spec?.selftest) out.selftest = spec.selftest
    return out
  }, [
    commandList,
    execTarget,
    genesisRef,
    imageVersion,
    initScriptRef,
    maxRetries,
    needsCommand,
    needsRuntime,
    runtimeCode,
    sidecarsParsed.sidecars,
    spec?.selftest,
    suiteArchiveName,
    timeoutSec,
    toolCodes,
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
        const payload: JudgerRequest = {
          code: code.trim(),
          name: name.trim(),
          type: typeValue,
          executor_ref: executorRef.trim(),
          runtime_required: needsRuntime,
          default_timeout_sec: Number(defaultTimeout),
          resource_spec: buildResourceSpec(),
          status: Number(status) as JudgerStatus,
        }
        if (editing) await api.judge.updateJudger(judger.id, payload)
        else await api.judge.createJudger(payload)
        toast.success(editing ? '判题器配置已更新' : '判题器已登记,建议先自测一次')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请检查配置后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [
      buildResourceSpec,
      code,
      defaultTimeout,
      editing,
      executorRef,
      judger?.id,
      name,
      needsRuntime,
      onSaved,
      status,
      typeValue,
      validate,
    ],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? '修改判题器配置' : '登记判题器'}</ModalTitle>
          <ModalDescription>
            判题器决定一道题怎么判分。不同类型需要的声明不同,选好类型后表单会跟着变。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="判题器编码"
                htmlFor={`${fieldId}-code`}
                required
                error={errors.code}
                helper={editing ? '编码在登记后不能修改' : '教师出题时按这个编码引用'}
              >
                <Input
                  id={`${fieldId}-code`}
                  value={code}
                  disabled={editing}
                  placeholder="solidity-testcase"
                  invalid={Boolean(errors.code)}
                  onChange={(event) => setCode(event.target.value)}
                />
              </FormField>
              <FormField label="判题器名称" htmlFor={`${fieldId}-name`} required error={errors.name}>
                <Input
                  id={`${fieldId}-name`}
                  value={name}
                  placeholder="Solidity 测试用例判题"
                  invalid={Boolean(errors.name)}
                  onChange={(event) => setName(event.target.value)}
                />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="判题方式" htmlFor={`${fieldId}-type`} required>
                <Select
                  id={`${fieldId}-type`}
                  options={JUDGER_TYPES.map((item) => ({
                    value: String(item),
                    label: judgerTypeLabel(item),
                  }))}
                  value={type}
                  onValueChange={setType}
                />
              </FormField>
              <FormField
                label="执行器标识"
                htmlFor={`${fieldId}-executor`}
                required
                error={errors.executorRef}
                helper="平台按这个标识找到对应的判题实现"
              >
                <Input
                  id={`${fieldId}-executor`}
                  className="font-mono text-sm"
                  value={executorRef}
                  placeholder="builtin-testcase"
                  invalid={Boolean(errors.executorRef)}
                  onChange={(event) => setExecutorRef(event.target.value)}
                />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-3">
              <FormField
                label="默认超时(秒)"
                htmlFor={`${fieldId}-default-timeout`}
                required
                error={errors.defaultTimeout}
                helper="单个判题任务的时间上限"
              >
                <Input
                  id={`${fieldId}-default-timeout`}
                  type="number"
                  min={1}
                  value={defaultTimeout}
                  invalid={Boolean(errors.defaultTimeout)}
                  onChange={(event) => setDefaultTimeout(event.target.value)}
                />
              </FormField>
              <FormField
                label="覆盖超时(秒)"
                htmlFor={`${fieldId}-timeout`}
                error={errors.timeoutSec}
                helper="填 0 表示用上面的默认超时"
              >
                <Input
                  id={`${fieldId}-timeout`}
                  type="number"
                  min={0}
                  value={timeoutSec}
                  invalid={Boolean(errors.timeoutSec)}
                  onChange={(event) => setTimeoutSec(event.target.value)}
                />
              </FormField>
              <FormField
                label="失败重试次数"
                htmlFor={`${fieldId}-retries`}
                error={errors.maxRetries}
                helper="填 0 表示用平台默认重试策略"
              >
                <Input
                  id={`${fieldId}-retries`}
                  type="number"
                  min={0}
                  value={maxRetries}
                  invalid={Boolean(errors.maxRetries)}
                  onChange={(event) => setMaxRetries(event.target.value)}
                />
              </FormField>
            </div>

            <FormField label="登记状态" required helper="停用后不再分配新任务">
              <SegmentedControl
                aria-label="登记状态"
                options={JUDGER_STATUSES.map((item) => ({
                  value: String(item),
                  label: judgerStatusLabel(item),
                }))}
                value={status}
                onValueChange={setStatus}
              />
            </FormField>

            {RUNTIME_BOUND_TYPES.includes(typeValue) ? (
              <Callout tone="info">
                这种判题方式必须在链环境里跑,下面的链环境三项是必填的。
              </Callout>
            ) : (
              <Checkbox
                checked={runtimeRequired}
                label="判题时需要起一个链环境"
                onCheckedChange={(checked) => setRuntimeRequired(checked === true)}
              />
            )}

            {needsRuntime ? (
              <div className="flex flex-col gap-4 rounded-md border border-line bg-surface-sunken p-4">
                <div>
                  <p className="text-base text-ink">链环境</p>
                  <p className="text-sm text-ink-sub">
                    判题时会按这个运行时与镜像版本起一个一次性沙箱,判完即销毁。
                  </p>
                </div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <FormField
                    label="运行时"
                    htmlFor={`${fieldId}-runtime`}
                    required
                    error={errors.runtimeCode}
                    helper="只列已登记的链运行时"
                  >
                    <Select
                      id={`${fieldId}-runtime`}
                      options={(runtimes.data ?? []).map((item) => ({
                        value: item.code,
                        label: `${item.name} · ${item.code}`,
                      }))}
                      value={runtimeCode}
                      placeholder={(runtimes.data ?? []).length > 0 ? '选择运行时' : '还没有登记运行时'}
                      disabled={(runtimes.data ?? []).length === 0}
                      onValueChange={(value) => {
                        setRuntimeCode(value)
                        setImageVersion('')
                      }}
                    />
                  </FormField>
                  <FormField
                    label="镜像版本"
                    htmlFor={`${fieldId}-image`}
                    required
                    error={errors.imageVersion}
                    helper={selectedRuntime ? '只列这个运行时下的版本' : '先选运行时'}
                  >
                    <Select
                      id={`${fieldId}-image`}
                      options={(images.data ?? []).map((item) => ({
                        value: item.version,
                        label: item.is_default ? `${item.version}(默认)` : item.version,
                      }))}
                      value={imageVersion}
                      placeholder={
                        !selectedRuntime
                          ? '先选运行时'
                          : (images.data ?? []).length > 0
                            ? '选择版本'
                            : '这个运行时还没有镜像版本'
                      }
                      disabled={!selectedRuntime || (images.data ?? []).length === 0}
                      onValueChange={setImageVersion}
                    />
                  </FormField>
                </div>

                <FormField
                  label="创世状态标识"
                  htmlFor={`${fieldId}-genesis`}
                  required
                  error={errors.genesisRef}
                  helper="判题从这个初始链状态开始,保证每次判分条件一致"
                >
                  <Input
                    id={`${fieldId}-genesis`}
                    className="font-mono text-sm"
                    value={genesisRef}
                    placeholder="genesis-default"
                    invalid={Boolean(errors.genesisRef)}
                    onChange={(event) => setGenesisRef(event.target.value)}
                  />
                </FormField>
              </div>
            ) : null}

            <FormField
              label="需要的沙箱工具"
              helper="判题容器里要用到的工具,只列已登记的。不需要就都不选"
            >
              <div className="flex flex-col gap-2">
                {(tools.data ?? []).length === 0 ? (
                  <span className="text-sm text-ink-sub">还没有登记沙箱工具。</span>
                ) : (
                  (tools.data ?? []).map((tool) => (
                    <Checkbox
                      key={tool.id}
                      checked={toolCodes.includes(tool.code)}
                      label={`${tool.name} · ${tool.code}`}
                      onCheckedChange={(checked) =>
                        setToolCodes((current) =>
                          checked === true
                            ? [...current, tool.code]
                            : current.filter((item) => item !== tool.code),
                        )
                      }
                    />
                  ))
                )}
              </div>
            </FormField>

            {toolCodes.length > 0 ? (
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-sm text-ink-sub">已选工具:</span>
                {toolCodes.map((item) => (
                  <Badge key={item} tone="jade">
                    {item}
                  </Badge>
                ))}
              </div>
            ) : null}

            {needsCommand ? (
              <div className="flex flex-col gap-4 rounded-md border border-line bg-surface-sunken p-4">
                <div>
                  <p className="text-base text-ink">执行入口</p>
                  <p className="text-sm text-ink-sub">
                    判题在这些容器里执行指定命令。命令按空格拆成参数,不经过 shell。
                  </p>
                </div>

                <FormField
                  label="执行命令"
                  htmlFor={`${fieldId}-command`}
                  required
                  error={errors.command}
                  helper="按空格拆成参数,不支持管道与重定向"
                >
                  <Input
                    id={`${fieldId}-command`}
                    className="font-mono text-sm"
                    value={command}
                    placeholder="forge test --json"
                    invalid={Boolean(errors.command)}
                    onChange={(event) => setCommand(event.target.value)}
                  />
                </FormField>

                <div className="grid gap-4 sm:grid-cols-2">
                  <FormField
                    label="执行目标"
                    htmlFor={`${fieldId}-exec-target`}
                    required
                    error={errors.execTarget}
                    helper="写成「容器组/容器」两段,指向下面声明的执行容器"
                  >
                    <Input
                      id={`${fieldId}-exec-target`}
                      className="font-mono text-sm"
                      value={execTarget}
                      placeholder="runner/runner"
                      invalid={Boolean(errors.execTarget)}
                      onChange={(event) => setExecTarget(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="测试集压缩包名"
                    htmlFor={`${fieldId}-suite`}
                    helper="题目附带的测试集在容器里的文件名。留空表示不需要"
                  >
                    <Input
                      id={`${fieldId}-suite`}
                      className="font-mono text-sm"
                      value={suiteArchiveName}
                      placeholder="tests.tar"
                      onChange={(event) => setSuiteArchiveName(event.target.value)}
                    />
                  </FormField>
                </div>

                <FormField
                  label="执行容器声明"
                  htmlFor={`${fieldId}-sidecars`}
                  required
                  error={errors.sidecars}
                  helper="判题跑在这些容器里,判完即销毁。至少一个"
                >
                  <Textarea
                    id={`${fieldId}-sidecars`}
                    className="font-mono text-sm"
                    value={sidecarText}
                    rows={12}
                    spellCheck={false}
                    invalid={Boolean(errors.sidecars)}
                    onChange={(event) => setSidecarText(event.target.value)}
                  />
                </FormField>
              </div>
            ) : null}

            <FormField
              label="初始化脚本标识"
              htmlFor={`${fieldId}-init`}
              helper="判题开始前先跑一段准备脚本。留空表示不需要"
            >
              <Input
                id={`${fieldId}-init`}
                className="font-mono text-sm"
                value={initScriptRef}
                onChange={(event) => setInitScriptRef(event.target.value)}
              />
            </FormField>

            {isManual ? (
              <Callout tone="info" title="人工评分不执行任何命令">
                这类判题器只把任务挂到待评分,由教师在批改中心打分。填了执行命令会被拒绝。
              </Callout>
            ) : null}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={submitting}>
              {editing ? '保存配置' : '登记判题器'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** ParsedSidecars 是执行容器清单的解析结果。 */
interface ParsedSidecars {
  sidecars: WorkloadComponent[]
  error?: string
}

/**
 * parseSidecars 解析执行容器清单。
 * 后端要求至少一个容器;容器内部字段(镜像签名、资源限制)由后端最终判定,
 * 这里只挡住能在本地确定的错误。
 */
function parseSidecars(text: string): ParsedSidecars {
  const trimmed = text.trim()
  if (trimmed === '') return { sidecars: [], error: '请填写执行容器声明。' }

  let raw: unknown
  try {
    raw = JSON.parse(trimmed)
  } catch {
    return { sidecars: [], error: '内容不是合法的声明格式,检查是否漏了逗号或引号。' }
  }
  if (!Array.isArray(raw)) return { sidecars: [], error: '执行容器声明要是一个数组。' }
  if (raw.length === 0) return { sidecars: [], error: '至少要声明一个执行容器。' }
  for (const item of raw) {
    const record = typeof item === 'object' && item !== null ? (item as Record<string, unknown>) : undefined
    if (!record || typeof record.name !== 'string' || record.name.trim() === '') {
      return { sidecars: [], error: '每个执行容器都要有名字(name)。' }
    }
  }
  return { sidecars: raw as WorkloadComponent[] }
}
