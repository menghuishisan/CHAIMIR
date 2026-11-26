import { useEffect, useRef, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { mountTerminal, type MountedTerminal } from '@chaimir/ide'
import { Button } from '@chaimir/ui'
import { useTicketedWebSocket } from '../../../../hooks/useTicketedWebSocket'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import type { SandboxWorkspaceApi } from './types'

const encoder = new TextEncoder()

// 透传终端字节流，不在前端分行或解释命令。
export function TerminalPanel({
  sandboxId,
  workspaceApi,
}: {
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
}) {
  const host = useRef<HTMLDivElement>(null)
  const terminalRef = useRef<MountedTerminal | undefined>(undefined)
  const [ready, setReady] = useState(false)
  const [mountAttempt, setMountAttempt] = useState(0)
  const [mountError, setMountError] = useState<string>()

  const url = workspaceApi.getTerminalWsUrl(sandboxId)

  const socket = useTicketedWebSocket({
    url: ready ? url : undefined,
    onBinaryMessage: (data) => terminalRef.current?.write(data),
    onOpen: () => terminalRef.current?.focus(),
  })
  const send = socket.send

  // xterm 与本面板同生共死;输入转发经 socket.send,不在此持有任何凭据
  useEffect(() => {
    const container = host.current
    if (!container) return
    let disposed = false
    let mounted: MountedTerminal | undefined
    setReady(false)
    setMountError(undefined)

    void mountTerminal(container, {
      onData: (data) => send(encoder.encode(data).buffer),
      onBinary: (data) => send(data),
    })
      .then((terminal) => {
        if (disposed) {
          terminal.dispose()
          return
        }
        mounted = terminal
        terminalRef.current = terminal
        setReady(true)
      })
      .catch((error) => {
        if (!disposed) setMountError(userFacingErrorMessage(error, '终端没有加载成功,请重新加载。'))
      })

    return () => {
      disposed = true
      mounted?.dispose()
      terminalRef.current = undefined
      setReady(false)
    }
  }, [mountAttempt, sandboxId, send])

  useEffect(() => {
    const terminal = terminalRef.current
    if (!ready || socket.status !== 'open' || !terminal) return
    const sendSize = ({ cols, rows }: { cols: number; rows: number }) => {
      send(JSON.stringify({ Width: cols, Height: rows }))
    }
    sendSize(terminal.getSize())
    const subscription = terminal.onResize(sendSize)
    return () => subscription.dispose()
  }, [ready, socket.status, send])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 items-center justify-between gap-2 border-b border-dark-line px-3 py-2">
        <span className="text-xs text-on-dark-sub">{socketStatusText(socket.status)}</span>
        <div className="flex items-center gap-2">
          {mountError ? (
            <Button
              variant="on-dark"
              size="sm"
              leftIcon={RefreshCw}
              onClick={() => setMountAttempt((current) => current + 1)}
            >
              重新加载终端
            </Button>
          ) : null}
          <Button variant="on-dark" size="sm" onClick={() => terminalRef.current?.clear()}>
            清屏
          </Button>
          {socket.status === 'error' || socket.status === 'closed' ? (
            <Button variant="on-dark" size="sm" leftIcon={RefreshCw} onClick={socket.reconnect}>
              重新连接
            </Button>
          ) : null}
        </div>
      </div>
      {mountError || socket.error ? (
        <p className="shrink-0 px-3 py-2 text-xs text-on-dark-danger">
          {mountError ?? socket.error}
        </p>
      ) : null}
      <div ref={host} className="min-h-0 flex-1 bg-terminal" />
    </div>
  )
}

function socketStatusText(status: ReturnType<typeof useTicketedWebSocket>['status']): string {
  switch (status) {
    case 'connecting':
      return '正在连接终端…'
    case 'open':
      return '终端已连接'
    case 'closed':
      return '终端连接已断开'
    case 'error':
      return '终端连接出错'
    default:
      return '终端未连接'
  }
}
