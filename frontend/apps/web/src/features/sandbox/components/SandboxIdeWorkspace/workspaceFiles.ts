// workspaceFiles 提供沙箱目录遍历、IDE 文件映射和 UTF-8 Base64 转换。

import type { IdeWorkbenchFile } from '@chaimir/ide'
import type { SandboxFileEntry } from '@chaimir/api-client'
import { api } from '../../../../app/api'

export interface WorkspaceFile extends IdeWorkbenchFile {
  path: string
}

const MAX_DIRECTORY_DEPTH = 12
const MAX_FILE_COUNT = 500

/** listWorkspaceFiles 递归读取工作区目录，并以明确上限保护浏览器。 */
export async function listWorkspaceFiles(sandboxId: string, path = '.', depth = 0, collected: SandboxFileEntry[] = []): Promise<SandboxFileEntry[]> {
  if (depth > MAX_DIRECTORY_DEPTH) throw new Error('工作区目录层级过深，暂时无法完整打开。')
  const response = await api.sandbox.listFiles(sandboxId, path)
  for (const entry of response.entries) {
    if (collected.length >= MAX_FILE_COUNT) throw new Error('工作区文件过多，请精简文件后重试。')
    if (entry.is_dir) await listWorkspaceFiles(sandboxId, entry.relative_path, depth + 1, collected)
    else collected.push(entry)
  }
  return collected
}

/** toWorkspaceFile 把后端文件条目映射为共享 IDE 文件模型。 */
export function toWorkspaceFile(entry: SandboxFileEntry): WorkspaceFile {
  return { id: entry.relative_path, path: entry.relative_path, name: entry.name, language: languageFromPath(entry.relative_path) }
}

/** encodeBase64 用 UTF-8 安全编码文件内容。 */
export function encodeBase64(value: string): string {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
}

/** decodeBase64 用 UTF-8 安全解码后端文件和命令输出。 */
export function decodeBase64(value: string): string {
  if (!value) return ''
  const binary = atob(value)
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}

/** languageFromPath 根据扩展名选择 Monaco 语言，不依赖页面业务。 */
function languageFromPath(path: string): string {
  const extension = path.split('.').pop()?.toLowerCase()
  return ({ ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript', go: 'go', sol: 'sol', py: 'python', json: 'json', md: 'markdown', yaml: 'yaml', yml: 'yaml', sh: 'shell', css: 'css', html: 'html' } as Record<string, string>)[extension || ''] || 'plaintext'
}
