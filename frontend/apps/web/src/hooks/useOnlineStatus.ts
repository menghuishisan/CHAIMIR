// useOnlineStatus 订阅浏览器网络连通状态,供侧栏底部常驻状态行显示真实连通性
// (规范 §6.2 要求该行常显;显示的必须是真实状态,不写死文案)。

import { useSyncExternalStore } from 'react'

/** subscribe 监听浏览器上下线事件 */
function subscribe(callback: () => void): () => void {
  window.addEventListener('online', callback)
  window.addEventListener('offline', callback)
  return () => {
    window.removeEventListener('online', callback)
    window.removeEventListener('offline', callback)
  }
}

/**
 * useOnlineStatus 返回当前浏览器是否处于联网状态。
 */
export function useOnlineStatus(): boolean {
  return useSyncExternalStore(subscribe, () => window.navigator.onLine)
}
