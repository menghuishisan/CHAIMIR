// WorkbenchShell 组件：提供沉浸式工具的深色壳、三栏槽位和底部控制区。

import React from 'react'
import { clsx } from 'clsx'
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
  return (
    <main className={clsx('chaimir-workbench-shell', className)} {...props}>
      <header className="chaimir-workbench-shell__bar">
        <div className="chaimir-workbench-shell__title">
          {eyebrow && <p>{eyebrow}</p>}
          <h1>{title}</h1>
        </div>
        {status && <div className="chaimir-workbench-shell__status">{status}</div>}
        {actions && <div className="chaimir-workbench-shell__actions">{actions}</div>}
      </header>
      <section className={clsx('chaimir-workbench-shell__layout', leftPanel && 'has-left', rightPanel && 'has-right')}>
        {leftPanel && <aside className="chaimir-workbench-shell__panel is-left">{leftPanel}</aside>}
        <div className="chaimir-workbench-shell__stage">{children}</div>
        {rightPanel && <aside className="chaimir-workbench-shell__panel is-right">{rightPanel}</aside>}
      </section>
      {controls && <footer className="chaimir-workbench-shell__controls">{controls}</footer>}
    </main>
  )
}
