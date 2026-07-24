// Monaco 封装：为实验 IDE 提供统一编辑器生命周期和只读/变更事件装配。

import type { EditorMountOptions, MountedEditor } from './types'
import { cssColor } from './theme'

const monacoThemeTokens = {
  background: '--color-terminal-bg',
  lineHighlight: '--color-editor-line-highlight',
  lineNumber: '--color-editor-line-number',
  indentGuide: '--color-editor-indent-guide',
  suggestBackground: '--color-editor-suggest-bg',
  suggestBorder: '--color-editor-suggest-border',
} as const

/**
 * mountMonacoEditor 动态加载 monaco-editor 并挂载到指定容器，避免四端首屏强制加载编辑器资源。
 */
export async function mountMonacoEditor(container: HTMLElement, options: EditorMountOptions): Promise<MountedEditor> {
  const { monaco, prepareMonacoLanguage } = await import('./monaco-runtime')
  await prepareMonacoLanguage(options.language)
  const editorBackground = cssColor(monacoThemeTokens.background)

  monaco.editor.defineTheme('chaimir-dark', {
    base: 'vs-dark',
    inherit: true,
    rules: [
      { token: '', background: colorWithoutHash(editorBackground) }
    ],
    colors: {
      'editor.background': editorBackground,
      'editor.lineHighlightBackground': cssColor(monacoThemeTokens.lineHighlight),
      'editorLineNumber.foreground': cssColor(monacoThemeTokens.lineNumber),
      'editorIndentGuide.background': cssColor(monacoThemeTokens.indentGuide),
      'editorSuggestWidget.background': cssColor(monacoThemeTokens.suggestBackground),
      'editorSuggestWidget.border': cssColor(monacoThemeTokens.suggestBorder),
    }
  })

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
    theme: 'chaimir-dark',
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

/** colorWithoutHash 把 CSS 颜色转换为 Monaco 需要的不带井号格式。 */
function colorWithoutHash(color: string): string {
  return color.startsWith('#') ? color.slice(1) : color
}
