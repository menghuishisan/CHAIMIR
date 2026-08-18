// base64 文本编解码:M2 沙箱的文件读写与命令工具输出都以 base64 传输
// (二进制安全,后端不猜编码)。浏览器原生 atob/btoa 只处理 Latin-1,
// 直接用会把中文注释与文件名弄坏,故统一在此按 UTF-8 转换一次,各页面不再各写一遍。

/** encodeUtf8Base64 把文本按 UTF-8 编成 base64。 */
export function encodeUtf8Base64(text: string): string {
  const bytes = new TextEncoder().encode(text)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary)
}

/**
 * decodeUtf8Base64 把 base64 按 UTF-8 解成文本。
 * 内容不是合法 base64 时回空串而不是抛错:调用方拿到的是「这个文件读不出内容」,
 * 而不是让一个坏文件把整个工作台打崩。
 */
export function decodeUtf8Base64(value: string): string {
  try {
    const binary = atob(value)
    const bytes = new Uint8Array(binary.length)
    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index)
    }
    return new TextDecoder().decode(bytes)
  } catch (error) {
    console.error('[sandbox] 文件内容不是合法的 base64', { kind: error instanceof Error ? error.name : typeof error })
    return ''
  }
}
