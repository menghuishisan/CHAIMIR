// xterm 封装：为沙箱终端提供统一写入、清屏、输入转发和销毁生命周期。

import '@xterm/xterm/css/xterm.css'
import type { MountedTerminal, TerminalMountOptions } from './types'

/**
 * mountTerminal 动态加载 xterm 并挂载终端，前端只负责渲染和输入转发，不持有后端凭据。
 */
export async function mountTerminal(
  container: HTMLElement,
  options: TerminalMountOptions = {}
): Promise<MountedTerminal> {
  const { Terminal } = await import('@xterm/xterm')
  const { FitAddon } = await import('@xterm/addon-fit')

  const terminal = new Terminal({
    convertEol: true,
    cursorBlink: true,
    fontFamily: 'var(--font-mono)',
    fontSize: 14,
    theme: {
      background: cssColor('--color-terminal'),
      foreground: cssColor('--color-on-dark'),
      cursor: cssColor('--color-accent'),
    },
  })

  const fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)

  terminal.open(container)

  // 切标签隐藏/重新显示与窗口变化均由容器尺寸驱动,不以延时猜测布局完成。
  let disposed = false
  const fit = () => {
    if (!disposed && container.clientWidth > 0 && container.clientHeight > 0) fitAddon.fit()
  }
  const observer = new ResizeObserver(fit)
  observer.observe(container)
  fit()
  if (options.initialText) {
    terminal.write(options.initialText)
  }
  const dataSubscription = terminal.onData((data) => {
    options.onData?.(data)
  })
  // xterm的二进制输入用字符码承载原始字节,不能经WebSocket文本编码再次转码。
  const binarySubscription = terminal.onBinary((data) => {
    const buffer = new ArrayBuffer(data.length)
    const bytes = new Uint8Array(buffer)
    for (let index = 0; index < data.length; index += 1) bytes[index] = data.charCodeAt(index)
    options.onBinary?.(buffer)
  })

  return {
    write: (data) => terminal.write(data),
    clear: () => terminal.clear(),
    focus: () => terminal.focus(),
    resize: () => fitAddon.fit(),
    dispose: () => {
      disposed = true
      observer.disconnect()
      dataSubscription.dispose()
      binarySubscription.dispose()
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
