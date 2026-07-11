// EditorTerminalPanes 装配共享 Monaco 与 xterm，并管理各自可释放生命周期。

import React, { useCallback, useEffect, useMemo, useRef } from 'react'
import type { MountedEditor, MountedTerminal } from '@chaimir/ide'
import { mountMonacoEditor, mountTerminal } from '@chaimir/ide'
import { api } from '../../../../app/api'
import { useTicketedWebSocket } from '../../../../hooks'
import type { WorkspaceFile } from './workspaceFiles'
import styles from './SandboxIdeWorkspace.module.css'

/** MonacoPane 装配共享 Monaco 封装并在文件切换时释放旧实例。 */
export function MonacoPane({ file, value, onChange }: { file: WorkspaceFile; value: string; onChange: (value: string) => void }): React.ReactElement {
  const hostRef = useRef<HTMLDivElement>(null)
  const editorRef = useRef<MountedEditor | null>(null)
  const onChangeRef = useRef(onChange)
  const initialValue = useRef(value).current
  onChangeRef.current = onChange

  useEffect(() => {
    let active = true
    if (!hostRef.current) return undefined
    void mountMonacoEditor(hostRef.current, {
      value: initialValue,
      language: file.language || 'plaintext',
      readOnly: file.readOnly,
      onChange: (next) => onChangeRef.current(next),
    }).then((editor) => {
      if (!active) editor.dispose()
      else editorRef.current = editor
    })
    return () => {
      active = false
      editorRef.current?.dispose()
      editorRef.current = null
    }
  }, [file.language, file.path, file.readOnly, initialValue])

  return <div className={styles.editor} ref={hostRef} />
}

/** SandboxTerminal 将 xterm 输入输出连接到后端短时票据终端通道。 */
export function SandboxTerminal({ sandboxId }: { sandboxId: string }): React.ReactElement {
  const hostRef = useRef<HTMLDivElement>(null)
  const terminalRef = useRef<MountedTerminal | null>(null)
  const url = useMemo(() => api.sandbox.getTerminalWsUrl(sandboxId), [sandboxId])
  const { status, send } = useTicketedWebSocket({
    url,
    binaryType: 'arraybuffer',
    onMessage: useCallback((event: MessageEvent) => {
      void websocketText(event.data).then((value) => terminalRef.current?.write(value))
    }, []),
  })

  useEffect(() => {
    let active = true
    if (!hostRef.current) return undefined
    void mountTerminal(hostRef.current, {
      initialText: '正在连接实验终端...\r\n',
      onData: (data) => send(data),
    }).then((terminal) => {
      if (!active) terminal.dispose()
      else terminalRef.current = terminal
    })
    return () => {
      active = false
      terminalRef.current?.dispose()
      terminalRef.current = null
    }
  }, [sandboxId, send])

  return <div className={styles.terminal} ref={hostRef} data-status={status} />
}

/** websocketText 统一读取终端文本帧和二进制帧。 */
async function websocketText(data: unknown): Promise<string> {
  if (typeof data === 'string') return data
  if (data instanceof Blob) return new TextDecoder().decode(await data.arrayBuffer())
  if (data instanceof ArrayBuffer) return new TextDecoder().decode(data)
  return ''
}
