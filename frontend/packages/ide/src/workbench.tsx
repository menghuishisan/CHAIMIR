// IDE 工作台组件：组合文件区、编辑器区、终端区和工具状态，不绑定具体实验业务。

import React from 'react'
import { WorkbenchShell } from '@chaimir/ui'
import './workbench.css'

export interface IdeWorkbenchFile {
  id: string
  name: string
  path?: string
  language?: string
  dirty?: boolean
  readOnly?: boolean
}

export interface IdeWorkbenchTool {
  id: string
  label: string
  detail?: string
  status?: 'idle' | 'running' | 'ready' | 'failed'
  action?: React.ReactNode
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
  onSelectFile?: (file: IdeWorkbenchFile) => void
}

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
  onSelectFile,
  className,
  ...props
}: IdeWorkbenchProps): React.ReactElement {
  const activeFile = files.find((file) => file.id === activeFileId)

  return (
    <WorkbenchShell
      className={className}
      title={title}
      eyebrow={subtitle}
      status={status}
      actions={actions}
      controls={controls}
      leftPanel={<IdeFilePanel files={files} activeFileId={activeFileId} onSelectFile={onSelectFile} />}
      rightPanel={<IdeToolPanel tools={tools} inspector={inspector} />}
      {...props}
    >
      <section className="chaimir-ide-workbench" aria-label="代码实验工作区">
        <header className="chaimir-ide-workbench__editor-head">
          <div>
            <span>当前文件</span>
            <strong>{activeFile?.name ?? '请选择文件'}</strong>
          </div>
          {activeFile?.path && <code>{activeFile.path}</code>}
        </header>
        <div className="chaimir-ide-workbench__editor">{editor}</div>
        <section className="chaimir-ide-workbench__terminal" aria-label="终端输出">
          <header>
            <span>终端</span>
            <small>运行输出和命令输入</small>
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
  onSelectFile,
}: {
  files: IdeWorkbenchFile[]
  activeFileId?: string
  onSelectFile?: (file: IdeWorkbenchFile) => void
}): React.ReactElement {
  return (
    <nav className="chaimir-ide-file-panel" aria-label="文件列表">
      <header>
        <span>文件</span>
        <small>{files.length} 个文件</small>
      </header>
      <div className="chaimir-ide-file-panel__list">
        {files.map((file) => {
          const selected = file.id === activeFileId
          return (
            <button
              key={file.id}
              type="button"
              className={classNames('chaimir-ide-file', selected && 'is-selected')}
              aria-current={selected ? 'page' : undefined}
              onClick={() => onSelectFile?.(file)}
            >
              <span>{file.name}</span>
              <small>
                {file.language ?? '文本'}
                {file.readOnly ? ' · 只读' : ''}
                {file.dirty ? ' · 未保存' : ''}
              </small>
            </button>
          )
        })}
      </div>
    </nav>
  )
}

/**
 * IdeToolPanel 渲染工具状态与可选检查面板，供实验、判分或链操作复用。
 */
function IdeToolPanel({ tools, inspector }: { tools: IdeWorkbenchTool[]; inspector?: React.ReactNode }): React.ReactElement {
  return (
    <aside className="chaimir-ide-tool-panel" aria-label="工具状态">
      <section>
        <header>
          <span>工具</span>
          <small>{tools.length > 0 ? '已配置' : '暂无工具'}</small>
        </header>
        <div className="chaimir-ide-tool-panel__list">
          {tools.length > 0 ? (
            tools.map((tool) => (
              <article className={classNames('chaimir-ide-tool', tool.status && `is-${tool.status}`)} key={tool.id}>
                <div>
                  <strong>{tool.label}</strong>
                  {tool.detail && <p>{tool.detail}</p>}
                </div>
                {tool.status && <span>{toolStatusLabel(tool.status)}</span>}
                {tool.action && <div className="chaimir-ide-tool__action">{tool.action}</div>}
              </article>
            ))
          ) : (
            <p className="chaimir-ide-tool-panel__empty">当前实验没有额外工具。</p>
          )}
        </div>
      </section>
      {inspector && <section className="chaimir-ide-tool-panel__inspector">{inspector}</section>}
    </aside>
  )
}

/**
 * toolStatusLabel 将工具状态转成用户可读文案，避免暴露底层运行术语。
 */
function toolStatusLabel(status: NonNullable<IdeWorkbenchTool['status']>): string {
  const labels = {
    idle: '等待',
    running: '运行中',
    ready: '可用',
    failed: '需处理',
  }
  return labels[status]
}

/**
 * classNames 合并可选类名，避免为 IDE 包引入额外样式工具。
 */
function classNames(...items: Array<string | false | undefined>): string {
  return items.filter(Boolean).join(' ')
}
