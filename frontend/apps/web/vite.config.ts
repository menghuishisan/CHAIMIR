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

/** monacoManualChunk 按 Monaco 稳定源码边界拆分仅在 IDE 打开时加载的编辑器核心。 */
function monacoManualChunk(moduleId: string): string | undefined {
  const normalized = moduleId.replace(/\\/g, '/')
  const marker = '/monaco-editor/esm/vs/'
  const markerIndex = normalized.indexOf(marker)
  if (markerIndex < 0) return undefined
  const relative = normalized.slice(markerIndex + marker.length)
  const segments = relative.split('/')

  if (segments[0] === 'base' && segments[1]) return `monaco-base-${segments[1]}`
  if (segments[0] === 'platform') return 'monaco-platform'
  if (segments[0] === 'editor' && segments[1] === 'common' && segments[2]) {
    if (segments[2] === 'model' || segments[2] === 'services') return 'monaco-editor-common-model-services'
    return `monaco-editor-common-${segments[2]}`
  }
  if (segments[0] === 'editor' && segments[1] === 'browser' && segments[2]) {
    const browserCore = ['controller', 'coreCommands.js', 'editorExtensions.js', 'services', 'view', 'view.js', 'viewParts', 'widget']
    if (browserCore.includes(segments[2])) return 'monaco-editor-browser-core'
    return `monaco-editor-browser-${segments[2]}`
  }
  if (segments[0] === 'editor' && segments[1] === 'contrib' && segments[2]) {
    const hoverFeatures = ['colorPicker', 'hover', 'inlayHints']
    const suggestFeatures = ['inlineCompletions', 'snippet', 'suggest']
    if (hoverFeatures.includes(segments[2])) return 'monaco-editor-contrib-hover'
    if (suggestFeatures.includes(segments[2])) return 'monaco-editor-contrib-suggest'
    return `monaco-editor-contrib-${segments[2]}`
  }
  if (segments[0] === 'editor' && segments[1] === 'standalone') return 'monaco-editor-standalone'
  if (segments[0] === 'editor') return 'monaco-editor-api'
  return undefined
}

export default defineConfig({
  envDir: path.resolve(__dirname, '../..'),
  plugins: [fontAssetsPlugin(), react(), tailwindcss()],
  preview: {
    headers: readFrontendSecurityHeaders(),
  },
  // 路径别名不设:tsconfig 未声明对应 paths,只在 Vite 侧配别名会让 tsc 与打包解析结果不一致;
  // 全站统一用相对路径导入,单一解析规则。
  build: {
    // Monaco 仅在 IDE 打开后加载并已拆到约 500 KiB;保留少量版本波动空间。
    chunkSizeWarningLimit: 550,
    rollupOptions: {
      output: {
        manualChunks: monacoManualChunk,
      },
    },
  },
})
