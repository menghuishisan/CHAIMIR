// SandboxIdeWorkspace 把后端沙箱能力接入共享 IDE 工作台，供实验和竞赛沉浸页复用。

import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { IdeWorkbench } from '@chaimir/ide'
import type { SandboxFileSaveResponse, SandboxProgressMessage } from '@chaimir/api-client'
import { Button, Callout, ResourceState } from '@chaimir/ui'
import { RefreshCw, Save } from 'lucide-react'
import { api } from '../../../../app/api'
import { useAsyncResource, useTicketedWebSocket } from '../../../../hooks'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { MonacoPane, SandboxTerminal } from './EditorTerminalPanes'
import { SandboxOperations, toWorkbenchTools } from './SandboxTools'
import { decodeBase64, encodeBase64, listWorkspaceFiles, toWorkspaceFile } from './workspaceFiles'
import type { WorkspaceFile } from './workspaceFiles'
import styles from './SandboxIdeWorkspace.module.css'

export interface SandboxIdeWorkspaceProps {
  sandboxId: string
  title: React.ReactNode
  subtitle?: React.ReactNode
  inspector?: React.ReactNode
  controls?: React.ReactNode
  actions?: React.ReactNode
  onSaved?: (result: SandboxFileSaveResponse) => void
}

/** SandboxIdeWorkspace 维护沙箱文件、编辑器、终端和工具的统一生命周期。 */
export function SandboxIdeWorkspace({
  sandboxId,
  title,
  subtitle,
  inspector,
  controls,
  actions,
  onSaved,
}: SandboxIdeWorkspaceProps): React.ReactElement {
  const [activeFileId, setActiveFileId] = useState<string>()
  const [contents, setContents] = useState<Record<string, string>>({})
  const [dirtyPaths, setDirtyPaths] = useState<Set<string>>(new Set())
  const [message, setMessage] = useState<string>()
  const [actionError, setActionError] = useState<string>()
  const [saving, setSaving] = useState(false)
  const [progressMessage, setProgressMessage] = useState<SandboxProgressMessage>()
  const resource = useAsyncResource(
    async () => {
      const instance = await api.sandbox.getInstance(sandboxId)
      const entries = instance.capabilities.file_workspace ? await listWorkspaceFiles(sandboxId) : []
      return { instance, files: entries.map(toWorkspaceFile) }
    },
    [sandboxId],
  )
  const progressUrl = useMemo(() => api.sandbox.getProgressWsUrl(sandboxId), [sandboxId])
  const progress = useTicketedWebSocket({
    url: progressUrl,
    onMessage: useCallback((event: MessageEvent) => {
      const next = parseProgressMessage(event.data)
      if (next) setProgressMessage(next)
      resource.reload()
    }, [resource]),
  })
  const files = useMemo(() => resource.data?.files || [], [resource.data?.files])
  const activeFile = files.find((file) => file.id === activeFileId)

  useEffect(() => {
    setActiveFileId(undefined)
    setContents({})
    setDirtyPaths(new Set())
    setMessage(undefined)
    setActionError(undefined)
    setProgressMessage(undefined)
  }, [sandboxId])

  /** selectFile 按需读取选中文件，避免进入工作台时下载全部源码。 */
  const selectFile = useCallback(async (file: WorkspaceFile) => {
    setActiveFileId(file.id)
    if (contents[file.path] !== undefined) return
    setActionError(undefined)
    try {
      const response = await api.sandbox.readFile(sandboxId, file.path)
      setContents((current) => ({ ...current, [file.path]: decodeBase64(response.content_base64) }))
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '暂时无法读取该文件，请稍后重试。'))
    }
  }, [contents, sandboxId])

  useEffect(() => {
    if (!activeFileId && files[0]) void selectFile(files[0])
  }, [activeFileId, files, selectFile])

  /** updateActiveContent 更新当前缓冲区并记录未保存状态。 */
  const updateActiveContent = useCallback((value: string) => {
    if (!activeFile) return
    setContents((current) => ({ ...current, [activeFile.path]: value }))
    setDirtyPaths((current) => new Set(current).add(activeFile.path))
  }, [activeFile])

  /** saveWorkspace 先写回所有修改文件，再调用后端持久化工作区快照。 */
  const saveWorkspace = useCallback(async () => {
    if (saving) return
    setSaving(true)
    setMessage(undefined)
    setActionError(undefined)
    try {
      for (const path of dirtyPaths) {
        await api.sandbox.writeFile(sandboxId, {
          relative_path: path,
          content_base64: encodeBase64(contents[path] || ''),
        })
      }
      const result = await api.sandbox.saveFiles(sandboxId)
      setDirtyPaths(new Set())
      setMessage('工作区已保存。')
      onSaved?.(result)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '工作区保存失败，请稍后重试。'))
    } finally {
      setSaving(false)
    }
  }, [contents, dirtyPaths, onSaved, sandboxId, saving])

  if (resource.status === 'loading') return <ResourceState status="loading" title="正在打开代码工作区" />
  if (resource.status === 'error') return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  if (!resource.data) return <ResourceState status="loading" title="正在打开代码工作区" />

  const workbenchFiles = files.map((file) => ({ ...file, dirty: dirtyPaths.has(file.path) }))
  const toolItems = toWorkbenchTools(resource.data.instance)
  return (
    <IdeWorkbench
      title={title}
      subtitle={subtitle}
      status={<span role="status">{progressMessage ? `${progressMessage.message}${progressMessage.trace_id ? `（编号 ${progressMessage.trace_id}）` : ''}` : progress.status === 'open' ? '环境状态已连接' : '正在连接实验环境'}</span>}
      files={workbenchFiles}
      activeFileId={activeFileId}
      onSelectFile={(file) => void selectFile(file as WorkspaceFile)}
      editor={activeFile ? (
        contents[activeFile.path] === undefined
          ? <ResourceState status="loading" title="正在读取文件" />
          : <MonacoPane key={activeFile.path} file={activeFile} value={contents[activeFile.path]} onChange={updateActiveContent} />
      ) : <p className={styles.empty}>当前工作区没有可编辑文件。</p>}
      terminal={resource.data.instance.capabilities.terminal ? <SandboxTerminal sandboxId={sandboxId} /> : undefined}
      tools={toolItems}
      inspector={(
        <div className={styles.inspector}>
          {message && <Callout variant="success" title="操作完成">{message}</Callout>}
          {actionError && <Callout variant="danger" title="操作未完成">{actionError}</Callout>}
          <SandboxOperations sandboxId={sandboxId} instance={resource.data.instance} />
          {inspector}
        </div>
      )}
      controls={controls}
      actions={(
        <div className={styles.actions}>
          {actions}
          <Button variant="on-dark" size="sm" icon={<RefreshCw size={15} />} onClick={resource.reload}>刷新环境</Button>
          {resource.data.instance.capabilities.file_workspace && <Button variant="primary" size="sm" icon={<Save size={15} />} loading={saving} onClick={() => void saveWorkspace()}>保存代码</Button>}
        </div>
      )}
    />
  )
}

/** parseProgressMessage 校验沙箱进度 WS 的用户向结构,避免把任意帧直接渲染到页面。 */
function parseProgressMessage(data: unknown): SandboxProgressMessage | undefined {
  if (typeof data !== 'string') return undefined
  try {
    const value: unknown = JSON.parse(data)
    if (!value || typeof value !== 'object') return undefined
    const message = value as Record<string, unknown>
    if (typeof message.phase !== 'number' || typeof message.status !== 'number' || typeof message.stage !== 'string' || typeof message.message !== 'string') return undefined
    return {
      phase: message.phase,
      status: message.status,
      stage: message.stage,
      message: message.message,
      trace_id: typeof message.trace_id === 'string' ? message.trace_id : undefined,
    }
  } catch {
    return undefined
  }
}
