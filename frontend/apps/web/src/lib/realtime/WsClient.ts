// WebSocket 客户端封装：统一处理短时连接票据、重连、心跳和事件分发。
import { REALTIME_ERROR_MESSAGES } from '../../copy/errors'

export interface WsClientConfig {
  url: string
  protocols?: string | string[]
  reconnect?: boolean
  reconnectInterval?: number
  maxReconnectAttempts?: number
  heartbeatInterval?: number
  getTicket?: (url: string) => Promise<string | null>
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
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private eventHandlers = new Map<string, Set<StoredWsEventHandler>>()
  private isManualClose = false
  private connectionAttempt = 0

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
      getTicket: config.getTicket || (async () => null),
      onClientError: config.onClientError || (() => undefined),
      url: config.url,
    }
  }

  /**
   * 连接 WebSocket。
   */
  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) {
      return
    }

    this.clearReconnectTimer()
    this.isManualClose = false
    this.connectionAttempt += 1
    void this.openSocket(this.connectionAttempt)
  }

  /**
   * openSocket 获取短时连接票据后建立 WebSocket,重连时会重新取票。
   */
  private async openSocket(attempt: number): Promise<void> {
    try {
      const ticket = await this.config.getTicket(this.config.url)
      if (this.isManualClose || attempt !== this.connectionAttempt) {
        return
      }
      const url = appendTicketQuery(this.config.url, ticket)
      this.ws = new WebSocket(url, this.config.protocols)
      this.setupEventListeners()
    } catch (error) {
      if (this.isManualClose || attempt !== this.connectionAttempt) {
        return
      }
      this.reportClientError(error, 'connect')
      this.handleReconnect()
    }
  }

  /**
   * 断开连接
   */
  disconnect(): void {
    this.isManualClose = true
    this.connectionAttempt += 1
    this.clearReconnectTimer()
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
      this.reportClientError(new Error(REALTIME_ERROR_MESSAGES.NOT_CONNECTED), 'send')
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
    if (this.isManualClose || !this.config.reconnect) {
      return
    }
    if (this.reconnectAttempts >= this.config.maxReconnectAttempts) {
      this.emit('reconnect_failed', {})
      return
    }

    this.reconnectAttempts++
    this.clearReconnectTimer()

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      if (this.isManualClose) {
        return
      }
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
   * clearReconnectTimer 清理待执行重连，避免页面卸载或主动断开后重新连回。
   */
  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
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
 * appendTicketQuery 为 WebSocket URL 附加后端要求的短时连接票据,并避免重复追加。
 */
function appendTicketQuery(url: string, ticket: string | null): string {
  if (!ticket) {
    return url
  }
  if (/[?&]ticket=/.test(url)) {
    return url
  }
  const separator = url.includes('?') ? '&' : '?'
  return `${url}${separator}ticket=${encodeURIComponent(ticket)}`
}
