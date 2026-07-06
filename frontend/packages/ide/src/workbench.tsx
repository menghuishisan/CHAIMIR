// IDE 工作台组件：组合文件区、编辑器区、终端区和工具状态，不绑定具体实验业务。

import React from 'react'
import { Empty, PanelHeader, StatusIndicator, WorkbenchShell, triggerHaptic } from '@chaimir/ui'
import './workbench.css'

export interface IdeWorkbenchFile {
  id: string
  name: string
  path?: string
  language?: string
  dirty?: boolean
  readOnly?: boolean
}

export type IdeWorkbenchToolKind = 'platform-builtin' | 'terminal' | 'web-embed' | 'command-tool'
export type IdeWorkbenchToolStatus = 'idle' | 'running' | 'ready' | 'failed'

export interface IdeWorkbenchTool {
  id: string
  label: string
  detail?: string
  kind?: IdeWorkbenchToolKind
  status?: IdeWorkbenchToolStatus
  action?: React.ReactNode
}

export interface IdeWorkbenchLabels {
  workspace: string
  currentFile: string
  noFileSelected: string
  filePanel: string
  fileCount: (count: number) => string
  emptyFiles: string
  defaultLanguage: string
  readOnly: string
  dirty: string
  terminal: string
  terminalDetail: string
  toolPanel: string
  toolConfigured: string
  toolEmptyState: string
  inspector: string
  toolStatus: Record<IdeWorkbenchToolStatus, string>
  toolKind: Record<IdeWorkbenchToolKind, string>
}

export type IdeWorkbenchLabelOverrides = Partial<Omit<IdeWorkbenchLabels, 'toolStatus' | 'toolKind'>> & {
  toolStatus?: Partial<Record<IdeWorkbenchToolStatus, string>>
  toolKind?: Partial<Record<IdeWorkbenchToolKind, string>>
}

export interface IdeWorkbenchProps extends Omit<React.HTMLAttributes<HTMLElement>, 'title'> {
  title: React.ReactNode
  subtitle?: React.ReactNode
  status?: React.ReactNode
  files: IdeWorkbenchFile[]
  activeFileId?: string
  editor: React.ReactNode
  terminal: React.ReactNode
  tools?: IdeWorkbenchTool[]
  inspector?: React.ReactNode
  controls?: React.ReactNode
  actions?: React.ReactNode
  labels?: IdeWorkbenchLabelOverrides
  onSelectFile?: (file: IdeWorkbenchFile) => void
}

const defaultLabels: IdeWorkbenchLabels = {
  workspace: '代码实验工作区',
  currentFile: '当前文件',
  noFileSelected: '请选择文件',
  filePanel: '文件列表',
  fileCount: (count) => `${count} 个文件`,
  emptyFiles: '当前实验还没有可编辑文件。',
  defaultLanguage: '文本',
  readOnly: '只读',
  dirty: '未保存',
  terminal: '终端',
  terminalDetail: '运行输出和命令输入',
  toolPanel: '工具状态',
  toolConfigured: '已配置',
  toolEmptyState: '当前实验没有额外工具。',
  inspector: '检查面板',
  toolStatus: {
    idle: '等待',
    running: '运行中',
    ready: '可用',
    failed: '需处理',
  },
  toolKind: {
    'platform-builtin': '平台内置',
    terminal: '终端',
    'web-embed': 'Web 工具',
    'command-tool': '命令工具',
  },
}

const MIN_TERMINAL_HEIGHT = 140
const DEFAULT_TERMINAL_HEIGHT = 280
const TERMINAL_RESIZE_STEP = 24

/**
 * IdeWorkbench 提供沉浸式代码实验 UI 骨架，真实文件、终端和运行状态由调用方注入。
 */
export function IdeWorkbench({
  title,
  subtitle,
  status,
  files,
  activeFileId,
  editor,
  terminal,
  tools = [],
  inspector,
  controls,
  actions,
  labels,
  onSelectFile,
  className,
  ...props
}: IdeWorkbenchProps): React.ReactElement {
  const activeFile = files.find((file) => file.id === activeFileId)
  const resolvedLabels = mergeLabels(labels)

  const [terminalHeight, setTerminalHeight] = React.useState(DEFAULT_TERMINAL_HEIGHT)
  const [isDragging, setIsDragging] = React.useState(false)
  const dragStartY = React.useRef(0)
  const startTerminalHeight = React.useRef(0)
  const terminalMaxHeight = Math.round(maxTerminalHeight())

  const resizeTerminalHeight = React.useCallback((nextHeight: number) => {
    setTerminalHeight(clampTerminalHeight(nextHeight))
  }, [])

  const handlePointerDown = (e: React.PointerEvent) => {
    setIsDragging(true)
    dragStartY.current = e.clientY
    startTerminalHeight.current = terminalHeight
    e.currentTarget.setPointerCapture(e.pointerId)
    triggerHaptic()
  }

  const handlePointerMove = (e: React.PointerEvent) => {
    if (!isDragging) return
    const deltaY = e.clientY - dragStartY.current
    resizeTerminalHeight(startTerminalHeight.current - deltaY)
  }

  const handlePointerUp = (e: React.PointerEvent) => {
    setIsDragging(false)
    e.currentTarget.releasePointerCapture(e.pointerId)
    notifyWorkbenchResize()
  }

  const handleResizerKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      resizeTerminalHeight(terminalHeight + TERMINAL_RESIZE_STEP)
      notifyWorkbenchResize()
      return
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      resizeTerminalHeight(terminalHeight - TERMINAL_RESIZE_STEP)
      notifyWorkbenchResize()
      return
    }
    if (e.key === 'Home') {
      e.preventDefault()
      resizeTerminalHeight(MIN_TERMINAL_HEIGHT)
      notifyWorkbenchResize()
      return
    }
    if (e.key === 'End') {
      e.preventDefault()
      resizeTerminalHeight(maxTerminalHeight())
      notifyWorkbenchResize()
    }
  }

  return (
    <WorkbenchShell
      className={className}
      title={title}
      eyebrow={subtitle}
      status={status}
      actions={actions}
      controls={controls}
      leftPanel={<IdeFilePanel files={files} activeFileId={activeFileId} labels={resolvedLabels} onSelectFile={onSelectFile} />}
      rightPanel={<IdeToolPanel tools={tools} inspector={inspector} labels={resolvedLabels} />}
      {...props}
    >
      <section className="chaimir-ide-workbench" aria-label={resolvedLabels.workspace}>
        <header className="chaimir-ide-workbench__editor-head">
          <PanelHeader
            compact
            eyebrow={resolvedLabels.currentFile}
            title={activeFile?.name ?? resolvedLabels.noFileSelected}
          />
          {activeFile?.path && <code>{activeFile.path}</code>}
        </header>
        <div className="chaimir-ide-workbench__editor">{editor}</div>

        {/* 终端高度调整手柄 */}
        <div
          className={classNames('chaimir-ide-workbench__resizer', isDragging && 'is-dragging')}
          role="separator"
          aria-label="调整终端高度"
          aria-orientation="horizontal"
          aria-valuemin={MIN_TERMINAL_HEIGHT}
          aria-valuemax={terminalMaxHeight}
          aria-valuenow={Math.round(terminalHeight)}
          tabIndex={0}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerUp}
          onKeyDown={handleResizerKeyDown}
        />

        <section className="chaimir-ide-workbench__terminal" aria-label={resolvedLabels.terminal} style={{ height: `${terminalHeight}px` }}>
          <header>
            <PanelHeader compact title={resolvedLabels.terminal} description={resolvedLabels.terminalDetail} />
          </header>
          <div>{terminal}</div>
        </section>
      </section>
    </WorkbenchShell>
  )
}

/**
 * IdeFilePanel 渲染文件列表并保留未保存、只读等状态文字。
 */
function IdeFilePanel({
  files,
  activeFileId,
  labels,
  onSelectFile,
}: {
  files: IdeWorkbenchFile[]
  activeFileId?: string
  labels: IdeWorkbenchLabels
  onSelectFile?: (file: IdeWorkbenchFile) => void
}): React.ReactElement {
  return (
    <nav className="chaimir-ide-file-panel" aria-label={labels.filePanel}>
      <header>
        <PanelHeader compact title={labels.filePanel} meta={labels.fileCount(files.length)} />
      </header>
      <div className="chaimir-ide-file-panel__list">
        {files.length > 0 ? (
          files.map((file) => {
            const selected = file.id === activeFileId
            return (
              <button
                key={file.id}
                type="button"
                className={classNames('chaimir-ide-file', selected && 'is-selected')}
                aria-current={selected ? 'page' : undefined}
                onClick={() => {
                  triggerHaptic()
                  onSelectFile?.(file)
                }}
              >
                <span>{file.name}</span>
                <small className="chaimir-ide-file__meta">
                  <span>{file.language ?? labels.defaultLanguage}</span>
                  {file.readOnly && <span>{labels.readOnly}</span>}
                  {file.dirty && <span>{labels.dirty}</span>}
                </small>
              </button>
            )
          })
        ) : (
          <Empty title={labels.emptyFiles} />
        )}
      </div>
    </nav>
  )
}

/**
 * IdeToolPanel 渲染工具状态与可选检查面板，供实验、判分或链操作复用。
 */
function IdeToolPanel({
  tools,
  inspector,
  labels,
}: {
  tools: IdeWorkbenchTool[]
  inspector?: React.ReactNode
  labels: IdeWorkbenchLabels
}): React.ReactElement {
  return (
    <aside className="chaimir-ide-tool-panel" aria-label={labels.toolPanel}>
      <section>
        <header>
          <PanelHeader compact title={labels.toolPanel} meta={tools.length > 0 ? labels.toolConfigured : labels.toolEmptyState} />
        </header>
        <div className="chaimir-ide-tool-panel__list">
          {tools.length > 0 ? (
            tools.map((tool) => (
              <article className={classNames('chaimir-ide-tool', tool.status && `is-${tool.status}`)} key={tool.id}>
                <div>
                  <strong>{tool.label}</strong>
                  {tool.detail && <p>{tool.detail}</p>}
                  {tool.kind && <small>{labels.toolKind[tool.kind]}</small>}
                </div>
                {tool.status && <StatusIndicator label={labels.toolStatus[tool.status]} tone={toolStatusTone(tool.status)} pulse={tool.status === 'running'} />}
                {tool.action && <div className="chaimir-ide-tool__action">{tool.action}</div>}
              </article>
            ))
          ) : (
            <Empty title={labels.toolEmptyState} />
          )}
        </div>
      </section>
      {inspector && (
        <section className="chaimir-ide-tool-panel__inspector" aria-label={labels.inspector}>
          {inspector}
        </section>
      )}
    </aside>
  )
}

/**
 * toolStatusTone 将 IDE 工具状态映射到 UI 层状态语义。
 */
function toolStatusTone(status: IdeWorkbenchToolStatus): 'neutral' | 'success' | 'warning' | 'danger' | 'primary' {
  if (status === 'ready') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'primary'
  return 'warning'
}

/**
 * mergeLabels 保留默认实验工作台文案，同时允许业务页按实验语境覆盖。
 */
function mergeLabels(labels: IdeWorkbenchLabelOverrides | undefined): IdeWorkbenchLabels {
  return {
    ...defaultLabels,
    ...labels,
    toolStatus: {
      ...defaultLabels.toolStatus,
      ...labels?.toolStatus,
    },
    toolKind: {
      ...defaultLabels.toolKind,
      ...labels?.toolKind,
    },
  }
}

/**
 * classNames 合并可选类名，避免为 IDE 包引入额外样式工具。
 */
function classNames(...items: Array<string | false | undefined>): string {
  return items.filter(Boolean).join(' ')
}

function maxTerminalHeight(): number {
  if (typeof window === 'undefined') {
    return 720
  }
  return Math.max(MIN_TERMINAL_HEIGHT, window.innerHeight * 0.7)
}

function clampTerminalHeight(height: number): number {
  return Math.min(Math.max(height, MIN_TERMINAL_HEIGHT), maxTerminalHeight())
}

function notifyWorkbenchResize(): void {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event('resize'))
  }
}
