import type { EditorMountOptions, MountedEditor } from './types'

const RELEASED_COMMAND = 'editor.action.insertLineAfter'
let keysReleased = false

const monacoThemeTokens = {
  background: '--color-terminal',
  lineHighlight: '--color-dark-surface',
  lineNumber: '--color-on-dark-faint',
  indentGuide: '--color-dark-line',
  suggestBackground: '--color-dark-elevated',
  suggestBorder: '--color-dark-line',
} as const

/**
 * mountMonacoEditor 动态加载 monaco-editor 并挂载到指定容器，避免四端首屏强制加载编辑器资源。
 */
export async function mountMonacoEditor(
  container: HTMLElement,
  options: EditorMountOptions
): Promise<MountedEditor> {
  const { monaco } = await import('./monaco-runtime')
  if (!keysReleased) {
    // 「-命令」是 Monaco 的移除规则:撤下该命令的默认键位,而不是改绑到别的命令上
    monaco.editor.addKeybindingRule({
      keybinding: monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter,
      command: `-${RELEASED_COMMAND}`,
    })
    keysReleased = true
  }
  const editorBackground = cssColor(monacoThemeTokens.background)

  monaco.editor.defineTheme('chaimir-dark', {
    base: 'vs-dark',
    inherit: true,
    rules: [{ token: '', background: colorWithoutHash(editorBackground) }],
    colors: {
      'editor.background': editorBackground,
      'editor.lineHighlightBackground': cssColor(monacoThemeTokens.lineHighlight),
      'editorLineNumber.foreground': cssColor(monacoThemeTokens.lineNumber),
      'editorIndentGuide.background': cssColor(monacoThemeTokens.indentGuide),
      'editorSuggestWidget.background': cssColor(monacoThemeTokens.suggestBackground),
      'editorSuggestWidget.border': cssColor(monacoThemeTokens.suggestBorder),
    },
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
  // 空 lineEnding 沿用模型的换行符，preserveBOM 避免编辑后丢失原文件标记。
  const getValue = () => editor.getValue({ preserveBOM: true, lineEnding: '' })
  const changeSubscription = editor.onDidChangeModelContent(() => options.onChange?.(getValue()))

  return {
    getValue,
    setValue: (value) => {
      if (getValue() !== value) {
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

function cssColor(token: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(token).trim()
  if (!value) {
    throw new Error(`缺少前端颜色令牌 ${token}`)
  }
  return value
}

function colorWithoutHash(color: string): string {
  return color.startsWith('#') ? color.slice(1) : color
}
