// Monaco 运行时文件装配编辑器核心、必要交互贡献、Worker 与按语言加载边界。

import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import EditorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import CSSWorker from 'monaco-editor/esm/vs/language/css/css.worker?worker'
import HTMLWorker from 'monaco-editor/esm/vs/language/html/html.worker?worker'
import JSONWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker'
import TypeScriptWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker?worker'

type MonacoWorkerEnvironment = typeof globalThis & {
  MonacoEnvironment?: {
    getWorker: (moduleId: string, label: string) => Worker
  }
}

const loadedLanguages = new Map<string, Promise<unknown>>()
let editorContributions: Promise<unknown[]> | undefined

;(globalThis as MonacoWorkerEnvironment).MonacoEnvironment = {
  getWorker: (_moduleId, label) => {
    if (label === 'json') return new JSONWorker()
    if (label === 'css' || label === 'scss' || label === 'less') return new CSSWorker()
    if (label === 'html' || label === 'handlebars' || label === 'razor') return new HTMLWorker()
    if (label === 'typescript' || label === 'javascript') return new TypeScriptWorker()
    return new EditorWorker()
  },
}

/** prepareMonacoLanguage 只加载当前文件需要的语言贡献,同一语言在页面生命周期内只加载一次。 */
export async function prepareMonacoLanguage(language: string): Promise<void> {
  await prepareEditorContributions()
  const normalized = supportedLanguage(language)
  if (normalized === 'plaintext') return
  let pending = loadedLanguages.get(normalized)
  if (!pending) {
    pending = loadLanguageContribution(normalized)
    loadedLanguages.set(normalized, pending)
  }
  await pending
}

/** prepareEditorContributions 按需并行加载 IDE 使用的编辑、查找、提示和格式化能力。 */
function prepareEditorContributions(): Promise<unknown[]> {
  if (!editorContributions) {
    editorContributions = Promise.all([
      import('monaco-editor/esm/vs/editor/contrib/bracketMatching/browser/bracketMatching'),
      import('monaco-editor/esm/vs/editor/contrib/find/browser/findController'),
      import('monaco-editor/esm/vs/editor/contrib/folding/browser/folding'),
      import('monaco-editor/esm/vs/editor/contrib/format/browser/formatActions'),
      import('monaco-editor/esm/vs/editor/contrib/hover/browser/hoverContribution'),
      import('monaco-editor/esm/vs/editor/contrib/suggest/browser/suggestController'),
    ])
  }
  return editorContributions
}

/** supportedLanguage 把未知语言收敛为纯文本,避免动态拼接模块路径。 */
function supportedLanguage(language: string): string {
  switch (language) {
    case 'typescript':
    case 'javascript':
    case 'json':
    case 'css':
    case 'html':
    case 'go':
    case 'sol':
    case 'python':
    case 'markdown':
    case 'yaml':
    case 'shell':
      return language
    default:
      return 'plaintext'
  }
}

/** loadLanguageContribution 使用封闭映射生成独立语言块,不引入 Monaco 全语言入口。 */
function loadLanguageContribution(language: string): Promise<unknown> {
  switch (language) {
    case 'typescript':
    case 'javascript':
      return import('monaco-editor/esm/vs/language/typescript/monaco.contribution')
    case 'json':
      return import('monaco-editor/esm/vs/language/json/monaco.contribution')
    case 'css':
      return import('monaco-editor/esm/vs/language/css/monaco.contribution')
    case 'html':
      return import('monaco-editor/esm/vs/language/html/monaco.contribution')
    case 'go':
      return import('monaco-editor/esm/vs/basic-languages/go/go.contribution')
    case 'sol':
      return import('monaco-editor/esm/vs/basic-languages/solidity/solidity.contribution')
    case 'python':
      return import('monaco-editor/esm/vs/basic-languages/python/python.contribution')
    case 'markdown':
      return import('monaco-editor/esm/vs/basic-languages/markdown/markdown.contribution')
    case 'yaml':
      return import('monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution')
    case 'shell':
      return import('monaco-editor/esm/vs/basic-languages/shell/shell.contribution')
    default:
      return Promise.resolve()
  }
}

export { monaco }
