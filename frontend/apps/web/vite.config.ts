// vite.config.ts 配置统一 Web 应用的构建环境、源码别名和本地开发服务。
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

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
  plugins: [react(), tailwindcss()],
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
