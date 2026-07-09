// useFocusTrap：为 Modal 和 Drawer 提供焦点循环与关闭后的焦点恢复。

import { useEffect, RefObject } from 'react'

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

/**
 * useFocusTrap 在浮层打开时聚焦首个可操作元素，并把 Tab 焦点限制在容器内。
 */
export function useFocusTrap(
  containerRef: RefObject<HTMLElement>,
  enabled: boolean = true
): void {
  useEffect(() => {
    if (!enabled) return

    const container = containerRef.current
    if (!container) return

    const previousActiveElement = document.activeElement as HTMLElement

    // 优先聚焦容器内的第一个可交互元素，若无则聚焦容器本身
    const focusableElements = getFocusableElements(container)
    if (focusableElements.length > 0) {
      focusableElements[0].focus()
    } else {
      container.focus()
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Tab') {
        const elements = getFocusableElements(container)
        if (elements.length === 0) {
          e.preventDefault()
          return
        }

        const first = elements[0]
        const last = elements[elements.length - 1]

        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault()
          last.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }

    document.addEventListener('keydown', handleKeyDown)

    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      // 组件卸载或 disabled 时，恢复之前的焦点
      if (previousActiveElement && document.body.contains(previousActiveElement)) {
        previousActiveElement.focus()
      }
    }
  }, [containerRef, enabled])
}

/**
 * getFocusableElements 返回当前容器内可见且可通过键盘访问的元素。
 */
function getFocusableElements(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))
    .filter((element) => element.offsetParent !== null)
}
