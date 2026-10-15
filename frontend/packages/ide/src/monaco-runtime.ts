// Monaco 运行时文件装配同源 AMD 编辑器、必要语言服务与 Worker。

import type * as MonacoNamespace from 'monaco-editor'

type MonacoApi = typeof MonacoNamespace
type AmdModule = Record<string, unknown>
type AmdRequire = {
  (modules: string[], onLoad: (module: AmdModule) => void, onError?: (error: unknown) => void): void
  config: (options: { paths: { vs: string } }) => void
}
type MonacoGlobal = typeof globalThis & {
  require?: AmdRequire
  MonacoEnvironment?: {
    baseUrl: string
    getWorkerUrl: (workerId: string, label: string) => string
  }
}
type LanguageDefaults = Record<string, unknown>
type LanguageModeModule = {
  setupMode?: (defaults: LanguageDefaults) => unknown
  setupTypeScript?: (defaults: LanguageDefaults) => unknown
  setupJavaScript?: (defaults: LanguageDefaults) => unknown
}
type BasicLanguageModule = {
  conf: Parameters<MonacoApi['languages']['setLanguageConfiguration']>[1]
  language: Parameters<MonacoApi['languages']['setMonarchTokensProvider']>[1]
}

const MONACO_BASE_URL = '/assets/monaco'
const loadedLanguages = new Map<string, Promise<void>>()
let amdLoaderPromise: Promise<AmdRequire> | undefined
let monacoStylesheetPromise: Promise<void> | undefined
let monacoPromise: Promise<MonacoApi> | undefined
export let monaco: MonacoApi

/** getAmdRequire 返回 Monaco loader 提供的同源 AMD require,缺失时显式报告运行时契约错误。 */
function getAmdRequire(): AmdRequire {
  const amdRequire = (globalThis as MonacoGlobal).require
  if (!amdRequire) throw new Error('Monaco AMD loader 未完成加载')
  return amdRequire
}

/** loadAmdLoader 只加载一次同源 loader.js,不使用 CDN、内联脚本或 Blob URL。 */
function loadAmdLoader(): Promise<AmdRequire> {
  if (!amdLoaderPromise) {
    amdLoaderPromise = new Promise<AmdRequire>((resolve, reject) => {
      const script = document.createElement('script')
      script.async = true
      script.dataset.chaimirMonacoLoader = 'true'
      script.src = `${MONACO_BASE_URL}/vs/loader.js`
      script.addEventListener('load', () => {
        try {
          resolve(getAmdRequire())
        } catch (error) {
          reject(error)
        }
      }, { once: true })
      script.addEventListener('error', () => {
        reject(new Error(`Monaco loader 加载失败: ${script.src}`))
      }, { once: true })
      document.head.appendChild(script)
    })
  }
  return amdLoaderPromise
}

/** loadMonacoStylesheet 仅在 IDE 首次打开时加载编辑器样式,避免普通页面承担 Monaco 请求。 */
function loadMonacoStylesheet(): Promise<void> {
  if (!monacoStylesheetPromise) {
    monacoStylesheetPromise = new Promise<void>((resolve, reject) => {
      const link = document.createElement('link')
      link.rel = 'stylesheet'
      link.href = `${MONACO_BASE_URL}/vs/editor/editor.main.css`
      link.dataset.chaimirMonacoStylesheet = 'true'
      link.addEventListener('load', () => resolve(), { once: true })
      link.addEventListener('error', () => reject(new Error(`Monaco 样式加载失败: ${link.href}`)), { once: true })
      document.head.appendChild(link)
    })
  }
  return monacoStylesheetPromise
}

/** loadAmdModule 通过封闭的模块 ID 加载预构建 AMD 模块,避免运行时拼接任意路径。 */
async function loadAmdModule<T extends AmdModule>(moduleId: string): Promise<T> {
  const amdRequire = await loadAmdLoader()
  return new Promise<T>((resolve, reject) => {
    amdRequire([moduleId], (module) => resolve(module as T), reject)
  })
}

/** ensureMonaco 初始化同源 Worker 策略并返回编辑器 API。 */
async function ensureMonaco(): Promise<MonacoApi> {
  if (monaco) return monaco
  if (!monacoPromise) {
    monacoPromise = (async () => {
      const [amdRequire] = await Promise.all([loadAmdLoader(), loadMonacoStylesheet()])
      ;(globalThis as MonacoGlobal).MonacoEnvironment = {
        baseUrl: MONACO_BASE_URL,
        getWorkerUrl: () => `${MONACO_BASE_URL}/vs/base/worker/workerMain.js`,
      }
      amdRequire.config({ paths: { vs: `${MONACO_BASE_URL}/vs` } })
      const editorModule = await new Promise<AmdModule>((resolve, reject) => {
        amdRequire(['vs/editor/editor.main'], resolve, reject)
      })
      monaco = editorModule as MonacoApi
      return monaco
    })()
  }
  return monacoPromise
}

/** prepareMonacoLanguage 只加载当前文件需要的语言服务,同一语言在页面生命周期内只加载一次。 */
export async function prepareMonacoLanguage(language: string): Promise<void> {
  await ensureMonaco()
  const normalized = supportedLanguage(language)
  if (normalized === 'plaintext') return
  let pending = loadedLanguages.get(normalized)
  if (!pending) {
    pending = loadLanguageContribution(normalized)
    loadedLanguages.set(normalized, pending)
  }
  await pending
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

/** loadLanguageContribution 注册当前语言并装配对应的预构建语言服务。 */
async function loadLanguageContribution(language: string): Promise<void> {
  switch (language) {
    case 'typescript': {
      const mode = await loadAmdModule<LanguageModeModule>('vs/language/typescript/tsMode')
      registerLanguage({ id: 'typescript', extensions: ['.ts', '.tsx'], aliases: ['TypeScript', 'ts', 'tsx'] })
      registerLanguage({ id: 'javascript', extensions: ['.js', '.jsx'], aliases: ['JavaScript', 'js', 'jsx'] })
      if (!mode.setupTypeScript || !mode.setupJavaScript) throw new Error('TypeScript Monaco 语言服务导出不完整')
      await setupTypeScriptModes(mode)
      return
    }
    case 'javascript': {
      const mode = await loadAmdModule<LanguageModeModule>('vs/language/typescript/tsMode')
      registerLanguage({ id: 'javascript', extensions: ['.js', '.jsx'], aliases: ['JavaScript', 'js', 'jsx'] })
      if (!mode.setupJavaScript) throw new Error('JavaScript Monaco 语言服务导出不完整')
      await setupJavaScriptMode(mode)
      return
    }
    case 'json': {
      const mode = await loadAmdModule<LanguageModeModule>('vs/language/json/jsonMode')
      registerLanguage({ id: 'json', extensions: ['.json'], aliases: ['JSON', 'json'], mimetypes: ['application/json'] })
      if (!mode.setupMode) throw new Error('JSON Monaco 语言服务导出不完整')
      await setupMode(mode, createLanguageDefaults({
        languageId: 'json',
        diagnosticsOptions: { validate: true, allowComments: true, schemas: [], enableSchemaRequest: false, schemaRequest: 'warning', schemaValidation: 'warning', comments: 'error', trailingCommas: 'error' },
        modeConfiguration: { documentFormattingEdits: true, documentRangeFormattingEdits: true, completionItems: true, hovers: true, documentSymbols: true, tokens: true, colors: true, foldingRanges: true, diagnostics: true, selectionRanges: true },
      }))
      return
    }
    case 'css': {
      const mode = await loadAmdModule<LanguageModeModule>('vs/language/css/cssMode')
      registerLanguage({ id: 'css', extensions: ['.css'], aliases: ['CSS', 'css'], mimetypes: ['text/css'] })
      if (!mode.setupMode) throw new Error('CSS Monaco 语言服务导出不完整')
      await setupMode(mode, createLanguageDefaults({
        languageId: 'css',
        options: { validate: true, lint: {}, data: { useDefaultDataProvider: true } },
        modeConfiguration: { completionItems: true, hovers: true, documentSymbols: true, definitions: true, references: true, documentHighlights: true, rename: true, colors: true, foldingRanges: true, diagnostics: true, selectionRanges: true, documentFormattingEdits: true, documentRangeFormattingEdits: true },
      }))
      return
    }
    case 'html': {
      const mode = await loadAmdModule<LanguageModeModule>('vs/language/html/htmlMode')
      registerLanguage({ id: 'html', extensions: ['.html', '.htm'], aliases: ['HTML', 'html'], mimetypes: ['text/html'] })
      if (!mode.setupMode) throw new Error('HTML Monaco 语言服务导出不完整')
      await setupMode(mode, createLanguageDefaults({
        languageId: 'html',
        options: { format: { tabSize: 4, insertSpaces: false, wrapLineLength: 120, preserveNewLines: true, indentInnerHtml: false, indentHandlebars: false, endWithNewline: false, extraLiners: 'head, body, /html', wrapAttributes: 'auto' }, suggest: {}, data: { useDefaultDataProvider: true } },
        modeConfiguration: { completionItems: true, hovers: true, documentSymbols: true, links: true, documentHighlights: true, rename: true, colors: true, foldingRanges: true, selectionRanges: true, diagnostics: true, documentFormattingEdits: true, documentRangeFormattingEdits: true },
      }))
      return
    }
    case 'go':
      await loadBasicLanguage('go', 'vs/basic-languages/go/go', ['.go'], ['Go'])
      return
    case 'sol':
      await loadBasicLanguage('sol', 'vs/basic-languages/solidity/solidity', ['.sol'], ['Solidity', 'sol'])
      return
    case 'python':
      await loadBasicLanguage('python', 'vs/basic-languages/python/python', ['.py'], ['Python', 'py'])
      return
    case 'markdown':
      await loadBasicLanguage('markdown', 'vs/basic-languages/markdown/markdown', ['.md', '.markdown'], ['Markdown', 'md'])
      return
    case 'yaml':
      await loadBasicLanguage('yaml', 'vs/basic-languages/yaml/yaml', ['.yaml', '.yml'], ['YAML', 'yaml'])
      return
    case 'shell':
      await loadBasicLanguage('shell', 'vs/basic-languages/shell/shell', ['.sh', '.bash'], ['Shell', 'shell'])
      return
    default:
      return
  }
}

/** registerLanguage 只注册一次语言扩展点,保证语言装配幂等。 */
function registerLanguage(language: Parameters<MonacoApi['languages']['register']>[0]): void {
  if (!monaco.languages.getLanguages().some((item) => item.id === language.id)) monaco.languages.register(language)
}

/** setupMode 装配 CSS、HTML、JSON 等语言服务并保持 Monaco 的异步错误可见。 */
async function setupMode(mode: LanguageModeModule, defaults: LanguageDefaults): Promise<void> {
  if (!mode.setupMode) throw new Error('Monaco 语言服务缺少 setupMode')
  await Promise.resolve(mode.setupMode(defaults))
}

/** createLanguageDefaults 为 AMD 语言服务提供与官方默认对象一致的变更事件契约。 */
function createLanguageDefaults(defaults: LanguageDefaults): LanguageDefaults {
  const emitter = new monaco.Emitter<void>()
  return { ...defaults, onDidChange: emitter.event }
}

/** setupTypeScriptModes 装配 TypeScript 与 JavaScript 的默认语言服务。 */
async function setupTypeScriptModes(mode: LanguageModeModule): Promise<void> {
  const defaults = createTypeScriptDefaults()
  await Promise.resolve(mode.setupTypeScript?.(defaults))
  await Promise.resolve(mode.setupJavaScript?.({ ...defaults, languageId: 'javascript' }))
}

/** setupJavaScriptMode 装配 JavaScript 的默认语言服务。 */
async function setupJavaScriptMode(mode: LanguageModeModule): Promise<void> {
  await Promise.resolve(mode.setupJavaScript?.({ ...createTypeScriptDefaults(), languageId: 'javascript' }))
}

/** createTypeScriptDefaults 提供与 Monaco 官方默认贡献一致的最小只读配置对象。 */
function createTypeScriptDefaults(): LanguageDefaults {
  const emitter = new monaco.Emitter<void>()
  const extraLibs = Object.create(null) as Record<string, unknown>
  let eagerModelSync = false
  return {
    onDidChange: emitter.event,
    onDidExtraLibsChange: emitter.event,
    getCompilerOptions: () => ({ allowNonTsExtensions: true, target: 99 }),
    getDiagnosticsOptions: () => ({ noSemanticValidation: false, noSyntaxValidation: false, onlyVisible: false }),
    getWorkerOptions: () => ({}),
    getInlayHintsOptions: () => ({}),
    getExtraLibs: () => extraLibs,
    getEagerModelSync: () => eagerModelSync,
    setEagerModelSync: (value: boolean) => { eagerModelSync = value },
    setMaximumWorkerIdleTime: () => undefined,
    languageId: 'typescript',
    modeConfiguration: { completionItems: true, hovers: true, documentSymbols: true, definitions: true, references: true, documentHighlights: true, rename: true, diagnostics: true, documentRangeFormattingEdits: true, signatureHelp: true, onTypeFormattingEdits: true, codeActions: true, inlayHints: true },
    compilerOptions: { allowNonTsExtensions: true, target: 99 },
    diagnosticsOptions: { noSemanticValidation: false, noSyntaxValidation: false, onlyVisible: false },
    workerOptions: {},
    inlayHintsOptions: {},
  }
}

/** loadBasicLanguage 装配无需独立 Worker 的 Monarch 语言,保留配置和语法高亮行为。 */
async function loadBasicLanguage(id: string, moduleId: string, extensions: string[], aliases: string[]): Promise<void> {
  const language = await loadAmdModule<BasicLanguageModule>(moduleId)
  registerLanguage({ id, extensions, aliases })
  monaco.languages.setMonarchTokensProvider(id, language.language)
  monaco.languages.setLanguageConfiguration(id, language.conf)
}
