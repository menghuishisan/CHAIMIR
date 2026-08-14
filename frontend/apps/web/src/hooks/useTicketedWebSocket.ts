// useTicketedWebSocket 是全站唯一的 WebSocket 建连实现。
//
// 浏览器不能给 WebSocket 请求带 Authorization 头,所以后端把所有 WS 入口都放在
// `WebSocketMiddleware` 后面,只认 `?ticket=` 上的短时票据(见 auth/middleware.go
// webSocketTicketClaims)。票据要先用当前会话向 `POST /auth/ws-ticket` 换,且与目标路径绑定 ——
// 也就是说「换票 → 带票建连」这两步不可拆,任何页面自己 new WebSocket 都会 401。
// 故收敛到这一处:沙箱终端、沙箱进度、判题进度、仿真实时流与全局业务事件共用它。
//
// 票据是一次性的,重连必须重新换票;因此 reconnect 走的是同一条「换票 → 建连」路径,
// 不复用上一张票。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../app/api'
import { userFacingErrorMessage } from '../utils/userFacingError'

export type SocketStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error'

export interface TicketedSocketState {
  status: SocketStatus
  /** 用户向失败文案;技术原因进控制台,不上界面 */
  error?: string
  /** 向服务端发送一段文本;未连接时返回 false 并给出用户向状态 */
  send: (data: string) => boolean
  /** 重新换票并建连 */
  reconnect: () => void
}

export interface TicketedSocketOptions {
  /** 目标 WS 地址;传 undefined 表示当前不需要连接(如运行时未声明该能力) */
  url: string | undefined
  /** 收到一段服务端消息(二进制帧已转成文本) */
  onMessage?: (data: string) => void
  /** 连接就绪 */
  onOpen?: () => void
}

/**
 * useTicketedWebSocket 换取短时票据后建立连接,并在卸载时关闭。
 */
export function useTicketedWebSocket(options: TicketedSocketOptions): TicketedSocketState {
  const { url, onMessage, onOpen } = options
  const [status, setStatus] = useState<SocketStatus>('idle')
  const [error, setError] = useState<string>()
  const [attempt, setAttempt] = useState(0)
  const socketRef = useRef<WebSocket | undefined>(undefined)

  // 回调放进 ref:换票是异步的,把回调列进 effect 依赖会让每次渲染都重连
  const handlersRef = useRef({ onMessage, onOpen })
  handlersRef.current = { onMessage, onOpen }

  useEffect(() => {
    if (!url) {
      setStatus('idle')
      return
    }

    let active = true
    setStatus('connecting')
    setError(undefined)

    /** connect 换票后建连;票据与路径绑定,失败即给出用户向说明。 */
    const connect = async () => {
      let ticket: string | null
      try {
        ticket = await api.webSocketTicketProvider(url)
      } catch (ticketError) {
        if (!active) return
        console.error('[ws] 换取连接票据失败', { url, error: ticketError })
        setError(userFacingErrorMessage(ticketError, '实时连接没能建立,请稍后重试。'))
        setStatus('error')
        return
      }
      if (!active) return
      if (!ticket) {
        console.error('[ws] 后端未签发连接票据', { url })
        setError('与实验环境的连接建立失败,请稍后重试。')
        setStatus('error')
        return
      }

      const separator = url.includes('?') ? '&' : '?'
      const socket = new WebSocket(`${url}${separator}ticket=${encodeURIComponent(ticket)}`)
      socketRef.current = socket

      socket.onopen = () => {
        if (!active) return
        setStatus('open')
        handlersRef.current.onOpen?.()
      }
      socket.onmessage = (event: MessageEvent<unknown>) => {
        if (!active) return
        const handle = handlersRef.current.onMessage
        if (!handle) return
        if (typeof event.data === 'string') {
          handle(event.data)
          return
        }
        if (event.data instanceof Blob) {
          void event.data.text().then((text) => {
            if (active) handle(text)
          })
        }
      }
      socket.onerror = () => {
        if (!active) return
        // WebSocket 的 error 事件不带原因(规范如此),原因只能从随后的 close 码判断
        console.error('[ws] 连接出错', { url })
        setError('与实验环境的连接中断。可以重试连接。')
        setStatus('error')
      }
      socket.onclose = () => {
        if (!active) return
        setStatus((current) => (current === 'error' ? current : 'closed'))
      }
    }

    void connect()

    return () => {
      active = false
      const socket = socketRef.current
      socketRef.current = undefined
      if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
        socket.close()
      }
    }
  }, [attempt, url])

  const send = useCallback((data: string) => {
    const socket = socketRef.current
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(data)
      return true
    }
    setError('实时连接尚未就绪,请稍后重试。')
    return false
  }, [])

  const reconnect = useCallback(() => setAttempt((current) => current + 1), [])

  return useMemo(() => ({ status, error, send, reconnect }), [error, reconnect, send, status])
}
