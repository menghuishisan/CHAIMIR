// vite.config.ts 配置统一 Web 应用的构建环境、源码别名和本地开发服务。
import { defineConfig } from 'vite'
import type { Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import fs from 'node:fs'
import path from 'path'

type FontAssetDefinition = {
  name: string
  packagePath: string
  stylesheet: string
}

const fontAssetDefinitions: FontAssetDefinition[] = [
  {
    name: 'noto-sans-sc',
    packagePath: path.resolve(__dirname, '../../packages/ui/node_modules/@fontsource-variable/noto-sans-sc'),
    stylesheet: 'index.css',
  },
  {
    name: 'jetbrains-mono',
    packagePath: path.resolve(__dirname, '../../packages/ui/node_modules/@fontsource-variable/jetbrains-mono'),
    stylesheet: 'index.css',
  },
  {
    name: 'zcool-qingke-huangyou',
    packagePath: path.resolve(__dirname, '../../packages/ui/node_modules/@fontsource/zcool-qingke-huangyou'),
    stylesheet: '400.css',
  },
]

/** resolveFontAsset 将固定字体包路径与请求相对路径安全拼接,拒绝越过包根目录。 */
function resolveFontAsset(definition: FontAssetDefinition, relativePath: string): string | undefined {
  const packageRoot = path.resolve(definition.packagePath)
  const assetPath = path.resolve(packageRoot, relativePath)
  const relative = path.relative(packageRoot, assetPath)
  if (relative.startsWith('..') || path.isAbsolute(relative)) return undefined
  return assetPath
}

/** readFontAsset 读取已通过路径校验的字体资源,缺失资源交由调用方生成明确构建错误。 */
function readFontAsset(definition: FontAssetDefinition, relativePath: string): Buffer | undefined {
  const assetPath = resolveFontAsset(definition, relativePath)
  if (!assetPath) return undefined
  if (!fs.existsSync(assetPath)) return undefined
  return fs.readFileSync(assetPath)
}

/** fontAssetsPlugin 将字体包作为独立同源静态资源服务和输出,避免 Rollup 反复解析全部分片。 */
function fontAssetsPlugin(): Plugin {
  return {
    name: 'chaimir-font-assets',
    configureServer(server) {
      server.middlewares.use('/fonts', (request, response, next) => {
        let requestPath: string
        try {
          requestPath = decodeURIComponent((request.url ?? '').split('?')[0]).replace(/^\/+/, '')
        } catch {
          response.statusCode = 400
          response.end('字体资源路径无效')
          return
        }
        const separator = requestPath.indexOf('/')
        if (separator <= 0) {
          next()
          return
        }
        const packageName = requestPath.slice(0, separator)
        const relativePath = requestPath.slice(separator + 1)
        const definition = fontAssetDefinitions.find((item) => item.name === packageName)
        if (!definition) {
          next()
          return
        }
        const assetPath = resolveFontAsset(definition, relativePath)
        if (!assetPath || !fs.existsSync(assetPath)) {
          next()
          return
        }
        const extension = path.extname(assetPath)
        const contentType = extension === '.css' ? 'text/css; charset=utf-8' : extension === '.woff' ? 'font/woff' : 'font/woff2'
        response.setHeader('Content-Type', contentType)
        fs.createReadStream(assetPath).on('error', next).pipe(response)
      })
    },
    transformIndexHtml() {
      return fontAssetDefinitions.map((definition) => ({
        tag: 'link',
        attrs: { rel: 'stylesheet', href: `/fonts/${definition.name}/${definition.stylesheet}` },
        injectTo: 'head-prepend',
      }))
    },
    generateBundle() {
      for (const definition of fontAssetDefinitions) {
        const stylesheet = readFontAsset(definition, definition.stylesheet)
        if (!stylesheet) throw new Error(`字体资源缺失: ${definition.name}/${definition.stylesheet}`)
        this.emitFile({
          type: 'asset',
          fileName: `fonts/${definition.name}/${definition.stylesheet}`,
          source: stylesheet,
        })
        const filesDirectory = resolveFontAsset(definition, 'files')
        if (!filesDirectory || !fs.existsSync(filesDirectory)) throw new Error(`字体分片目录缺失: ${definition.name}`)
        for (const fileName of fs.readdirSync(filesDirectory)) {
          if (!/\.woff2?$/.test(fileName)) continue
          const source = readFontAsset(definition, `files/${fileName}`)
          if (!source) throw new Error(`字体分片读取失败: ${definition.name}/files/${fileName}`)
          this.emitFile({
            type: 'asset',
            fileName: `fonts/${definition.name}/files/${fileName}`,
            source,
          })
        }
      }
    },
  }
}

/**
 * readFrontendSecurityHeaders 从生产前端 Nginx 的唯一安全头配置读取本地入口策略。
 * Vite preview 入口承载构建产物，复用生产 Nginx 的 CSP、Referrer-Policy 与防嗅探头；
 * dev 入口保留 Vite React 预注入与 HMR 所需的开发协议，正式验收使用 HTTPS Ingress。
 */
function readFrontendSecurityHeaders(): Record<string, string> {
  const configPath = path.resolve(__dirname, '../../../images/service/frontend/security-headers.conf')
  const config = fs.readFileSync(configPath, 'utf8')
  const headers: Record<string, string> = {}
  const declaration = /^\s*add_header\s+([A-Za-z0-9-]+)\s+"([^"]*)"\s+always;\s*$/gm
  for (const match of config.matchAll(declaration)) {
    headers[match[1]] = match[2]
  }
  delete headers['Strict-Transport-Security']
  return headers
}

const monacoAmdAssets = [
  'loader.js',
  'editor/editor.main.js',
  'editor/editor.main.css',
  'base/worker/workerMain.js',
  'base/browser/ui/codicons/codicon/codicon.ttf',
  'language/css/cssMode.js',
  'language/css/cssWorker.js',
  'language/html/htmlMode.js',
  'language/html/htmlWorker.js',
  'language/json/jsonMode.js',
  'language/json/jsonWorker.js',
  'language/typescript/tsMode.js',
  'language/typescript/tsWorker.js',
  'basic-languages/go/go.js',
  'basic-languages/css/css.js',
  'basic-languages/html/html.js',
  'basic-languages/markdown/markdown.js',
  'basic-languages/python/python.js',
  'basic-languages/shell/shell.js',
  'basic-languages/solidity/solidity.js',
  'basic-languages/typescript/typescript.js',
  'basic-languages/yaml/yaml.js',
] as const

/** monacoAmdAssetsPlugin 输出 Monaco 官方预构建 AMD 文件,避免 Rollup 解析近千个 ESM 模块。 */
function monacoAmdAssetsPlugin(): Plugin {
  const packageRoot = path.resolve(__dirname, '../../packages/ide/node_modules/monaco-editor/min/vs')
  return {
    name: 'chaimir-monaco-amd-assets',
    generateBundle() {
      for (const relativePath of monacoAmdAssets) {
        const assetPath = path.resolve(packageRoot, relativePath)
        const relative = path.relative(packageRoot, assetPath)
        if (relative.startsWith('..') || path.isAbsolute(relative) || !fs.existsSync(assetPath)) {
          throw new Error(`Monaco 资源缺失: ${relativePath}`)
        }
        this.emitFile({
          type: 'asset',
          fileName: `assets/monaco/vs/${relativePath}`,
          source: fs.readFileSync(assetPath),
        })
      }
    },
  }
}

/**
 * tailwindAppStylesPlugins 只让 Tailwind 处理应用的唯一 CSS 入口。
 * Tailwind v4 会为每个 CSS 模块建立扫描器;Monaco 与终端带入的大量依赖 CSS
 * 没有 Tailwind 指令,在 Alpine 构建基座中反复扫描源码会把冷构建放大到不可接受。
 * 应用入口已经通过 @import 引入设计系统,因此收窄 transform 不会改变最终 CSS。
 */
function tailwindAppStylesPlugins(): Plugin[] {
  const appStylesPattern = /[\\/]apps[\\/]web[\\/]src[\\/]styles[\\/]app\.css(?:\?.*)?$/
  return tailwindcss({ optimize: false }).map((plugin) => {
    const transform = plugin.transform
    if (!transform || typeof transform !== 'object' || typeof transform.handler !== 'function') return plugin
    return {
      ...plugin,
      transform: {
        ...transform,
        filter: { id: { include: [appStylesPattern] } },
      },
    }
  })
}

export default defineConfig({
  envDir: path.resolve(__dirname, '../..'),
  // Tailwind 插件的 Lightning CSS 优化与 Vite 生产 CSS 处理重复;
  // 关闭前者,保留 Vite 单一 CSS 产物链,避免 Alpine 构建基座重复执行原生优化。
  plugins: [fontAssetsPlugin(), monacoAmdAssetsPlugin(), react(), ...tailwindAppStylesPlugins()],
  preview: {
    headers: readFrontendSecurityHeaders(),
  },
  // 路径别名不设:tsconfig 未声明对应 paths,只在 Vite 侧配别名会让 tsc 与打包解析结果不一致;
  // 全站统一用相对路径导入,单一解析规则。
  build: {
    // Monaco 由同源 AMD 静态资源按需加载,主应用不再把编辑器模块图纳入 Rollup。
    chunkSizeWarningLimit: 550,
    // Docker 构建阶段会统一生成 gzip 静态资源;Vite 再逐文件计算压缩体积只增加构建耗时,
    // 不改变产物内容或运行期压缩策略。
    reportCompressedSize: false,
  },
})
