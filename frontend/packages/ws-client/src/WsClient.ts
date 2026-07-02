// WebSocket 客户端封装：统一处理后端 query token 鉴权、重连、心跳和事件分发。

export interface WsClientConfig {
  url: string
  protocols?: string | string[]
  reconnect?: boolean
  reconnectInterval?: number
  maxReconnectAttempts?: number
  heartbeatInterval?: number
  getToken?: () => string | null
  onClientError?: (error: unknown, context: string) => void
}

export interface WsMessage<T = unknown> {
  type: string
  data: T
  timestamp?: number
}

export type WsEventHandler<T = unknown> = (data: T) => void
type StoredWsEventHandler = WsEventHandler<unknown>

export class WsClient {
  private ws: WebSocket | null = null
  private config: Required<WsClientConfig>
  private reconnectAttempts = 0
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private eventHandlers = new Map<string, Set<StoredWsEventHandler>>()
  private isManualClose = false

  /**
   * constructor 创建一个可重连的 WebSocket 客户端实例。
   */
  constructor(config: WsClientConfig) {
    this.config = {
      protocols: config.protocols || [],
      reconnect: config.reconnect ?? true,
      reconnectInterval: config.reconnectInterval || 3000,
      maxReconnectAttempts: config.maxReconnectAttempts || 5,
      heartbeatInterval: config.heartbeatInterval || 30000,
      getToken: config.getToken || (() => null),
      onClientError: config.onClientError || (() => undefined),
      url: config.url,
    }
  }

  /**
   * 连接 WebSocket
   */
  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return
    }

    this.isManualClose = false

    // 后端 WebSocket 中间件要求通过 query token 鉴权，保留调用方已有查询参数。
    const url = appendTokenQuery(this.config.url, this.config.getToken())

    try {
      this.ws = new WebSocket(url, this.config.protocols)
      this.setupEventListeners()
    } catch (error) {
      this.reportClientError(error, 'connect')
      this.handleReconnect()
    }
  }

  /**
   * 断开连接
   */
  disconnect(): void {
    this.isManualClose = true
    this.clearHeartbeat()

    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  /**
   * 发送消息
   */
  send<T = unknown>(type: string, data: T): void {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      this.reportClientError(new Error('实时连接尚未建立'), 'send')
      return
    }

    const message: WsMessage<T> = {
      type,
      data,
      timestamp: Date.now(),
    }

    this.ws.send(JSON.stringify(message))
  }

  /**
   * 订阅事件
   */
  on<T = unknown>(type: string, handler: WsEventHandler<T>): () => void {
    if (!this.eventHandlers.has(type)) {
      this.eventHandlers.set(type, new Set())
    }
    this.eventHandlers.get(type)!.add(handler as StoredWsEventHandler)

    // 返回取消订阅函数
    return () => {
      this.off(type, handler)
    }
  }

  /**
   * 取消订阅
   */
  off<T = unknown>(type: string, handler: WsEventHandler<T>): void {
    const handlers = this.eventHandlers.get(type)
    if (handlers) {
      handlers.delete(handler as StoredWsEventHandler)
      if (handlers.size === 0) {
        this.eventHandlers.delete(type)
      }
    }
  }

  /**
   * 获取连接状态
   */
  getReadyState(): number {
    return this.ws?.readyState ?? WebSocket.CLOSED
  }

  /**
   * 是否已连接
   */
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  /**
   * setupEventListeners 绑定浏览器 WebSocket 事件并转换为业务事件。
   */
  private setupEventListeners(): void {
    if (!this.ws) return

    this.ws.onopen = () => {
      this.reconnectAttempts = 0
      this.startHeartbeat()
      this.emit('connected', {})
    }

    this.ws.onmessage = (event) => {
      try {
        const message: WsMessage = JSON.parse(event.data)
        this.emit(message.type, message.data)
      } catch (error) {
        this.reportClientError(error, 'message_parse')
      }
    }

    this.ws.onerror = (error) => {
      this.reportClientError(error, 'socket_error')
      this.emit('error', error)
    }

    this.ws.onclose = (event) => {
      this.clearHeartbeat()
      this.emit('disconnected', { code: event.code, reason: event.reason })

      if (!this.isManualClose && this.config.reconnect) {
        this.handleReconnect()
      }
    }
  }

  /**
   * handleReconnect 在非手动关闭后按配置执行有限重连。
   */
  private handleReconnect(): void {
    if (this.reconnectAttempts >= this.config.maxReconnectAttempts) {
      this.emit('reconnect_failed', {})
      return
    }

    this.reconnectAttempts++

    setTimeout(() => {
      this.connect()
    }, this.config.reconnectInterval)
  }

  /**
   * startHeartbeat 定期发送 ping,保持统一实时通道活跃。
   */
  private startHeartbeat(): void {
    this.clearHeartbeat()

    this.heartbeatTimer = setInterval(() => {
      if (this.isConnected()) {
        this.send('ping', {})
      }
    }, this.config.heartbeatInterval)
  }

  /**
   * clearHeartbeat 清理心跳定时器,避免断开后残留后台任务。
   */
  private clearHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  /**
   * emit 分发已解析的 WebSocket 事件到订阅处理器。
   */
  private emit<T = unknown>(type: string, data: T): void {
    const handlers = this.eventHandlers.get(type)
    if (handlers) {
      handlers.forEach((handler) => {
        try {
          handler(data)
        } catch (error) {
          this.reportClientError(error, `handler:${type}`)
        }
      })
    }
  }

  /**
   * reportClientError 把客户端内部错误交给应用层处理,避免共享包直接输出开发语义。
   */
  private reportClientError(error: unknown, context: string): void {
    this.config.onClientError(error, context)
  }
}

/**
 * appendTokenQuery 为 WebSocket URL 附加后端要求的 query token,并避免重复追加。
 */
function appendTokenQuery(url: string, token: string | null): string {
  if (!token) {
    return url
  }
  if (/[?&]token=/.test(url)) {
    return url
  }
  const separator = url.includes('?') ? '&' : '?'
  return `${url}${separator}token=${encodeURIComponent(token)}`
}
