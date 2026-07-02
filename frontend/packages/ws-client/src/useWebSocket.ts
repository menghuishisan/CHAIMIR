// WebSocket React Hook：绑定 WsClient 生命周期、连接状态和组件级错误上报。

import { useCallback, useEffect, useRef, useState } from 'react'
import { WsClient } from './WsClient'
import type { WsClientConfig, WsEventHandler } from './WsClient'

export interface UseWebSocketOptions extends Omit<WsClientConfig, 'url'> {
  /** 是否自动连接 */
  autoConnect?: boolean
}

export interface UseWebSocketReturn {
  /** WebSocket 客户端实例 */
  client: WsClient | null
  /** 是否已连接 */
  isConnected: boolean
  /** 最近一次客户端侧错误,用于页面展示友好提示或上报。 */
  lastError: unknown | null
  /** 连接 */
  connect: () => void
  /** 断开连接 */
  disconnect: () => void
  /** 发送消息 */
  send: <T = unknown>(type: string, data: T) => void
  /** 订阅事件 */
  on: <T = unknown>(type: string, handler: WsEventHandler<T>) => () => void
}

export function useWebSocket(
  url: string | null,
  options: UseWebSocketOptions = {}
): UseWebSocketReturn {
  const {
    autoConnect = true,
    protocols,
    reconnect,
    reconnectInterval,
    maxReconnectAttempts,
    heartbeatInterval,
    getToken,
    onClientError,
  } = options

  const clientRef = useRef<WsClient | null>(null)
  const [client, setClient] = useState<WsClient | null>(null)
  const [isConnected, setIsConnected] = useState(false)
  const [lastError, setLastError] = useState<unknown | null>(null)

  /**
   * reportHookError 统一记录 Hook 边界错误,并转交给应用层错误上报入口。
   */
  const reportHookError = useCallback(
    (error: unknown, context: string) => {
      setLastError(error)
      onClientError?.(error, context)
    },
    [onClientError]
  )

  /**
   * createClient 为当前 URL 创建 WsClient,并保留调用方传入的鉴权和重连配置。
   */
  const createClient = useCallback(
    (targetUrl: string) => {
      return new WsClient({
        url: targetUrl,
        protocols,
        reconnect,
        reconnectInterval,
        maxReconnectAttempts,
        heartbeatInterval,
        getToken,
        onClientError: reportHookError,
      })
    },
    [
      protocols,
      reconnect,
      reconnectInterval,
      maxReconnectAttempts,
      heartbeatInterval,
      getToken,
      reportHookError,
    ]
  )

  /**
   * connect 显式建立实时连接,无 URL 时给出可定位错误而不是静默忽略。
   */
  const connect = useCallback(() => {
    if (!url) {
      reportHookError(new Error('实时连接地址无效'), 'connect')
      return
    }
    if (!clientRef.current) {
      const nextClient = createClient(url)
      clientRef.current = nextClient
      setClient(nextClient)
    }
    clientRef.current.connect()
  }, [createClient, reportHookError, url])

  /**
   * disconnect 断开当前实时连接,用于页面卸载或用户主动退出。
   */
  const disconnect = useCallback(() => {
    clientRef.current?.disconnect()
  }, [])

  /**
   * send 发送业务消息,未创建客户端时显式上报调用时序错误。
   */
  const send = useCallback(<T = unknown>(type: string, data: T) => {
    if (!clientRef.current) {
      reportHookError(new Error('实时连接尚未建立'), 'send')
      return
    }
    clientRef.current.send(type, data)
  }, [reportHookError])

  /**
   * on 订阅业务事件,未创建客户端时返回空取消函数并显式上报。
   */
  const on = useCallback(<T = unknown>(type: string, handler: WsEventHandler<T>) => {
    if (!clientRef.current) {
      reportHookError(new Error('实时连接尚未建立'), 'subscribe')
      return () => {}
    }
    return clientRef.current.on(type, handler)
  }, [reportHookError])

  useEffect(() => {
    if (!url) {
      clientRef.current?.disconnect()
      clientRef.current = null
      setClient(null)
      setIsConnected(false)
      return
    }

    // URL 或连接配置变化时创建新的客户端,避免继续连接旧后端入口。
    const nextClient = createClient(url)
    clientRef.current = nextClient
    setClient(nextClient)
    setLastError(null)

    // 监听连接状态,供页面按钮和状态条渲染。
    const unsubConnected = nextClient.on('connected', () => {
      setIsConnected(true)
    })

    const unsubDisconnected = nextClient.on('disconnected', () => {
      setIsConnected(false)
    })

    // 自动连接只由调用方配置决定,默认进入页面即连接。
    if (autoConnect) {
      nextClient.connect()
    }

    // 清理当前 effect 创建的客户端,防止旧连接继续占用后端会话。
    return () => {
      unsubConnected()
      unsubDisconnected()
      nextClient.disconnect()
      if (clientRef.current === nextClient) {
        clientRef.current = null
        setClient(null)
        setIsConnected(false)
      }
    }
  }, [autoConnect, createClient, reportHookError, url])

  return {
    client,
    isConnected,
    lastError,
    connect,
    disconnect,
    send,
    on,
  }
}
