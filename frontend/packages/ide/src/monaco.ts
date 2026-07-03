// Monaco 封装：为实验 IDE 提供统一编辑器生命周期和只读/变更事件装配。

import type { EditorMountOptions, MountedEditor } from './types'

/**
 * mountMonacoEditor 动态加载 monaco-editor 并挂载到指定容器，避免四端首屏强制加载编辑器资源。
 */
export async function mountMonacoEditor(container: HTMLElement, options: EditorMountOptions): Promise<MountedEditor> {
  const monaco = await import('monaco-editor')
  const editor = monaco.editor.create(container, {
    value: options.value,
    language: options.language,
    readOnly: options.readOnly ?? false,
    automaticLayout: true,
    minimap: { enabled: false },
    fontFamily: 'var(--font-mono)',
    fontSize: 14,
    tabSize: 2,
    scrollBeyondLastLine: false,
    theme: 'vs-dark',
  })
  const changeSubscription = editor.onDidChangeModelContent(() => {
    options.onChange?.(editor.getValue())
  })

  return {
    getValue: () => editor.getValue(),
    setValue: (value) => {
      if (editor.getValue() !== value) {
        editor.setValue(value)
      }
    },
    focus: () => editor.focus(),
    dispose: () => {
      changeSubscription.dispose()
      editor.dispose()
    },
  }
}
