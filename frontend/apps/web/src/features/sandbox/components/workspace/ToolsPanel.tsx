import { useEffect, useState } from 'react'
import { Play, RefreshCw } from 'lucide-react'
import { SandboxToolStatus, type SandboxToolAccess } from '@chaimir/api-client'
import { Button, FormField, Textarea } from '@chaimir/ui'
import { appConfig } from '../../../../app/config'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { decodeUtf8Base64, encodeUtf8Base64 } from '../../../../utils/base64'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { validCommand } from '../../dynamicSpec'
import { isSandboxReadyForOperation } from '../../rules'
import { CommandArgvFields } from '../CommandArgvFields'
import type { SandboxWorkspaceApi } from './types'

interface CommandPanelProps {
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
  tools: SandboxToolAccess[]
  available: boolean
  onReload: () => void
}

// 服务端按完整参数数组强制白名单，页面不复制平台侧策略。
export function CommandPanel({
  sandboxId,
  workspaceApi,
  tools,
  available,
  onReload,
}: CommandPanelProps) {
  const [selectedToolCode, setToolCode] = useState(tools[0]?.tool_code ?? '')
  const [command, setCommand] = useState([''])
  const [stdin, setStdin] = useState('')
  const [execution, setExecution] = useState<{
    toolCode: string
    output?: string
    error?: string
  }>()
  const [busy, setBusy] = useState(false)
  const selectedTool = tools.find((tool) => tool.tool_code === selectedToolCode) ?? tools[0]
  const toolCode = selectedTool?.tool_code ?? ''
  const toolReady = available && selectedTool?.status === SandboxToolStatus.READY
  const currentExecution = execution?.toolCode === toolCode ? execution : undefined

  const { errors, validate } = useFormValidation(() => ({
    command: !command[0]?.trim()
      ? '请输入要执行的命令。'
      : validCommand(command)
        ? null
        : '请填写完整参数，并移除不需要的空项。',
  }))

  async function run() {
    if (busy || !toolReady) return
    if (!validate()) return
    setBusy(true)
    setExecution(undefined)
    try {
      const response = await workspaceApi.runCommandTool(sandboxId, toolCode, {
        command,
        ...(stdin === '' ? {} : { stdin_base64: encodeUtf8Base64(stdin) }),
      })
      const stdout = decodeUtf8Base64(response.stdout_base64)
      const stderr = decodeUtf8Base64(response.stderr_base64)
      setExecution({
        toolCode,
        output: [stdout, stderr ? `\n${stderr}` : '', `\n[结束状态 ${response.exit_code}]`].join(
          ''
        ),
      })
    } catch (error) {
      setExecution({
        toolCode,
        error: userFacingErrorMessage(error, '命令执行或结果读取没有成功，请检查参数后重试。'),
      })
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 overflow-y-auto p-3">
      {tools.length > 1 ? (
        <div className="flex flex-wrap items-center gap-2">
          {tools.map((tool) => (
            <Button
              key={tool.tool_code}
              variant={tool.tool_code === toolCode ? 'primary' : 'on-dark'}
              size="sm"
              disabled={busy}
              onClick={() => setToolCode(tool.tool_code)}
            >
              {tool.tool_code}
            </Button>
          ))}
        </div>
      ) : null}

      {!toolReady ? (
        <ToolReadinessNotice
          status={available ? selectedTool?.status : undefined}
          onReload={onReload}
        />
      ) : null}

      <CommandArgvFields
        label="命令与参数"
        value={command}
        onChange={setCommand}
        onBlur={() => validate(['command'])}
        helper="先填写命令，再逐个添加参数；参数中的空格和引号会原样保留。"
        error={errors.command}
        required
        onDark
        disabled={!toolReady || busy}
      />

      <FormField
        label="输入内容"
        helper="需要输入内容的命令可在这里填写；空格和换行会原样传入。"
        onDark
      >
        <Textarea
          value={stdin}
          onChange={(event) => setStdin(event.target.value)}
          disabled={!toolReady || busy}
          onDark
        />
      </FormField>

      <div>
        <Button
          variant="primary"
          size="sm"
          leftIcon={Play}
          loading={busy}
          disabled={!toolReady}
          onClick={() => void run()}
        >
          执行
        </Button>
      </div>

      {currentExecution?.error ? (
        <p role="alert" className="text-xs text-on-dark-danger">
          {currentExecution.error}
        </p>
      ) : null}

      {currentExecution?.output ? (
        <pre className="min-h-0 overflow-auto whitespace-pre-wrap rounded-md border border-dark-line bg-terminal p-3 font-mono text-xs text-on-dark">
          {currentExecution.output}
        </pre>
      ) : null}
    </div>
  )
}

// 工具只经平台代理访问，凭证限制在当前工具路径。
export function WebToolPanel({
  sandboxId,
  workspaceApi,
  toolCode,
  status,
  onReload,
}: {
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
  toolCode: string
  status: SandboxToolStatus | undefined
  onReload: () => void
}) {
  const [loadAttempt, setLoadAttempt] = useState(0)

  const [loadState, setLoadState] = useState<{ key: string; src?: string; error?: string }>({
    key: '',
    src: undefined,
  })
  const loadKey = `${sandboxId}:${toolCode}:${status ?? ''}:${loadAttempt}`
  useEffect(() => {
    let active = true
    if (status !== SandboxToolStatus.READY) return
    // 首次打开和主动重开都重读权威状态,旧标签中的Ready不能直接换票。
    void (async () => {
      const current = await workspaceApi.getInstance(sandboxId)
      if (!active) return
      const tool = current.tool_access.find((item) => item.tool_code === toolCode)
      if (!isSandboxReadyForOperation(current) || tool?.status !== SandboxToolStatus.READY) {
        setLoadState({ key: loadKey, error: '工具尚未准备好,请稍后重新打开。' })
        onReload()
        return
      }
      const url = await workspaceApi.getToolProxyUrl(
        sandboxId,
        toolCode,
        '',
        appConfig.sandboxToolOrigin
      )
      if (active) setLoadState({ key: loadKey, src: url })
    })().catch((error) => {
      if (active)
        setLoadState({
          key: loadKey,
          error: userFacingErrorMessage(error, '这个工具暂时打不开,请稍后重试。'),
        })
    })
    return () => {
      active = false
    }
  }, [loadAttempt, loadKey, onReload, sandboxId, status, toolCode, workspaceApi])

  if (status !== SandboxToolStatus.READY) {
    return (
      <div className="grid h-full place-items-center px-4">
        <ToolReadinessNotice status={status} onReload={onReload} />
      </div>
    )
  }

  const loadError = loadState.key === loadKey ? loadState.error : undefined
  const src = loadState.key === loadKey ? loadState.src : undefined

  if (loadError) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 px-4 text-center">
        <p role="alert" className="text-sm text-on-dark-danger">
          {loadError}
        </p>
        <Button
          variant="on-dark"
          size="sm"
          leftIcon={RefreshCw}
          onClick={() => setLoadAttempt((current) => current + 1)}
        >
          重新打开工具
        </Button>
      </div>
    )
  }
  if (!src) {
    return (
      <div className="grid h-full place-items-center px-4 text-sm text-on-dark-sub">
        工具正在准备...
      </div>
    )
  }
  return (
    <iframe
      src={src}
      title={`${toolCode} 工具`}
      sandbox="allow-scripts allow-same-origin allow-forms allow-downloads allow-modals"
      referrerPolicy="no-referrer"
      className="h-full w-full border-0 bg-dark-surface"
    />
  )
}

function ToolReadinessNotice({
  status,
  onReload,
}: {
  status: SandboxToolStatus | undefined
  onReload: () => void
}) {
  if (status === SandboxToolStatus.STARTING) {
    return (
      <p role="status" className="text-sm text-on-dark-sub">
        工具正在准备,请稍候。
      </p>
    )
  }
  return (
    <div className="flex flex-col items-start gap-3">
      <p role="alert" className="text-sm text-on-dark-danger">
        {status === SandboxToolStatus.FAILED
          ? '工具没有准备成功,请重新读取状态。'
          : '当前工具暂时无法使用,请重新读取状态。'}
      </p>
      <Button variant="on-dark" size="sm" leftIcon={RefreshCw} onClick={onReload}>
        重新读取状态
      </Button>
    </div>
  )
}
