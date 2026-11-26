import { useState } from 'react'
import { Play } from 'lucide-react'
import type { SandboxChainOperation } from '@chaimir/api-client'
import { Button, Input, Select, Textarea } from '@chaimir/ui'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { sandboxChainOperationLabel } from '../../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import type { SandboxWorkspaceApi } from './types'

interface ChainPanelProps {
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
  operations: Record<string, SandboxChainOperation[]>
}

// 部署与交易保留运行时定义的对象参数，查询只传目标标识。
export function ChainPanel({ sandboxId, workspaceApi, operations: operationsByInstance }: ChainPanelProps) {
  const runtimeInstances = Object.keys(operationsByInstance)
  const [selectedOperation, setOperation] = useState<SandboxChainOperation | undefined>(
    operationsByInstance[runtimeInstances[0] ?? '']?.[0]
  )
  const [selectedRuntimeInstance, setRuntimeInstance] = useState(runtimeInstances[0] ?? '')
  const runtimeInstance = runtimeInstances.includes(selectedRuntimeInstance)
    ? selectedRuntimeInstance
    : (runtimeInstances[0] ?? '')
  const operations = operationsByInstance[runtimeInstance] ?? []
  const operation =
    selectedOperation && operations.includes(selectedOperation) ? selectedOperation : operations[0]
  const [payloadText, setPayloadText] = useState('{\n  \n}')
  const [target, setTarget] = useState('')
  const [execution, setExecution] = useState<{
    operation: SandboxChainOperation
    runtimeInstance: string
    result?: string
    error?: string
  }>()
  const [busy, setBusy] = useState(false)
  const currentExecution = execution?.operation === operation && execution?.runtimeInstance === runtimeInstance
    ? execution : undefined

  const { errors, validate } = useFormValidation(() => {
    let payloadError: string | null = null
    if (operation && operation !== 'query') {
      try {
        const parsed: unknown = JSON.parse(payloadText)
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          payloadError = '内容格式不正确,最外层应当是大括号包裹的内容。'
        }
      } catch {
        payloadError = '内容格式不正确,请检查是否漏了逗号或引号。'
      }
    }
    return {
      target:
        operation === 'query' && !target.trim()
          ? '请填写要查询的目标,例如合约地址或状态键。'
          : null,
      runtimeInstance:
        operation === 'query' && !runtimeInstance ? '当前环境没有可用的运行时实例。' : null,
      payloadText: payloadError,
    }
  }, operation)

  async function run() {
    if (busy || !operation || !runtimeInstance || !validate()) return
    setBusy(true)
    setExecution(undefined)
    try {
      let response
      if (operation === 'query') {
        response = await workspaceApi.chainQuery(sandboxId, runtimeInstance, target.trim())
      } else {
        // 校验已确认参数为对象，接口继续接收运行时规定的原对象。
        const request = {
          runtime_instance: runtimeInstance,
          payload: JSON.parse(payloadText) as Record<string, unknown>,
        }
        response = operation === 'deploy'
          ? await workspaceApi.chainDeploy(sandboxId, request)
          : await workspaceApi.chainSendTx(sandboxId, request)
      }
      setExecution({ operation, runtimeInstance, result: JSON.stringify(response, null, 2) })
    } catch (error) {
      setExecution({
        operation, runtimeInstance,
        error: userFacingErrorMessage(error, '这次链操作没有成功,请检查参数后重试。'),
      })
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 overflow-y-auto p-3">
      <div className="flex flex-wrap items-center gap-2">
        <Select
          onDark
          aria-label="运行时实例"
          id={`${sandboxId}-chain-runtime`}
          options={runtimeInstances.map((item) => ({ value: item, label: item }))}
          value={runtimeInstance}
          placeholder="选择运行时实例"
          disabled={busy || runtimeInstances.length === 0}
          invalid={Boolean(errors.runtimeInstance)}
          aria-describedby={errors.runtimeInstance ? `${sandboxId}-chain-runtime-error` : undefined}
          onBlur={() => validate(['runtimeInstance'])}
          onValueChange={setRuntimeInstance}
        />
        {operations.map((item) => (
          <Button
            key={item}
            variant={item === operation ? 'primary' : 'on-dark'}
            size="sm"
            disabled={busy}
            onClick={() => {
              setOperation(item)
              setExecution(undefined)
            }}
          >
            {sandboxChainOperationLabel(item)}
          </Button>
        ))}
      </div>

      {errors.runtimeInstance ? (
        <p
          id={`${sandboxId}-chain-runtime-error`}
          role="alert"
          className="text-xs text-on-dark-danger"
        >
          {errors.runtimeInstance}
        </p>
      ) : null}

      {operation === 'query' ? (
        <label className="flex flex-col gap-1">
          <span className="text-xs text-on-dark-sub">查询目标</span>
          <Input
            variant="dark"
            value={target}
            disabled={busy}
            placeholder="合约地址或状态键"
            className="font-mono text-sm"
            invalid={Boolean(errors.target)}
            aria-describedby={errors.target ? `${sandboxId}-chain-input-error` : undefined}
            onChange={(event) => setTarget(event.target.value)}
            onBlur={() => validate(['target'])}
          />
        </label>
      ) : (
        <label className="flex min-h-0 flex-col gap-1">
          <span className="text-xs text-on-dark-sub">
            {operation === 'deploy' ? '部署参数' : '交易参数'}
          </span>
          <Textarea
            onDark
            value={payloadText}
            disabled={busy}
            rows={8}
            spellCheck={false}
            className="font-mono text-sm"
            invalid={Boolean(errors.payloadText)}
            aria-describedby={errors.payloadText ? `${sandboxId}-chain-input-error` : undefined}
            onChange={(event) => setPayloadText(event.target.value)}
            onBlur={() => validate(['payloadText'])}
          />
        </label>
      )}

      {errors.target || errors.payloadText ? (
        <p
          id={`${sandboxId}-chain-input-error`}
          role="alert"
          className="text-xs text-on-dark-danger"
        >
          {errors.target ?? errors.payloadText}
        </p>
      ) : null}

      <div>
        <Button
          variant="primary"
          size="sm"
          leftIcon={Play}
          loading={busy}
          disabled={!operation || !runtimeInstance}
          onClick={() => void run()}
        >
          执行
        </Button>
      </div>

      {currentExecution?.error ? <p role="alert" className="text-xs text-on-dark-danger">{currentExecution.error}</p> : null}

      {currentExecution?.result ? (
        <pre className="min-h-0 overflow-auto rounded-md border border-dark-line bg-terminal p-3 font-mono text-xs text-on-dark">
          {currentExecution.result}
        </pre>
      ) : null}
    </div>
  )
}
