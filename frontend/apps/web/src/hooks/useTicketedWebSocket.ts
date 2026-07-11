// useTicketedWebSocket 管理短时票据 WebSocket 的连接、订阅、重连和清理。

import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../app/api'

export type WebSocketConnectionStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed' | 'error'

export interface TicketedWebSocketOptions {
  url: string | null
  enabled?: boolean
  subscribeMessage?: unknown
  binaryType?: BinaryType
  onMessage?: (event: MessageEvent) => void
}

export interface TicketedWebSocketState {
  status: WebSocketConnectionStatus
  reconnect: () => void
  send: (data: string | ArrayBufferLike | Blob | ArrayBufferView) => boolean
}

const MAX_RECONNECT_DELAY_MS = 10_000

/** appendTicket 把短时票据添加到 WebSocket URL，不携带 access token。 */
function appendTicket(url: string, ticket: string): string {
  const target = new URL(url, window.location.href)
  target.searchParams.set('ticket', ticket)
  return target.toString()
}

/**
 * useTicketedWebSocket 通过后端签发的 path-bound ticket 建连，并在非主动关闭时退避重连。
 */
export function useTicketedWebSocket({
  url,
  enabled = true,
  subscribeMessage,
  binaryType = 'blob',
  onMessage,
}: TicketedWebSocketOptions): TicketedWebSocketState {
  const [status, setStatus] = useState<WebSocketConnectionStatus>('idle')
  const [version, setVersion] = useState(0)
  const messageHandlerRef = useRef(onMessage)
  const reconnectAttemptRef = useRef(0)
  const socketRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    messageHandlerRef.current = onMessage
  }, [onMessage])

  const reconnect = useCallback(() => {
    reconnectAttemptRef.current = 0
    setVersion((current) => current + 1)
  }, [])

  /** send 仅在连接可写时发送消息，由业务层决定文本或二进制协议。 */
  const send = useCallback((data: string | ArrayBufferLike | Blob | ArrayBufferView): boolean => {
    if (socketRef.current?.readyState !== WebSocket.OPEN) return false
    socketRef.current.send(data)
    return true
  }, [])

  useEffect(() => {
    if (!enabled || !url) {
      setStatus('idle')
      return undefined
    }

    let active = true
    let socket: WebSocket | null = null
    let retryTimer: number | null = null

    /** connect 获取新票据后创建一次 WebSocket 连接。 */
    const connect = async () => {
      setStatus(reconnectAttemptRef.current > 0 ? 'reconnecting' : 'connecting')
      try {
        const ticket = await api.webSocketTicketProvider(url)
        if (!active || !ticket) {
          if (active) setStatus('error')
          return
        }
        socket = new WebSocket(appendTicket(url, ticket))
        socket.binaryType = binaryType
        socketRef.current = socket
        socket.addEventListener('open', () => {
          reconnectAttemptRef.current = 0
          setStatus('open')
          if (subscribeMessage !== undefined) {
            socket?.send(JSON.stringify(subscribeMessage))
          }
        })
        socket.addEventListener('message', (event: MessageEvent) => {
          messageHandlerRef.current?.(event)
        })
        socket.addEventListener('error', () => {
          if (active) setStatus('error')
        })
        socket.addEventListener('close', () => {
          if (!active) return
          reconnectAttemptRef.current += 1
          const delay = Math.min(1000 * (2 ** (reconnectAttemptRef.current - 1)), MAX_RECONNECT_DELAY_MS)
          setStatus('reconnecting')
          retryTimer = window.setTimeout(() => void connect(), delay)
        })
      } catch {
        if (!active) return
        reconnectAttemptRef.current += 1
        setStatus('error')
        const delay = Math.min(1000 * (2 ** (reconnectAttemptRef.current - 1)), MAX_RECONNECT_DELAY_MS)
        retryTimer = window.setTimeout(() => void connect(), delay)
      }
    }

    void connect()
    return () => {
      active = false
      if (retryTimer !== null) window.clearTimeout(retryTimer)
      socket?.close()
      if (socketRef.current === socket) socketRef.current = null
      setStatus('closed')
    }
  }, [binaryType, enabled, subscribeMessage, url, version])

  return { status, reconnect, send }
}
