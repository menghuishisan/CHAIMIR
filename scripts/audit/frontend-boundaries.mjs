/**
 * 前端部署边界静态门禁:校验公共 origin、工具 iframe origin、CAS 回调白名单、
 * Ingress overlay 和前端安全响应头是否仍保持单一契约。
 * 真实 TLS 响应头和登录态浏览器验收仍需在运行中的部署环境执行。
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const PUBLIC_HOST = 'www.chaimir.io'
const TOOL_ORIGIN = 'https://tools.chaimir.io'
const failures = []

function read(relativePath) {
  const absolutePath = path.join(ROOT, relativePath)
  if (!fs.existsSync(absolutePath)) {
    failures.push(`${relativePath}: 文件不存在`)
    return ''
  }
  return fs.readFileSync(absolutePath, 'utf8')
}

function requireMatch(relativePath, source, pattern, message) {
  if (!pattern.test(source)) failures.push(`${relativePath}: ${message}`)
}

for (const relativePath of ['frontend/.env', 'frontend/.env.example']) {
  const source = read(relativePath)
  requireMatch(relativePath, source, /^VITE_API_BASE_URL=\/api\/v1\s*$/m, 'API 必须使用同源 /api/v1')
  requireMatch(relativePath, source, /^VITE_WS_BASE_URL=\s*$/m, 'WebSocket 默认必须使用同源票据流')
  requireMatch(relativePath, source, new RegExp(`^VITE_SANDBOX_TOOL_ORIGIN=${TOOL_ORIGIN.replaceAll('.', '\\.') }\\s*$`, 'm'), `工具 origin 必须为 ${TOOL_ORIGIN}`)
}

for (const relativePath of ['backend/.env.example', 'deploy/config/chaimir.env']) {
  const source = read(relativePath)
  requireMatch(relativePath, source, new RegExp(`^IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS=https://${PUBLIC_HOST.replaceAll('.', '\\.') }\\s*$`, 'm'), `CAS service origin 默认必须为 https://${PUBLIC_HOST}`)
}

const headersPath = 'images/service/frontend/security-headers.conf'
const headers = read(headersPath)
for (const [pattern, message] of [
  [/add_header Content-Security-Policy[\s\S]*connect-src 'self'/, '必须限制 API/WS 为同源 connect-src'],
  [new RegExp(`frame-src 'self' ${TOOL_ORIGIN.replaceAll('.', '\\.')}`), `必须只允许同源和 ${TOOL_ORIGIN} 的 iframe`],
  [/add_header X-Content-Type-Options "nosniff"/, '必须启用 nosniff'],
  [/add_header Referrer-Policy "no-referrer"/, '必须启用 no-referrer'],
  [/add_header Strict-Transport-Security "max-age=31536000; includeSubDomains"/, '必须启用 HSTS'],
  [/add_header Permissions-Policy/, '必须显式限制浏览器高风险权限'],
]) requireMatch(headersPath, headers, pattern, message)

for (const name of ['local-dev', 'staging', 'prod-saas', 'prod-school']) {
  const relativePath = `deploy/overlays/${name}/kustomization.yaml`
  const source = read(relativePath)
  const publicHostCount = (source.match(new RegExp(`value:\\s*${PUBLIC_HOST.replaceAll('.', '\\.')}`, 'g')) ?? []).length
  if (publicHostCount !== 4) failures.push(`${relativePath}: 必须为主 Ingress 和拒绝 Ingress 的 rules/TLS 各设置一次 ${PUBLIC_HOST}(当前 ${publicHostCount} 次)`)
  if (/value:\s*chaimir\s*$/m.test(source)) failures.push(`${relativePath}: 不能保留旧主域名 overlay 值 chaimir`)
}

const stalePatterns = [
  /https?:\/\/chaimir(?:[/:]|$)/i,
  /chaimir\.local/i,
  /harbor\.chaimir\.local/i,
  /chaimir\.example\.edu/i,
  /registry\.chaimir\.local/i,
  /127\.0\.0\.1:5000/i,
]
const scanRoots = ['frontend/apps', 'frontend/packages', 'backend/internal', 'backend/cmd', 'deploy/base', 'deploy/overlays', 'images/service/frontend', 'scripts']
const ignoredNames = new Set(['node_modules', 'dist', '.tmp'])
function scan(relativeDirectory) {
  const directory = path.join(ROOT, relativeDirectory)
  if (!fs.existsSync(directory)) return
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (ignoredNames.has(entry.name)) continue
    const relativePath = path.join(relativeDirectory, entry.name)
    if (entry.isDirectory()) scan(relativePath)
    else if (/\.(?:ts|tsx|js|jsx|go|yaml|yml|env|conf|ps1|sh|md)$/i.test(entry.name)) {
      const source = fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
      for (const pattern of stalePatterns) {
        if (pattern.test(source)) {
          failures.push(`${relativePath}: 检出旧公共域名/registry 占位值 ${pattern}`)
          break
        }
      }
    }
  }
}
for (const root of scanRoots) scan(root)

if (failures.length > 0) {
  console.error('前端部署边界检查失败:')
  for (const failure of failures) console.error(`- ${failure}`)
  process.exitCode = 1
} else {
  console.log(`前端部署边界检查通过:公共入口 ${PUBLIC_HOST},工具 origin ${TOOL_ORIGIN},四个 overlay 与安全响应头一致。`)
}
