// 应用级 React Hooks：浏览器状态、媒体查询和剪贴板交互。

import { useState, useEffect, useCallback, useRef } from 'react'
import { copyToClipboard as writeClipboard } from './browser'
import { safeJsonParse } from './json'

export interface UseLocalStorageOptions {
  /** 读写失败时的显式错误上报入口,由应用层决定如何提示或记录。 */
  onError?: (error: unknown, context: 'read' | 'write') => void
}

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

export function useCopyToClipboard(
  onError?: (error: unknown) => void
): [(text: string) => Promise<boolean>, boolean] {
  const [copied, setCopied] = useState(false)
  const resetTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const copy = useCallback(async (text: string) => {
    const success = await writeClipboard(text, onError)
    if (success) {
      if (resetTimer.current) {
        clearTimeout(resetTimer.current)
      }
      setCopied(true)
      resetTimer.current = setTimeout(() => {
        resetTimer.current = null
        setCopied(false)
      }, 2000)
      return true
    }

    setCopied(false)
    return false
  }, [onError])

  useEffect(() => {
    return () => {
      if (resetTimer.current) {
        clearTimeout(resetTimer.current)
      }
    }
  }, [])

  return [copy, copied]
}

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

    media.addEventListener('change', listener)
    setMatches(media.matches)

    return () => {
      media.removeEventListener('change', listener)
    }
  }, [query])

  return matches
}

export function useToggle(initialValue = false): [boolean, () => void] {
  const [value, setValue] = useState(initialValue)
  const toggle = useCallback(() => setValue((v) => !v), [])
  return [value, toggle]
}
