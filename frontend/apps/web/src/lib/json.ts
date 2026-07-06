// JSON 工具：安全解析并由调用方决定错误处理。

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
