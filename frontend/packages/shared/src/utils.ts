// 共享工具函数：提供前端通用格式化、节流防抖、类名合并和安全解析能力。

/**
 * formatDate 把日期字符串或 Date 对象格式化为指定展示格式。
 */
export function formatDate(date: string | Date, format = 'YYYY-MM-DD HH:mm:ss'): string {
  const d = typeof date === 'string' ? new Date(date) : date

  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hours = String(d.getHours()).padStart(2, '0')
  const minutes = String(d.getMinutes()).padStart(2, '0')
  const seconds = String(d.getSeconds()).padStart(2, '0')

  return format
    .replace('YYYY', String(year))
    .replace('MM', month)
    .replace('DD', day)
    .replace('HH', hours)
    .replace('mm', minutes)
    .replace('ss', seconds)
}

/**
 * formatRelativeTime 把时间转换为面向用户的相对时间文案。
 */
export function formatRelativeTime(date: string | Date): string {
  const d = typeof date === 'string' ? new Date(date) : date
  const now = new Date()
  const diff = now.getTime() - d.getTime()

  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (seconds < 60) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  if (hours < 24) return `${hours}小时前`
  if (days < 7) return `${days}天前`

  return formatDate(d, 'YYYY-MM-DD')
}

/**
 * debounce 延迟执行连续触发的函数,用于输入搜索等高频交互。
 */
export function debounce<TArgs extends unknown[]>(
  fn: (...args: TArgs) => void,
  delay: number
): (...args: TArgs) => void {
  let timer: ReturnType<typeof setTimeout> | null = null

  return (...args: TArgs) => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => {
      fn(...args)
    }, delay)
  }
}

/**
 * throttle 限制函数在固定时间窗口内最多执行一次。
 */
export function throttle<TArgs extends unknown[]>(
  fn: (...args: TArgs) => void,
  delay: number
): (...args: TArgs) => void {
  let lastCall = 0

  return (...args: TArgs) => {
    const now = Date.now()
    if (now - lastCall >= delay) {
      lastCall = now
      fn(...args)
    }
  }
}

/**
 * generateId 生成仅供前端临时状态使用的唯一标识。
 */
export function generateId(prefix = 'id'): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).substring(2, 11)}`
}

/**
 * copyToClipboard 使用浏览器 Clipboard API 复制文本,失败时通过返回值交给调用方提示。
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (typeof navigator === 'undefined' || !navigator.clipboard) {
    return false
  }

  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

/**
 * formatFileSize 把字节数格式化为常用文件大小单位。
 */
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)

  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`
}

/**
 * downloadFile 基于受控下载授权 URL 触发浏览器下载。
 */
export function downloadFile(url: string, filename: string): void {
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
}

type ClassDictionary = Record<string, boolean | null | undefined>
type ClassValue = string | number | boolean | undefined | null | ClassValue[] | ClassDictionary

/**
 * cn 合并字符串、数组和对象形式的条件类名。
 */
export function cn(...inputs: ClassValue[]): string {
  const classes: string[] = []

  for (const input of inputs) {
    if (!input) continue

    if (typeof input === 'string' || typeof input === 'number') {
      classes.push(String(input))
      continue
    }

    if (Array.isArray(input)) {
      const result = cn(...input)
      if (result) {
        classes.push(result)
      }
      continue
    }

    if (typeof input === 'object') {
      for (const [className, enabled] of Object.entries(input)) {
        if (enabled) {
          classes.push(className)
        }
      }
    }
  }

  return classes.join(' ')
}

/**
 * sleep 返回在指定毫秒后完成的 Promise,用于受控轮询或交互等待。
 */
export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/**
 * maskPhone 脱敏展示 11 位手机号。
 */
export function maskPhone(phone: string): string {
  if (phone.length !== 11) return phone
  return `${phone.slice(0, 3)}****${phone.slice(7)}`
}

/**
 * isValidPhone 校验中国大陆手机号格式。
 */
export function isValidPhone(phone: string): boolean {
  return /^1[3-9]\d{9}$/.test(phone)
}

/**
 * isValidEmail 校验基础邮箱格式。
 */
export function isValidEmail(email: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)
}

/**
 * getFileExtension 读取文件扩展名,无扩展名时返回空字符串。
 */
export function getFileExtension(filename: string): string {
  const parts = filename.split('.')
  return parts.length > 1 ? parts[parts.length - 1].toLowerCase() : ''
}

/**
 * safeJsonParse 安全解析 JSON,失败时返回调用方提供的兜底值并可显式上报错误。
 */
export function safeJsonParse<T = unknown>(
  str: string,
  fallback: T,
  onError?: (error: unknown) => void
): T {
  try {
    return JSON.parse(str) as T
  } catch (error) {
    onError?.(error)
    return fallback
  }
}
