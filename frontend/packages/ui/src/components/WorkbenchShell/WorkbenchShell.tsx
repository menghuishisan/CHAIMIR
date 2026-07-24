// WorkbenchShell 组件：提供沉浸式工具的深色壳、三栏槽位和底部控制区。

import React, { useEffect, useState } from 'react'
import { clsx } from 'clsx'
import { PanelLeft, PanelRight, X } from 'lucide-react'
import { breakpoints } from '../../tokens'
import { useMediaQuery } from '../../hooks/useMediaQuery'
import './WorkbenchShell.css'

export interface WorkbenchShellProps extends Omit<React.HTMLAttributes<HTMLElement>, 'title'> {
  title: React.ReactNode
  eyebrow?: React.ReactNode
  status?: React.ReactNode
  actions?: React.ReactNode
  leftPanel?: React.ReactNode
  rightPanel?: React.ReactNode
  controls?: React.ReactNode
  children: React.ReactNode
}

/**
 * WorkbenchShell 统一沉浸式工具的框架，但具体 IDE、仿真、答题内容由调用方组合。
 */
export function WorkbenchShell({
  title,
  eyebrow,
  status,
  actions,
  leftPanel,
  rightPanel,
  controls,
  children,
  className,
  ...props
}: WorkbenchShellProps): React.ReactElement {
  const hasLeftPanel = Boolean(leftPanel)
  const hasRightPanel = Boolean(rightPanel)
  const isCompact = useMediaQuery(`(max-width: ${breakpoints.lg - 1}px)`)
  const [leftOpen, setLeftOpen] = useState(() => hasLeftPanel && !isCompactWorkbench())
  const [rightOpen, setRightOpen] = useState(() => hasRightPanel && !isCompactWorkbench())

  useEffect(() => {
    setLeftOpen(hasLeftPanel && !isCompact)
    setRightOpen(hasRightPanel && !isCompact)
  }, [hasLeftPanel, hasRightPanel, isCompact])

  /** toggleLeftPanel 切换文件侧栏，窄屏时同时关闭另一侧面板。 */
  const toggleLeftPanel = (): void => {
    setLeftOpen((open) => !open)
    if (isCompact) setRightOpen(false)
  }

  /** toggleRightPanel 切换检查器，窄屏时同时关闭另一侧面板。 */
  const toggleRightPanel = (): void => {
    setRightOpen((open) => !open)
    if (isCompact) setLeftOpen(false)
  }

  /** closeCompactPanels 关闭窄屏覆盖面板并归还舞台焦点空间。 */
  const closeCompactPanels = (): void => {
    setLeftOpen(false)
    setRightOpen(false)
  }

  return (
    <main className={clsx('chaimir-workbench-shell', className)} {...props}>
      <header className="chaimir-workbench-shell__bar">
        <div className="chaimir-workbench-shell__title">
          {eyebrow && <p>{eyebrow}</p>}
          <h1>{title}</h1>
        </div>
        {status && <div className="chaimir-workbench-shell__status">{status}</div>}
        <div className="chaimir-workbench-shell__actions">
          {leftPanel && (
            <button type="button" className="chaimir-workbench-shell__panel-toggle" aria-expanded={leftOpen} aria-label={leftOpen ? '收起左侧面板' : '展开左侧面板'} onClick={toggleLeftPanel}>
              {leftOpen ? <X size={18} /> : <PanelLeft size={18} />}
            </button>
          )}
          {rightPanel && (
            <button type="button" className="chaimir-workbench-shell__panel-toggle" aria-expanded={rightOpen} aria-label={rightOpen ? '收起右侧面板' : '展开右侧面板'} onClick={toggleRightPanel}>
              {rightOpen ? <X size={18} /> : <PanelRight size={18} />}
            </button>
          )}
          {actions}
        </div>
      </header>
      <section className={clsx('chaimir-workbench-shell__layout', leftOpen && 'has-left', rightOpen && 'has-right')}>
        {(leftOpen || rightOpen) && <button type="button" className="chaimir-workbench-shell__scrim" aria-label="关闭侧边面板" onClick={closeCompactPanels} />}
        {leftPanel && leftOpen && <aside className="chaimir-workbench-shell__panel is-left">{leftPanel}</aside>}
        <div className="chaimir-workbench-shell__stage">{children}</div>
        {rightPanel && rightOpen && <aside className="chaimir-workbench-shell__panel is-right">{rightPanel}</aside>}
      </section>
      {controls && <footer className="chaimir-workbench-shell__controls">{controls}</footer>}
    </main>
  )
}

/** isCompactWorkbench 读取共享断点，决定侧栏使用固定栏还是覆盖层。 */
function isCompactWorkbench(): boolean {
  return typeof window !== 'undefined' && window.matchMedia(`(max-width: ${breakpoints.lg - 1}px)`).matches
}
