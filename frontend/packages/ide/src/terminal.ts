// xterm 封装：为沙箱终端提供统一写入、清屏、输入转发和销毁生命周期。

import '@xterm/xterm/css/xterm.css'
import type { MountedTerminal, TerminalMountOptions } from './types'
import { cssColor } from './theme'

/**
 * mountTerminal 动态加载 xterm 并挂载终端，前端只负责渲染和输入转发，不持有后端凭据。
 */
export async function mountTerminal(container: HTMLElement, options: TerminalMountOptions = {}): Promise<MountedTerminal> {
  const { Terminal } = await import('@xterm/xterm')
  const { FitAddon } = await import('@xterm/addon-fit')
  const reducedMotion = prefersReducedMotion()

  const terminal = new Terminal({
    convertEol: true,
    cursorBlink: !reducedMotion,
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
  const initialFitTimer = window.setTimeout(() => fitAddon.fit(), 10)
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
      window.clearTimeout(initialFitTimer)
      dataSubscription.dispose()
      terminal.dispose()
    },
  }
}

/** prefersReducedMotion 读取系统动效偏好，避免终端光标闪动成为装饰性持续动画。 */
function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}
