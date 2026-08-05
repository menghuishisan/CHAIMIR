// 一次性审查脚本:提取 frontend/packages/api-client 每个公开方法的 HTTP 路径,
// 与 route-matrix 解析出的后端路由做双向核对(路径是否存在 + 守卫组是否与调用端角色相容),
// 并列出 SDK 公开方法在 apps/web 内是否有引用(可达性)。
// 用法:node scripts/audit/sdk-matrix.mjs
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, dirname, basename } from 'node:path'
import { execFileSync } from 'node:child_process'

const ROOT = process.cwd()

function walk(dir, filter, out = []) {
  let names
  try { names = readdirSync(dir) } catch { return out }
  for (const name of names) {
    if (name === 'node_modules' || name === '.git' || name === 'dist') continue
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) walk(p, filter, out)
    else if (filter(p)) out.push(p)
  }
  return out
}

// 后端路由(复用 route-matrix 的解析)
const matrix = JSON.parse(execFileSync(process.execPath, [join(ROOT, 'scripts/audit/route-matrix.mjs'), '--json'], { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 }))
const backend = matrix.routes.map((r) => ({ ...r, norm: r.path.replace(/:([A-Za-z_]\w*)/g, '{}').replace(/\*(\w+)/g, '{}') }))

// SDK:逐方法抓路径
const sdkFiles = walk(join(ROOT, 'frontend/packages/api-client/src/modules'), (p) => p.endsWith('.ts'))
const methods = []
for (const f of sdkFiles) {
  const lines = readFileSync(f, 'utf8').split(/\r?\n/)
  let cur = null
  lines.forEach((line, i) => {
    const dm = /^\s{2}(?:async\s+)?([a-zA-Z_]\w*)\s*(?:<[^>]*>)?\(/.exec(line)
    if (dm && !['constructor', 'if', 'for', 'while', 'switch', 'return', 'catch'].includes(dm[1])) {
      cur = { file: relative(ROOT, f).replace(/\\/g, '/'), line: i + 1, name: dm[1], module: basename(f, '.ts'), paths: [], verbs: [] }
      methods.push(cur)
      return
    }
    if (!cur) return
    // 路径字面量:模板串或普通串,以 / 开头
    for (const pm of line.matchAll(/[`'"](\/[^`'"$]*(?:\$\{[^}]*\}[^`'"]*)*)[`'"]/g)) {
      cur.paths.push(pm[1])
    }
    for (const vm of line.matchAll(/\.(get|post|put|patch|delete|getPaged|getAttachment|upload|wsUrl|webSocketUrl)\b/g)) {
      cur.verbs.push(vm[1])
    }
  })
}

// 归一化 SDK 路径:${x} -> {}
const normSdk = (p) => p.replace(/\$\{[^}]*\}/g, '{}').replace(/\/+$/, '')
const verbToMethod = { get: 'GET', getPaged: 'GET', getAttachment: 'GET', post: 'POST', put: 'PUT', patch: 'PATCH', delete: 'DELETE', upload: 'POST' }

const findings = []
for (const m of methods) {
  for (const raw of m.paths) {
    const p = normSdk(raw)
    if (!p.startsWith('/')) continue
    // SDK 路径不含 /api/v1 前缀(由 client 拼),两种都试
    const cands = [p, '/api/v1' + p]
    const hits = backend.filter((r) => cands.includes(r.norm))
    findings.push({ ...m, rawPath: raw, path: p, hits: hits.map((h) => ({ method: h.method, path: h.path, guards: h.guards, file: h.file, line: h.line })) })
  }
}

// 可达性:SDK 方法名在 apps/web、其他 packages 或 api-client 自身(含 modules/ 内的相互调用)中是否被调用。
// 注意 Windows 路径分隔符是 `\`,过滤目录必须归一化后再比较。
// modules/ 也要纳入搜索:同模块内一个方法包装另一个是真实调用(如 transfer.downloadArtifact 调
// downloadGrant),把 modules/ 整体排除会把这类方法误报成不可达。改为逐方法排除它自己的定义行。
const norm = (p) => p.replace(/\\/g, '/')
const appFiles = walk(join(ROOT, 'frontend/apps/web/src'), (p) => /\.(ts|tsx)$/.test(p))
const appSrc = appFiles.map((f) => ({ f: relative(ROOT, f).replace(/\\/g, '/'), src: readFileSync(f, 'utf8') }))
const pkgFiles = walk(join(ROOT, 'frontend/packages'), (p) => /\.(ts|tsx)$/.test(p) && !norm(p).includes('/api-client/'))
const pkgSrc = pkgFiles.map((f) => ({ f: relative(ROOT, f).replace(/\\/g, '/'), src: readFileSync(f, 'utf8') }))
const clientSrc = walk(join(ROOT, 'frontend/packages/api-client/src'), (p) => /\.ts$/.test(p))
  .map((f) => ({ f: relative(ROOT, f).replace(/\\/g, '/'), src: readFileSync(f, 'utf8').split(/\r?\n/) }))

const unreachable = []
for (const m of methods) {
  const re = new RegExp(`\\b${m.name}\\s*\\(`)
  const inApp = appSrc.filter((x) => re.test(x.src)).map((x) => x.f)
  const inPkg = pkgSrc.filter((x) => re.test(x.src)).map((x) => x.f)
  // api-client 内部:排除该方法自己的定义行(file+line 唯一确定),其余命中都是真实调用
  const inClient = clientSrc.filter((x) =>
    x.src.some((line, i) => re.test(line) && !(x.f === m.file && i + 1 === m.line)),
  ).map((x) => x.f)
  if (!inApp.length && !inPkg.length && !inClient.length) unreachable.push(m)
}

console.log('=== 统计 ===')
console.log(`SDK 方法 ${methods.length} 个 | 抓到路径 ${findings.length} 条`)

console.log('\n=== A. SDK 路径命中不到任何后端路由 ===')
let miss = 0
for (const f of findings) {
  if (f.hits.length) continue
  miss++
  console.log(`  ${f.module}.${f.name}  "${f.rawPath}"  [${f.file}:${f.line}]`)
}
if (!miss) console.log('  (无)')

console.log('\n=== B. SDK 命中 /internal 或 service 守卫的路由(浏览器不可调) ===')
let bad = 0
for (const f of findings) {
  for (const h of f.hits) {
    if (h.path.includes('/internal/') || /ServiceMiddleware/.test(h.guards)) {
      bad++
      console.log(`  ${f.module}.${f.name} -> ${h.method} ${h.path}  guards=${h.guards}  [${f.file}:${f.line}]`)
    }
  }
}
if (!bad) console.log('  (无)')

console.log(`\n=== C. SDK 公开方法在前端无任何引用(可达性缺口候选) ${unreachable.length} ===`)
for (const m of unreachable) console.log(`  ${m.module}.${m.name}  [${m.file}:${m.line}]  paths=${m.paths.join(' , ') || '(无字面路径)'}`)

console.log('\n=== D. 每条 SDK 路径的守卫组(供人工核对角色相容性) ===')
const byModule = new Map()
for (const f of findings) byModule.set(f.module, [...(byModule.get(f.module) || []), f])
for (const [mod, list] of [...byModule].sort()) {
  console.log(`\n-- ${mod} --`)
  for (const f of list) {
    for (const h of f.hits) console.log(`  ${f.name.padEnd(34)} ${h.method.padEnd(6)} ${h.path.padEnd(62)} ${h.guards}`)
  }
}
