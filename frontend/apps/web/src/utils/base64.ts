export function encodeUtf8Base64(text: string): string {
  const bytes = new TextEncoder().encode(text)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary)
}

// 解码失败交给调用方处理；不得把坏数据变成可保存的空文件或丢弃原 BOM。
export function decodeUtf8Base64(value: string): string {
  const bytes = Uint8Array.from(atob(value), (character) => character.charCodeAt(0))
  return new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(bytes)
}
