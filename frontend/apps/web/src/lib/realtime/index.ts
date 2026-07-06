// 实时连接入口：导出统一 WebSocket 客户端和 React Hook。

export { WsClient } from './WsClient'
export type { WsClientConfig, WsMessage, WsEventHandler } from './WsClient'

export { useWebSocket } from './useWebSocket'
export type { UseWebSocketOptions, UseWebSocketReturn } from './useWebSocket'
