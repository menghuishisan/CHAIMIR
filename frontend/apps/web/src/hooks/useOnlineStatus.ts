import { useSyncExternalStore } from 'react'

function subscribe(callback: () => void): () => void {
  window.addEventListener('online', callback)
  window.addEventListener('offline', callback)
  return () => {
    window.removeEventListener('online', callback)
    window.removeEventListener('offline', callback)
  }
}

/** useOnlineStatus 订阅浏览器联网标记；不代表平台服务可达。 */
export function useOnlineStatus(): boolean {
  return useSyncExternalStore(subscribe, () => window.navigator.onLine)
}
