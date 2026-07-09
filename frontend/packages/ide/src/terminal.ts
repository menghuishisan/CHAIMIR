// xterm 封装：为沙箱终端提供统一写入、清屏、输入转发和销毁生命周期。

import '@xterm/xterm/css/xterm.css'
import type { MountedTerminal, TerminalMountOptions } from './types'

/**
 * mountTerminal 动态加载 xterm 并挂载终端，前端只负责渲染和输入转发，不持有后端凭据。
 */
export async function mountTerminal(container: HTMLElement, options: TerminalMountOptions = {}): Promise<MountedTerminal> {
  const { Terminal } = await import('@xterm/xterm')
  const { FitAddon } = await import('@xterm/addon-fit')

  const terminal = new Terminal({
    convertEol: true,
    cursorBlink: true,
    fontFamily: 'var(--font-mono)',
    fontSize: 14,
    theme: {
      background: cssColor('--color-terminal-bg'),
      foreground: cssColor('--color-dark-text'),
      cursor: cssColor('--color-on-dark-accent'),
    },
  })

  const fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)

  terminal.open(container)

  try {
    const { WebglAddon } = await import('@xterm/addon-webgl')
    const webglAddon = new WebglAddon()
    terminal.loadAddon(webglAddon)
  } catch {
    // WebGL 不可用时回退到 xterm 默认渲染器。
  }

  // 等容器完成首次绘制后再适配终端尺寸。
  setTimeout(() => fitAddon.fit(), 10)
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
    resize: () => fitAddon.fit(),
    dispose: () => {
      dataSubscription.dispose()
      terminal.dispose()
    },
  }
}

/** cssColor 读取设计令牌的计算值，供 xterm 主题配置使用。 */
function cssColor(token: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(token).trim()
  if (!value) {
    throw new Error(`缺少前端颜色令牌 ${token}`)
  }
  return value
}
