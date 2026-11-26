import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../app/api'
import { errorDiagnostics, userFacingErrorMessage } from '../utils/userFacingError'

export type SocketStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error'

export interface TicketedSocketState {
  status: SocketStatus
  /** 用户向失败文案;技术原因进控制台,不上界面 */
  error?: string
  /** 向服务端发送文本或原始字节;未连接时返回 false 并给出用户向状态 */
  send: (data: string | ArrayBuffer) => boolean
  /** 重新换票并建连 */
  reconnect: () => void
}

export interface TicketedSocketOptions {
  /** 目标 WS 地址;传 undefined 表示当前不需要连接(如运行时未声明该能力) */
  url: string | undefined
  /** 收到一段服务端文本消息 */
  onMessage?: (data: string) => void
  /** 按到达顺序收到原始二进制消息,由消费者按其流协议处理 */
  onBinaryMessage?: (data: Uint8Array) => void
  /** 连接就绪 */
  onOpen?: () => void
}

/** 使用绑定到目标路径的短时票据建连。重连时重新签发，卸载时关闭。 */
export function useTicketedWebSocket(options: TicketedSocketOptions): TicketedSocketState {
  const { url, onMessage, onBinaryMessage, onOpen } = options
  const [status, setStatus] = useState<SocketStatus>('idle')
  const [error, setError] = useState<string>()
  const [attempt, setAttempt] = useState(0)
  const socketRef = useRef<WebSocket | undefined>(undefined)

  // 回调更新不应重连，也不应让连接读到尚未提交的渲染。
  const handlersRef = useRef({ onMessage, onBinaryMessage, onOpen })
  useEffect(() => {
    handlersRef.current = { onMessage, onBinaryMessage, onOpen }
  }, [onMessage, onBinaryMessage, onOpen])

  useEffect(() => {
    if (!url) return

    let active = true

    const connect = async () => {
      setError(undefined)
      setStatus('connecting')
      const address = new URL(url, window.location.href)
      const ticket = await api.webSocketTicketProvider(address.href)
      if (!active) return
      if (!ticket) throw new Error('WebSocket 票据为空')

      address.searchParams.set('ticket', ticket)
      const socket = new WebSocket(address)
      socket.binaryType = 'arraybuffer'
      socketRef.current = socket

      socket.onopen = () => {
        if (!active) return
        setStatus('open')
        handlersRef.current.onOpen?.()
      }
      socket.onmessage = (event: MessageEvent<unknown>) => {
        if (!active) return
        if (typeof event.data === 'string') {
          handlersRef.current.onMessage?.(event.data)
          return
        }
        if (event.data instanceof ArrayBuffer) {
          handlersRef.current.onBinaryMessage?.(new Uint8Array(event.data))
        }
      }
      socket.onerror = () => {
        if (!active) return
        console.error('[ws] 连接出错')
        setError('实时连接中断。可以重试连接。')
        setStatus('error')
      }
      socket.onclose = () => {
        if (!active) return
        if (socketRef.current === socket) socketRef.current = undefined
        setStatus((current) => (current === 'error' ? current : 'closed'))
      }
    }

    void connect().catch((cause: unknown) => {
      if (!active) return
      console.error('[ws] 建立连接失败', errorDiagnostics(cause))
      setError(userFacingErrorMessage(cause, '实时连接未能建立，请稍后重试。'))
      setStatus('error')
    })

    return () => {
      active = false
      const socket = socketRef.current
      socketRef.current = undefined
      if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
        socket.close()
      }
    }
  }, [attempt, url])

  // 输入订阅持有稳定函数；连接启停只替换 socket，不重建终端。
  const send = useCallback((data: string | ArrayBuffer) => {
    const socket = socketRef.current
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(data)
      return true
    }
    setError('实时连接尚未就绪,请稍后重试。')
    return false
  }, [])

  const reconnect = useCallback(() => setAttempt((current) => current + 1), [])

  return useMemo(
    () => ({
      status: url ? status : 'idle',
      error: url ? error : undefined,
      send,
      reconnect,
    }),
    [error, reconnect, send, status, url],
  )
}
