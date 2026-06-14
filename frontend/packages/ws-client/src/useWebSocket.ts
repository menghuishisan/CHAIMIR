// React Hook for WebSocket

import { useEffect, useRef, useState, useCallback } from 'react'
import { WsClient, WsClientConfig, WsEventHandler } from './WsClient'

export interface UseWebSocketOptions extends Omit<WsClientConfig, 'url'> {
  /** 是否自动连接 */
  autoConnect?: boolean
}

export interface UseWebSocketReturn {
  /** WebSocket 客户端实例 */
  client: WsClient | null
  /** 是否已连接 */
  isConnected: boolean
  /** 连接 */
  connect: () => void
  /** 断开连接 */
  disconnect: () => void
  /** 发送消息 */
  send: <T = any>(type: string, data: T) => void
  /** 订阅事件 */
  on: <T = any>(type: string, handler: WsEventHandler<T>) => () => void
}

export function useWebSocket(
  url: string | null,
  options: UseWebSocketOptions = {}
): UseWebSocketReturn {
  const { autoConnect = true, ...config } = options

  const clientRef = useRef<WsClient | null>(null)
  const [isConnected, setIsConnected] = useState(false)

  const connect = useCallback(() => {
    if (!url) return
    if (!clientRef.current) {
      clientRef.current = new WsClient({ url, ...config })
    }
    clientRef.current.connect()
  }, [url, config])

  const disconnect = useCallback(() => {
    clientRef.current?.disconnect()
  }, [])

  const send = useCallback(<T = any>(type: string, data: T) => {
    clientRef.current?.send(type, data)
  }, [])

  const on = useCallback(<T = any>(type: string, handler: WsEventHandler<T>) => {
    return clientRef.current?.on(type, handler) || (() => {})
  }, [])

  useEffect(() => {
    if (!url) return

    // 创建客户端
    if (!clientRef.current) {
      clientRef.current = new WsClient({ url, ...config })
    }

    // 监听连接状态
    const unsubConnected = clientRef.current.on('connected', () => {
      setIsConnected(true)
    })

    const unsubDisconnected = clientRef.current.on('disconnected', () => {
      setIsConnected(false)
    })

    // 自动连接
    if (autoConnect) {
      clientRef.current.connect()
    }

    // 清理
    return () => {
      unsubConnected()
      unsubDisconnected()
      clientRef.current?.disconnect()
    }
  }, [url, autoConnect])

  return {
    client: clientRef.current,
    isConnected,
    connect,
    disconnect,
    send,
    on,
  }
}
