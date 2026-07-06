// 浏览器能力：剪贴板、下载和临时前端标识。

export function generateId(prefix = 'id'): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).substring(2, 11)}`
}

export async function copyToClipboard(text: string, onError?: (error: unknown) => void): Promise<boolean> {
  if (typeof navigator === 'undefined' || !navigator.clipboard) {
    return false
  }

  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch (error) {
    onError?.(error)
    console.warn('复制到剪贴板失败', error)
    return false
  }
}

export function downloadFile(url: string, filename: string): void {
  if (typeof document === 'undefined') {
    return
  }

  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

export function downloadBlob(blob: Blob, filename: string): void {
  if (typeof URL === 'undefined') {
    return
  }

  const objectUrl = URL.createObjectURL(blob)
  try {
    downloadFile(objectUrl, filename)
  } finally {
    URL.revokeObjectURL(objectUrl)
  }
}

export function getFileExtension(filename: string): string {
  const parts = filename.split('.')
  return parts.length > 1 ? parts[parts.length - 1].toLowerCase() : ''
}
