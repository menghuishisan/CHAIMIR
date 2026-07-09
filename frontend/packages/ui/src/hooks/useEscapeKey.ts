// useEscapeKey：为弹层、抽屉和下拉组件提供统一的 Esc 关闭能力。

import { useEffect } from 'react'

/**
 * useEscapeKey 在启用时监听 Escape 键，并把原始键盘事件交给调用方处理。
 */
export function useEscapeKey(
  handler: (event: KeyboardEvent) => void,
  enabled: boolean = true
): void {
  useEffect(() => {
    if (!enabled) return

    const listener = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        handler(event)
      }
    }

    document.addEventListener('keydown', listener)

    return () => {
      document.removeEventListener('keydown', listener)
    }
  }, [handler, enabled])
}
