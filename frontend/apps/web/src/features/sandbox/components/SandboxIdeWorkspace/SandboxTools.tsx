// SandboxTools 映射沙箱工具状态，并提供命令与统一链操作面板。

import React, { useState } from 'react'
import type { IdeWorkbenchTool } from '@chaimir/ide'
import type { SandboxInstance } from '@chaimir/api-client'
import { SandboxToolKind, SandboxToolStatus } from '@chaimir/api-client'
import { Button, Input, Select, Textarea } from '@chaimir/ui'
import { ExternalLink, Play } from 'lucide-react'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { decodeBase64 } from './workspaceFiles'
import styles from './SandboxIdeWorkspace.module.css'

/** SandboxOperations 提供后端声明的命令工具和统一链操作入口。 */
export function SandboxOperations({ sandboxId, instance }: { sandboxId: string; instance: SandboxInstance }): React.ReactElement {
  const commandTools = instance.tool_access.filter((tool) => tool.kind === SandboxToolKind.COMMAND)
  const [mode, setMode] = useState('command')
  const [toolCode, setToolCode] = useState(commandTools[0]?.tool_code || '')
  const [input, setInput] = useState('')
  const [result, setResult] = useState('')
  const [error, setError] = useState<string>()
  const [running, setRunning] = useState(false)

  /** runOperation 校验输入后调用命令或链能力，并把结构化结果展示给用户。 */
  const runOperation = async () => {
    setError(undefined)
    setResult('')
    setRunning(true)
    try {
      if (mode === 'command') {
        const command = input.trim().split(/\s+/).filter(Boolean)
        if (!toolCode || command.length === 0) throw new Error('请选择命令工具并填写要执行的命令。')
        const response = await api.sandbox.runCommandTool(sandboxId, toolCode, { command })
        const output = `${decodeBase64(response.stdout_base64)}${decodeBase64(response.stderr_base64)}`.trim()
        setResult(output || `命令已完成，退出状态为 ${response.exit_code}。`)
      } else if (mode === 'query') {
        if (!input.trim()) throw new Error('请填写要查询的链上目标。')
        setResult(JSON.stringify(await api.sandbox.chainQuery(sandboxId, input.trim()), null, 2))
      } else {
        const payload = parseObject(input)
        const response = mode === 'deploy'
          ? await api.sandbox.chainDeploy(sandboxId, { payload })
          : await api.sandbox.chainSendTx(sandboxId, { payload })
        setResult(JSON.stringify(response, null, 2))
      }
    } catch (operationError) {
      setError(userFacingErrorMessage(operationError, '操作未完成，请检查输入后重试。'))
    } finally {
      setRunning(false)
    }
  }

  return (
    <section className={styles.operations} aria-label="沙箱工具">
      <h2>沙箱工具</h2>
      <Select value={mode} onChange={setMode} options={[
        { value: 'command', label: '运行命令工具' },
        { value: 'deploy', label: '部署到实验链' },
        { value: 'transaction', label: '发送实验链交易' },
        { value: 'query', label: '查询实验链状态' },
      ]} />
      {mode === 'command' && <Select value={toolCode} onChange={setToolCode} options={commandTools.map((tool) => ({ value: tool.tool_code, label: tool.tool_code }))} placeholder="选择命令工具" />}
      {mode === 'query' ? <Input value={input} onChange={(event) => setInput(event.target.value)} placeholder="查询目标" fullWidth /> : <Textarea value={input} onChange={(event) => setInput(event.target.value)} placeholder={mode === 'command' ? '输入受控命令' : '输入 JSON 参数'} rows={4} fullWidth />}
      <Button size="sm" icon={<Play size={14} />} loading={running} onClick={() => void runOperation()}>执行</Button>
      {error && <p className={styles.error} role="alert">{error}</p>}
      {result && <pre className={styles.output}>{result}</pre>}
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

/** parseObject 校验链操作参数必须是 JSON 对象。 */
function parseObject(value: string): Record<string, unknown> {
  const parsed: unknown = JSON.parse(value || '{}')
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('链操作参数必须是 JSON 对象。')
  return parsed as Record<string, unknown>
}
