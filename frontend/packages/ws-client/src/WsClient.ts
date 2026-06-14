// WebSocket 客户端封装
// 用于实时通知、判题进度、沙箱状态等

export interface WsClientConfig {
  url: string
  protocols?: string | string[]
  reconnect?: boolean
  reconnectInterval?: number
  maxReconnectAttempts?: number
  heartbeatInterval?: number
  getToken?: () => string | null
}

export interface WsMessage<T = any> {
  type: string
  data: T
  timestamp?: number
}

export type WsEventHandler<T = any> = (data: T) => void

export class WsClient {
  private ws: WebSocket | null = null
  private config: Required<WsClientConfig>
  private reconnectAttempts = 0
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private eventHandlers = new Map<string, Set<WsEventHandler>>()
  private isManualClose = false

  constructor(config: WsClientConfig) {
    this.config = {
      protocols: config.protocols || [],
      reconnect: config.reconnect ?? true,
      reconnectInterval: config.reconnectInterval || 3000,
      maxReconnectAttempts: config.maxReconnectAttempts || 5,
      heartbeatInterval: config.heartbeatInterval || 30000,
      getToken: config.getToken || (() => null),
      url: config.url,
    }
  }

  /**
   * 连接 WebSocket
   */
  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      console.warn('WebSocket 已连接')
      return
    }

    this.isManualClose = false

    // 构建 URL（带 Token）
    const token = this.config.getToken()
    const url = token ? `${this.config.url}?token=${token}` : this.config.url

    try {
      this.ws = new WebSocket(url, this.config.protocols)
      this.setupEventListeners()
    } catch (error) {
      console.error('WebSocket 连接失败:', error)
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
  send<T = any>(type: string, data: T): void {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      console.error('WebSocket 未连接')
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
  on<T = any>(type: string, handler: WsEventHandler<T>): () => void {
    if (!this.eventHandlers.has(type)) {
      this.eventHandlers.set(type, new Set())
    }
    this.eventHandlers.get(type)!.add(handler)

    // 返回取消订阅函数
    return () => {
      this.off(type, handler)
    }
  }

  /**
   * 取消订阅
   */
  off<T = any>(type: string, handler: WsEventHandler<T>): void {
    const handlers = this.eventHandlers.get(type)
    if (handlers) {
      handlers.delete(handler)
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

  private setupEventListeners(): void {
    if (!this.ws) return

    this.ws.onopen = () => {
      console.log('WebSocket 已连接')
      this.reconnectAttempts = 0
      this.startHeartbeat()
      this.emit('connected', {})
    }

    this.ws.onmessage = (event) => {
      try {
        const message: WsMessage = JSON.parse(event.data)
        this.emit(message.type, message.data)
      } catch (error) {
        console.error('解析 WebSocket 消息失败:', error)
      }
    }

    this.ws.onerror = (error) => {
      console.error('WebSocket 错误:', error)
      this.emit('error', error)
    }

    this.ws.onclose = (event) => {
      console.log('WebSocket 已断开:', event.code, event.reason)
      this.clearHeartbeat()
      this.emit('disconnected', { code: event.code, reason: event.reason })

      if (!this.isManualClose && this.config.reconnect) {
        this.handleReconnect()
      }
    }
  }

  private handleReconnect(): void {
    if (this.reconnectAttempts >= this.config.maxReconnectAttempts) {
      console.error('WebSocket 重连次数已达上限')
      this.emit('reconnect_failed', {})
      return
    }

    this.reconnectAttempts++
    console.log(`WebSocket 将在 ${this.config.reconnectInterval}ms 后重连（第 ${this.reconnectAttempts} 次）`)

    setTimeout(() => {
      this.connect()
    }, this.config.reconnectInterval)
  }

  private startHeartbeat(): void {
    this.clearHeartbeat()

    this.heartbeatTimer = setInterval(() => {
      if (this.isConnected()) {
        this.send('ping', {})
      }
    }, this.config.heartbeatInterval)
  }

  private clearHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  private emit<T = any>(type: string, data: T): void {
    const handlers = this.eventHandlers.get(type)
    if (handlers) {
      handlers.forEach((handler) => {
        try {
          handler(data)
        } catch (error) {
          console.error(`事件处理器错误 (${type}):`, error)
        }
      })
    }
  }
}
