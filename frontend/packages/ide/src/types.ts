export interface DisposableHandle {
  dispose: () => void
}

export interface EditorMountOptions {
  value: string
  language: string
  readOnly?: boolean
  onChange?: (value: string) => void
}

export interface MountedEditor extends DisposableHandle {
  getValue: () => string
  setValue: (value: string) => void
  focus: () => void
}

export interface TerminalMountOptions {
  initialText?: string
  onData?: (data: string) => void
  /** xterm生成的非UTF-8输入按字节透传,不经字符串编码。 */
  onBinary?: (data: ArrayBuffer) => void
}

export interface MountedTerminal extends DisposableHandle {
  write: (data: string | Uint8Array) => void
  clear: () => void
  focus: () => void
  getSize: () => { cols: number; rows: number }
  onResize: (listener: (size: { cols: number; rows: number }) => void) => DisposableHandle
}
