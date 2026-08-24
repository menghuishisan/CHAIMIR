// 判题器配置表单(判题器页内)。
//
// 后端把判题器拆成三部分:声明式判题环境组合、受控执行策略、自检样例。
//   - 组合(composition):主运行时 + 镜像版本 + 组件,访问边界固定为判题私有环境;
//     服务端编译后冻结成快照,前端只提交声明,不提交也不回填镜像地址、digest 与工作负载。
//   - 执行策略(resource_spec):初始链状态、判题命令、执行目标、超时与重试。
//   - 自检样例:随判题器镜像由平台目录同步提供,表单不改动。
//
// 需要私有执行容器的类型(测试用例 / 静态扫描)其容器声明来自判题器镜像 manifest,
// 由平台目录同步写入 —— 表单不提供镜像地址与启动命令的编辑入口
// (docs/对齐-后端待补齐清单-2026-08-23.md §7.5 / §8.3)。
//
// 已冻结的组合快照只作为事实展示,不回填成编辑输入:修改时要重新声明一遍判题环境。

import { useCallback, useId, useMemo, useState } from 'react'
import {
  JudgerStatus,
  JudgerType,
  SANDBOX_ACCESS_PROFILE,
  type Judger,
  type JudgerExecutionSpec,
  type JudgerRequest,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Checkbox,
  DescriptionList,
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
import { CompositionDeclarationFields } from '../../../sandbox/components/CompositionDeclarationFields'
import {
  compositionDeclarationError,
  compositionSpecFromDeclaration,
  emptyCompositionDeclaration,
} from '../../../sandbox/composition'
import { judgerStatusLabel, judgerTypeLabel } from '../../../../utils/labels/judge'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { JUDGER_STATUSES, JUDGER_TYPES } from '../../options'

/** 判题器编码规则,与后端 codePattern 一致。 */
const CODE_PATTERN = /^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/

/** 执行目标规则:pod/container 两段,各自都是受控编码(后端 safeExecTarget)。 */
const EXEC_TARGET_PATTERN = /^[a-z][a-z0-9-]*[a-z0-9]\/[a-z][a-z0-9-]*[a-z0-9]$/

/** 必须绑定判题环境的类型:后端对这三类强制要求 judge-private 组合。 */
const RUNTIME_BOUND_TYPES: readonly JudgerType[] = [
  JudgerType.TESTCASE,
  JudgerType.ONCHAIN_ASSERT,
  JudgerType.STATIC_SCAN,
]

/** 必须给出执行入口的类型:命令、执行目标与私有执行容器三者都不能缺。 */
const COMMAND_BOUND_TYPES: readonly JudgerType[] = [JudgerType.TESTCASE, JudgerType.STATIC_SCAN]

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

  // 判题环境每次都要重新声明:已冻结快照是服务端事实,不作为输入回填(§8.3)
  const [declaration, setDeclaration] = useState(emptyCompositionDeclaration)
  const [genesisRef, setGenesisRef] = useState(spec?.genesis_ref ?? '')
  const [initScriptRef, setInitScriptRef] = useState(spec?.init_script_ref ?? '')
  const [command, setCommand] = useState((spec?.command ?? []).join(' '))
  const [execTarget, setExecTarget] = useState(spec?.exec_target ?? '')
  const [suiteArchiveName, setSuiteArchiveName] = useState(spec?.suite_archive_name ?? '')
  const [timeoutSec, setTimeoutSec] = useState(String(spec?.timeout_sec ?? 0))
  const [maxRetries, setMaxRetries] = useState(String(spec?.max_retries ?? 0))

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const typeValue = Number(type) as JudgerType
  const needsRuntime = runtimeRequired || RUNTIME_BOUND_TYPES.includes(typeValue)
  const needsCommand = COMMAND_BOUND_TYPES.includes(typeValue)
  const isManual = typeValue === JudgerType.MANUAL

  // 私有执行容器由判题器镜像 manifest 提供,表单只带回不编辑
  const sidecars = useMemo(() => spec?.execution_sidecars ?? [], [spec?.execution_sidecars])
  const snapshot = spec?.composition_snapshot

  const commandList = useMemo(
    () =>
      command
        .split(/\s+/)
        .map((item) => item.trim())
        .filter((item) => item !== ''),
    [command],
  )

  /** validate 按类型逐项校验,口径与后端 parseJudgerExecutionSpec 一致。 */
  const validate = useCallback((): boolean => {
    const timeoutValue = Number(timeoutSec)
    const retriesValue = Number(maxRetries)
    const defaultTimeoutValue = Number(defaultTimeout)

    const next: Record<string, string | null> = {
      code: CODE_PATTERN.test(code.trim())
        ? null
        : '以小写字母或数字开头,只含小写字母、数字与连字符,长度 3 到 64 位',
      name: name.trim() === '' ? '请输入判题器名称' : null,
      executorRef: executorRef.trim() === '' ? '请填写判题实现名称' : null,
      defaultTimeout:
        !Number.isInteger(defaultTimeoutValue) || defaultTimeoutValue <= 0
          ? '默认超时要是大于 0 的整数秒'
          : null,
      timeoutSec:
        !Number.isInteger(timeoutValue) || timeoutValue < 0 ? '覆盖超时要是 0 或更大的整数秒' : null,
      maxRetries:
        !Number.isInteger(retriesValue) || retriesValue < 0 ? '重试次数要是 0 或更大的整数' : null,
      composition: null,
      genesisRef: null,
      command: null,
      execTarget: null,
      sidecars: null,
    }

    if (needsRuntime) {
      next.composition = compositionDeclarationError(declaration) ?? null
      next.genesisRef = genesisRef.trim() === '' ? '请填写初始链状态名称' : null
    }

    if (needsCommand) {
      next.command = commandList.length === 0 ? '请填写判题要执行的命令' : null
      next.execTarget = EXEC_TARGET_PATTERN.test(execTarget.trim())
        ? null
        : '执行目标写成「组/名称」两段,例如 sandbox/testcase-evm'
      next.sidecars =
        sidecars.length === 0
          ? '这类判题器需要判题器镜像提供私有执行容器。请先在平台镜像目录同步该判题器镜像,再回来配置。'
          : null
    }

    if (isManual && commandList.length > 0) {
      next.command = '人工评分不执行命令,请清空这一栏'
    }

    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [
    code,
    commandList,
    declaration,
    defaultTimeout,
    execTarget,
    executorRef,
    genesisRef,
    isManual,
    maxRetries,
    name,
    needsCommand,
    needsRuntime,
    sidecars.length,
    timeoutSec,
  ])

  /** buildExecutionSpec 只带该类型真正用到的执行策略字段,空值一律不写进声明。 */
  const buildExecutionSpec = useCallback((): JudgerExecutionSpec => {
    const out: JudgerExecutionSpec = {}
    if (needsRuntime) out.genesis_ref = genesisRef.trim()
    if (initScriptRef.trim() !== '') out.init_script_ref = initScriptRef.trim()
    if (needsCommand) {
      out.command = commandList
      out.exec_target = execTarget.trim()
      // 私有执行容器原样带回:镜像地址与安全上下文由平台镜像目录提供,表单不改写
      out.execution_sidecars = sidecars
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
    initScriptRef,
    maxRetries,
    needsCommand,
    needsRuntime,
    sidecars,
    spec?.selftest,
    suiteArchiveName,
    timeoutSec,
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
          // 不需要沙箱的判题方式提交空组合:后端要求这几项一律留空
          composition: needsRuntime
            ? compositionSpecFromDeclaration({
                id: `judge:${code.trim()}`,
                declaration,
                accessProfile: SANDBOX_ACCESS_PROFILE.JUDGE_PRIVATE,
              })
            : emptyComposition(),
          resource_spec: buildExecutionSpec(),
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
      buildExecutionSpec,
      code,
      declaration,
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
                label="判题器短名"
                htmlFor={`${fieldId}-code`}
                required
                error={errors.code}
                helper={editing ? '登记后不能修改' : '教师出题时按这个短名引用'}
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
                label="判题实现名称"
                htmlFor={`${fieldId}-executor`}
                required
                error={errors.executorRef}
                helper="平台会按这个名称找到对应的判题实现"
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
                这种判题方式必须在判题私有环境里跑,下面的判题环境是必填的。
              </Callout>
            ) : (
              <Checkbox
                checked={runtimeRequired}
                label="判题时需要起一个链环境"
                onCheckedChange={(checked) => setRuntimeRequired(checked === true)}
              />
            )}

            {needsRuntime ? (
              <div className="flex flex-col gap-4 well p-4">
                <div>
                  <p className="text-base text-ink">判题环境</p>
                  <p className="text-sm text-ink-sub">
                    判题时会按这份声明起一个学生不可见的一次性沙箱,判完即销毁。
                    {editing ? '修改配置要重新声明一遍 —— 已冻结的执行事实不作为输入回填。' : ''}
                  </p>
                </div>

                {errors.composition ? (
                  <Callout tone="danger">{errors.composition}</Callout>
                ) : null}

                <CompositionDeclarationFields
                  idPrefix={`${fieldId}-composition`}
                  value={declaration}
                  onChange={setDeclaration}
                  toolsHelper="判题执行时需要用到的工具,不需要就都不选"
                />

                <FormField
                  label="初始链状态名称"
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

            {snapshot ? (
              <div className="flex flex-col gap-2 well p-4">
                <div>
                  <p className="text-base text-ink">当前已冻结的判题环境(只读)</p>
                  <p className="text-sm text-ink-sub">
                    这是服务端编译后冻结的事实,判题任务实际使用它。它不会回填成上面的编辑项。
                  </p>
                </div>
                <DescriptionList
                  dense
                  columns={2}
                  items={[
                    { term: '运行时', description: snapshot.runtime.code, mono: true },
                    { term: '镜像版本', description: snapshot.runtime.image_version, mono: true },
                    {
                      term: '组件数',
                      description: `${snapshot.components?.length ?? 0} 个`,
                    },
                    {
                      term: '锁定镜像数',
                      description: `${snapshot.image_closure.length} 个`,
                    },
                  ]}
                />
              </div>
            ) : null}

            {needsCommand ? (
              <div className="flex flex-col gap-4 well p-4">
                <div>
                  <p className="text-base text-ink">执行入口</p>
                  <p className="text-sm text-ink-sub">
                    判题在判题器镜像提供的私有容器里执行指定命令。命令按空格拆成参数,不经过 shell。
                  </p>
                </div>

                {errors.sidecars ? <Callout tone="warning">{errors.sidecars}</Callout> : null}

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
                    placeholder="run-evm-tests"
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
                    helper="写成「组/名称」两段,指向判题器镜像声明的私有容器"
                  >
                    <Input
                      id={`${fieldId}-exec-target`}
                      className="font-mono text-sm"
                      value={execTarget}
                      placeholder="sandbox/testcase-evm"
                      invalid={Boolean(errors.execTarget)}
                      onChange={(event) => setExecTarget(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="测试集压缩包名"
                    htmlFor={`${fieldId}-suite`}
                    helper="题目附带的测试集文件名。留空表示不需要"
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

                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="text-sm text-ink-sub">私有执行容器:</span>
                  {sidecars.length === 0 ? (
                    <span className="text-sm text-ink-sub">
                      还没有。它由判题器镜像随平台镜像目录同步提供,不在这里编辑。
                    </span>
                  ) : (
                    sidecars.map((item, index) => (
                      <Badge key={index} tone="neutral">
                        {sidecarName(item) || `容器 ${index + 1}`}
                      </Badge>
                    ))
                  )}
                </div>
              </div>
            ) : null}

            <FormField
              label="初始化脚本名称"
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
            <Button type="submit" variant="primary" loading={submitting}>
              {editing ? '保存配置' : '登记判题器'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** emptyComposition 给出「不需要沙箱」时必须提交的空组合:后端要求这三项一律留空。 */
function emptyComposition(): JudgerRequest['composition'] {
  return {
    id: '',
    primary_runtime: { runtime_code: '', image_version: '' },
    access_profile: SANDBOX_ACCESS_PROFILE.JUDGE_PRIVATE,
  }
}

/** sidecarName 读出私有执行容器的名称,用于只读展示。 */
function sidecarName(item: unknown): string {
  if (typeof item !== 'object' || item === null) return ''
  const name = (item as Record<string, unknown>).name
  return typeof name === 'string' ? name : ''
}
