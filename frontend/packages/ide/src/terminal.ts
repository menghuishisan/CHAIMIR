// xterm 封装：为沙箱终端提供统一写入、清屏、输入转发和销毁生命周期。

import type { MountedTerminal, TerminalMountOptions } from './types'

/**
 * mountTerminal 动态加载 xterm 并挂载终端，前端只负责渲染和输入转发，不持有后端凭据。
 */
export async function mountTerminal(container: HTMLElement, options: TerminalMountOptions = {}): Promise<MountedTerminal> {
  const { Terminal } = await import('@xterm/xterm')
  const terminal = new Terminal({
    convertEol: true,
    cursorBlink: true,
    fontFamily: 'var(--font-mono)',
    fontSize: 14,
    theme: {
      background: 'var(--color-terminal-bg)',
      foreground: 'var(--color-dark-text)',
      cursor: 'var(--color-on-dark-accent)',
    },
  })
  terminal.open(container)
  if (options.initialText) {
    terminal.write(options.initialText)
  }
  const dataSubscription = terminal.onData((data) => {
    options.onData?.(data)
  })

  return {
    write: (data) => terminal.write(data),
    clear: () => terminal.clear(),
    focus: () => terminal.focus(),
    dispose: () => {
      dataSubscription.dispose()
      terminal.dispose()
    },
  }
}
