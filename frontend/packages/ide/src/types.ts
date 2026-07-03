// IDE 公共类型：描述编辑器和终端的受控生命周期。

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
}

export interface MountedTerminal extends DisposableHandle {
  write: (data: string) => void
  clear: () => void
  focus: () => void
}
