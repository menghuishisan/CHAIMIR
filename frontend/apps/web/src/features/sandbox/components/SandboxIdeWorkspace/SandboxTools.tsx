// SandboxTools 映射沙箱工具状态，并提供命令与统一链操作面板。

import React, { useState } from 'react'
import type { IdeWorkbenchTool } from '@chaimir/ide'
import type { SandboxInstance } from '@chaimir/api-client'
import { SandboxToolKind, SandboxToolStatus } from '@chaimir/api-client'
import { Button, CodeBlock, Select, Textarea } from '@chaimir/ui'
import { ExternalLink, Play } from 'lucide-react'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { decodeBase64 } from './workspaceFiles'
import styles from './SandboxIdeWorkspace.module.css'

/** SandboxOperations 执行受控命令工具，并展示后端声明的链能力。 */
export function SandboxOperations({ sandboxId, instance }: { sandboxId: string; instance: SandboxInstance }): React.ReactElement {
  const commandTools = instance.tool_access.filter((tool) => tool.kind === SandboxToolKind.COMMAND)
  const chainOperations = instance.capabilities.chain_operations
  const [toolCode, setToolCode] = useState(commandTools[0]?.tool_code || '')
  const [input, setInput] = useState('')
  const [result, setResult] = useState('')
  const [error, setError] = useState<string>()
  const [running, setRunning] = useState(false)

  if ((!instance.capabilities.command_tools || commandTools.length === 0) && chainOperations.length === 0) {
    return <section className={styles.operations} aria-label="沙箱工具"><h2>沙箱工具</h2><p>当前实验环境没有可执行的附加操作。</p></section>
  }

  /** runOperation 校验输入后调用命令或链能力，并把结构化结果展示给用户。 */
  const runOperation = async () => {
    setError(undefined)
    setResult('')
    setRunning(true)
    try {
      const command = input.trim().split(/\s+/).filter(Boolean)
      if (!toolCode || command.length === 0) throw new Error('请选择命令工具并填写要执行的命令。')
      const response = await api.sandbox.runCommandTool(sandboxId, toolCode, { command })
      const output = `${decodeBase64(response.stdout_base64)}${decodeBase64(response.stderr_base64)}`.trim()
      setResult(output || `命令已完成，退出状态为 ${response.exit_code}。`)
    } catch (operationError) {
      setError(userFacingErrorMessage(operationError, '操作未完成，请检查输入后重试。'))
    } finally {
      setRunning(false)
    }
  }

  return (
    <section className={styles.operations} aria-label="沙箱工具">
      <h2>沙箱工具</h2>
      {commandTools.length > 0 && instance.capabilities.command_tools && <>
        <Select value={toolCode} onChange={setToolCode} options={commandTools.map((tool) => ({ value: tool.tool_code, label: tool.tool_code }))} placeholder="选择命令工具" />
        <Textarea value={input} onChange={(event) => setInput(event.target.value)} placeholder="输入当前工具允许执行的命令" rows={4} fullWidth />
        <Button size="sm" icon={<Play size={14} />} loading={running} onClick={() => void runOperation()}>执行</Button>
      </>}
      {chainOperations.length > 0 && <p>链操作能力：{chainOperations.join('、')}。具体参数由实验流程提供。</p>}
      {error && <p className={styles.error} role="alert">{error}</p>}
      {result && <CodeBlock code={result} ariaLabel="命令输出" language="终端输出" />}
    </section>
  )
}

/** toWorkbenchTools 把沙箱工具能力映射为共享 IDE 工具状态和入口。 */
export function toWorkbenchTools(instance: SandboxInstance): IdeWorkbenchTool[] {
  return instance.tool_access.map((tool) => ({
    id: tool.tool_code,
    label: tool.tool_code,
    kind: toolKind(tool.kind),
    status: tool.status === SandboxToolStatus.READY ? 'ready' : tool.status === SandboxToolStatus.FAILED ? 'failed' : 'running',
    action: tool.kind === SandboxToolKind.WEB_EMBED ? <Button variant="ghost" size="sm" icon={<ExternalLink size={14} />} onClick={() => window.open(api.sandbox.getToolProxyUrl(String(instance.sandbox_id), tool.tool_code), '_blank', 'noopener,noreferrer')}>打开</Button> : undefined,
  }))
}

/** toolKind 对齐共享 IDE 支持的工具类别。 */
function toolKind(kind: SandboxToolKind): IdeWorkbenchTool['kind'] {
  if (kind === SandboxToolKind.TERMINAL) return 'terminal'
  if (kind === SandboxToolKind.WEB_EMBED) return 'web-embed'
  if (kind === SandboxToolKind.COMMAND) return 'command-tool'
  return 'platform-builtin'
}
