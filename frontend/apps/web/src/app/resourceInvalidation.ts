// resourceInvalidation 提供应用级有类型资源失效协议，统一刷新跨页面共享数据。

export type AppResourceKey = 'notification-unread' | 'profile'

const RESOURCE_INVALIDATION_EVENT = 'chaimir:resource-invalidated'

/** invalidateAppResource 通知所有共享数据消费者重新读取服务端权威状态。 */
export function invalidateAppResource(key: AppResourceKey): void {
  window.dispatchEvent(new CustomEvent<AppResourceKey>(RESOURCE_INVALIDATION_EVENT, { detail: key }))
}

/** subscribeAppResource 订阅指定共享资源失效，并返回解除订阅函数。 */
export function subscribeAppResource(key: AppResourceKey, listener: () => void): () => void {
  const handleInvalidation = (event: Event): void => {
    if ((event as CustomEvent<AppResourceKey>).detail === key) listener()
  }
  window.addEventListener(RESOURCE_INVALIDATION_EVENT, handleInvalidation)
  return () => window.removeEventListener(RESOURCE_INVALIDATION_EVENT, handleInvalidation)
}
