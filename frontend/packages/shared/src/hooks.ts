// React Hooks：提供前端共享状态、定时器、媒体查询和剪贴板交互能力。

import { useState, useEffect, useCallback, useRef } from 'react'
import { safeJsonParse } from './utils'

export interface UseLocalStorageOptions {
  /** 读写失败时的显式错误上报入口,由应用层决定如何提示或记录。 */
  onError?: (error: unknown, context: 'read' | 'write') => void
}

/**
 * useLocalStorage 把组件状态同步到 localStorage,不可用时退回内存状态。
 */
export function useLocalStorage<T>(
  key: string,
  initialValue: T,
  options?: UseLocalStorageOptions
): [T, (value: T) => void] {
  const [storedValue, setStoredValue] = useState<T>(() => {
    if (typeof window === 'undefined') {
      return initialValue
    }

    try {
      const item = window.localStorage.getItem(key)
      return item ? safeJsonParse<T>(item, initialValue, (error) => options?.onError?.(error, 'read')) : initialValue
    } catch (error) {
      options?.onError?.(error, 'read')
      return initialValue
    }
  })

  const setValue = useCallback(
    (value: T) => {
      if (typeof window === 'undefined') {
        setStoredValue(value)
        return
      }

      try {
        setStoredValue(value)
        window.localStorage.setItem(key, JSON.stringify(value))
      } catch (error) {
        options?.onError?.(error, 'write')
      }
    },
    [key, options]
  )

  return [storedValue, setValue]
}

/**
 * useDebounce 返回延迟稳定后的值,用于降低高频输入触发频率。
 */
export function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState<T>(value)

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedValue(value)
    }, delay)

    return () => clearTimeout(timer)
  }, [value, delay])

  return debouncedValue
}

/**
 * useInterval 使用最新 callback 执行可暂停的 interval。
 */
export function useInterval(callback: () => void, delay: number | null): void {
  const savedCallback = useRef(callback)

  useEffect(() => {
    savedCallback.current = callback
  }, [callback])

  useEffect(() => {
    if (delay === null) return

    const id = setInterval(() => savedCallback.current(), delay)
    return () => clearInterval(id)
  }, [delay])
}

/**
 * useOnClickOutside 在鼠标或触摸事件落到目标元素外部时触发处理器。
 */
export function useOnClickOutside<T extends HTMLElement = HTMLElement>(
  ref: React.RefObject<T>,
  handler: (event: MouseEvent | TouchEvent) => void
): void {
  useEffect(() => {
    const listener = (event: MouseEvent | TouchEvent) => {
      const el = ref.current
      if (!el || el.contains(event.target as Node)) {
        return
      }
      handler(event)
    }

    document.addEventListener('mousedown', listener)
    document.addEventListener('touchstart', listener)

    return () => {
      document.removeEventListener('mousedown', listener)
      document.removeEventListener('touchstart', listener)
    }
  }, [ref, handler])
}

/**
 * useCopyToClipboard 使用 Clipboard API 复制文本,并用返回值表达是否成功。
 */
export function useCopyToClipboard(
  onError?: (error: unknown) => void
): [(text: string) => Promise<boolean>, boolean] {
  const [copied, setCopied] = useState(false)

  const copy = useCallback(async (text: string) => {
    if (typeof navigator === 'undefined' || !navigator.clipboard) {
      setCopied(false)
      return false
    }

    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
      return true
    } catch (error) {
      setCopied(false)
      onError?.(error)
      return false
    }
  }, [onError])

  return [copy, copied]
}

/**
 * useMediaQuery 订阅浏览器媒体查询状态,服务端渲染时默认返回 false。
 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => {
    if (typeof window !== 'undefined') {
      return window.matchMedia(query).matches
    }
    return false
  })

  useEffect(() => {
    if (typeof window === 'undefined') return

    const media = window.matchMedia(query)
    const listener = (event: MediaQueryListEvent) => setMatches(event.matches)

    // 使用现代 API
    media.addEventListener('change', listener)

    // 初始化状态
    setMatches(media.matches)

    return () => {
      media.removeEventListener('change', listener)
    }
  }, [query])

  return matches
}

/**
 * useToggle 管理可切换的 boolean 状态。
 */
export function useToggle(initialValue = false): [boolean, () => void] {
  const [value, setValue] = useState(initialValue)
  const toggle = useCallback(() => setValue((v) => !v), [])
  return [value, toggle]
}
