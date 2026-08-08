// 一次性审查脚本:解析后端 gin 路由的绝对路径与守卫链,与 backend/api/paths/*.yaml 及
// openapi.yaml 做三方比对,并列出同 handler 多次注册的双轨嫌疑。
//
// 解析方式:按函数体作用域收集 Group 定义与路由注册,再按 register* 调用点把形参 gin.IRouter
// 绑定到实参(支持变量与内联 g.Group(...) 表达式);路由行上的内联中间件也计入守卫链。
// 用法:node scripts/audit/route-matrix.mjs [--json]
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, dirname } from 'node:path'

const ROOT = process.cwd()
const AS_JSON = process.argv.includes('--json')

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

// 按顶层逗号切分实参串(忽略括号、方括号与字符串内的逗号)
function splitArgs(s) {
  const out = []
  let depth = 0, cur = '', inStr = false
  for (let i = 0; i < s.length; i++) {
    const ch = s[i]
    if (inStr) { cur += ch; if (ch === '"' && s[i - 1] !== '\\') inStr = false; continue }
    if (ch === '"') { inStr = true; cur += ch; continue }
    if ('([{'.includes(ch)) depth++
    if (')]}'.includes(ch)) depth--
    if (ch === ',' && depth === 0) { out.push(cur.trim()); cur = ''; continue }
    cur += ch
  }
  if (cur.trim()) out.push(cur.trim())
  return out
}

// 取 name( ... ) 的实参串(要求括号在同一行闭合)
function callArgs(text, calleeRe) {
  const m = calleeRe.exec(text)
  if (!m) return null
  let i = m.index + m[0].length
  if (text[i - 1] !== '(') return null
  let depth = 1, start = i, inStr = false
  for (; i < text.length; i++) {
    const ch = text[i]
    if (inStr) { if (ch === '"' && text[i - 1] !== '\\') inStr = false; continue }
    if (ch === '"') { inStr = true; continue }
    if ('([{'.includes(ch)) depth++
    else if (')]}'.includes(ch)) { depth--; if (depth === 0) return { name: m[1], args: splitArgs(text.slice(start, i)) } }
  }
  return null
}

function guardsOf(text) {
  const g = []
  for (const m of text.matchAll(/(?:auth|authn)\.(\w+)/g)) g.push(m[1])
  const roles = [...text.matchAll(/contracts\.(Role\w+)/g)].map((x) => x[1].replace('Role', ''))
  if (roles.length) g.push(`[${roles.join(',')}]`)
  return g
}

function splitFuncs(lines) {
  const funcs = []
  let cur = null
  lines.forEach((line, i) => {
    const m = /^func(?:\s+\(([^)]*)\))?\s+(\w+)\s*\(([^)]*)\)/.exec(line)
    if (m) { cur = { name: m[2], params: m[3], body: [] }; funcs.push(cur); return }
    if (/^\}/.test(line)) { cur = null; return }
    if (cur) cur.body.push({ line: i + 1, text: line })
  })
  return funcs
}

function parsePackage(dir) {
  const isServerAssembly = relative(ROOT, dir).replace(/\\/g, '/') === 'backend/cmd/server'
  const files = walk(dir, (p) =>
    !p.endsWith('_test.go') && dirname(p) === dir && (/api[^/\\]*\.go$/.test(p) || (isServerAssembly && p.endsWith('main.go'))),
  )
  if (!files.length) return []

  const blocks = []
  for (const f of files) {
    const lines = readFileSync(f, 'utf8').split(/\r?\n/)
    for (const fn of splitFuncs(lines)) {
      const routerParam = (/(\w+)\s+gin\.IRouter/.exec(fn.params) || [])[1] || null
      const groupDefs = new Map()
      const registrations = []
      const calls = []
      for (const { line, text } of fn.body) {
        const t = text.trim()
        const gm = /^(\w+)\s*(?::=|=)\s*(\w+)\.Group\(/.exec(t)
        if (gm) {
          const ca = callArgs(t, new RegExp(`${gm[2]}\\.Group\\(`))
          if (ca) {
            const suffix = /^"([^"]*)"$/.exec(ca.args[0] || '""')
            groupDefs.set(gm[1], { base: gm[2], suffix: suffix ? suffix[1] : '', mw: ca.args.slice(1).join(', ') })
            continue
          }
        }
        // 路由注册。groupVar 可能是变量,也可能是链式内联 `x.Group("p", mw).GET(...)`,
        // 故先按最后一个 `.METHOD(` 切开,左侧整体当作组表达式交给 resolveExpr。
        const rm = /^(.*?)\.(GET|POST|PUT|PATCH|DELETE)\(/.exec(t)
        if (rm) {
          const ca = callArgs(t.slice(rm[1].length), new RegExp(`^\\.${rm[2]}\\(`))
          if (ca && ca.args.length >= 2) {
            const sub = /^"([^"]*)"$/.exec(ca.args[0])
            registrations.push({
              line, groupVar: rm[1], method: rm[2],
              sub: sub ? sub[1] : ca.args[0],
              inlineMw: ca.args.slice(1, -1).join(', '),
              handler: ca.args[ca.args.length - 1],
            })
            continue
          }
        }
        // g.Match([]string{http.MethodGet, ...}, "/path", handler) —— 一次注册多方法
        const mm2 = /\b(\w+)\.Match\(/.exec(t)
        if (mm2) {
          const ca = callArgs(t, new RegExp(`${mm2[1]}\\.Match\\(`))
          if (ca && ca.args.length >= 3) {
            const sub = /^"([^"]*)"$/.exec(ca.args[1])
            for (const meth of ca.args[0].matchAll(/http\.Method(\w+)/g)) {
              const m = meth[1].toUpperCase()
              if (!['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].includes(m)) continue
              registrations.push({
                line, groupVar: mm2[1], method: m,
                sub: sub ? sub[1] : ca.args[1],
                inlineMw: ca.args.slice(2, -1).join(', '),
                handler: ca.args[ca.args.length - 1],
              })
            }
            continue
          }
        }
        // register* / Register* 调用点:记录第一个实参
        const cm = callArgs(t, /(?:\w+\.)?(\w*[Rr]egister\w*)\(/)
        if (cm && cm.args.length) calls.push({ callee: cm.name, arg: cm.args[0] })
      }
      blocks.push({ fnName: fn.name, file: relative(ROOT, f).replace(/\\/g, '/'), routerParam, groupDefs, registrations, calls })
    }
  }

  // 在给定块内解析「组表达式」(变量名或内联 X.Group(...)) 为 { prefix, guards } 列表
  function resolveExpr(block, expr, depth = 0) {
    if (depth > 12) return [{ prefix: '?', guards: ['DEPTH'] }]
    const inline = /^(\w+)\.Group\(/.exec(expr)
    if (inline) {
      const ca = callArgs(expr, new RegExp(`^${inline[1]}\\.Group\\(`))
      const suffix = ca ? (/^"([^"]*)"$/.exec(ca.args[0] || '""') || ['', ''])[1] : ''
      const mw = ca ? ca.args.slice(1).join(', ') : ''
      return resolveExpr(block, inline[1], depth + 1).map((p) => ({ prefix: p.prefix + suffix, guards: [...p.guards, ...guardsOf(mw)] }))
    }
    if (!/^\w+$/.test(expr)) return [{ prefix: '', guards: [`EXPR(${expr})`] }]
    const def = block.groupDefs.get(expr)
    if (def) {
      return resolveExpr(block, def.base, depth + 1).map((p) => ({ prefix: p.prefix + def.suffix, guards: [...p.guards, ...guardsOf(def.mw)] }))
    }
    if (block.routerParam === expr) {
      // 同名方法(如 accountAPI.register / meAPI.register)只能靠同文件消歧,
      // 否则一个 register 的组前缀会被另一个的调用点污染。
      const collect = (pool) => {
        const results = []
        for (const caller of pool) {
          for (const c of caller.calls) {
            if (c.callee !== block.fnName) continue
            for (const r of resolveExpr(caller, c.arg, depth + 1)) results.push(r)
          }
        }
        return results
      }
      const sameFile = collect(blocks.filter((b) => b.file === block.file))
      const results = sameFile.length ? sameFile : collect(blocks)
      if (results.length) return results
      return [{ prefix: '', guards: [] }]
    }
    return [{ prefix: '', guards: [] }]
  }

  const out = []
  const seen = new Set()
  for (const b of blocks) {
    for (const reg of b.registrations) {
      for (const { prefix, guards } of resolveExpr(b, reg.groupVar)) {
        const allGuards = [...guards, ...guardsOf(reg.inlineMw)]
        const rec = { file: b.file, line: reg.line, method: reg.method, path: prefix + reg.sub, guards: allGuards.join('+') || '(none)', handler: reg.handler, fn: b.fnName }
        const k = `${rec.method} ${rec.path} ${rec.file}:${rec.line} ${rec.guards}`
        if (seen.has(k)) continue
        seen.add(k)
        out.push(rec)
      }
    }
  }
  return out
}

const pkgDirs = [
  ...readdirSync(join(ROOT, 'backend/internal/modules')).map((n) => join(ROOT, 'backend/internal/modules', n)),
  join(ROOT, 'backend/internal/platform/transfer'),
  join(ROOT, 'backend/internal/platform/storage'),
  // server 装配层注册健康探针;纳入 /api/healthz,但下方过滤仅供 K8s 的 /-/healthz 和 /-/readyz。
  join(ROOT, 'backend/cmd/server'),
]
const routes = pkgDirs.flatMap(parsePackage).filter((route) => !['/-/healthz', '/-/readyz'].includes(route.path))

// ── openapi ──────────────────────────────────────────────────────────────────
const yamlOps = []
for (const f of walk(join(ROOT, 'backend/api/paths'), (p) => p.endsWith('.yaml'))) {
  const lines = readFileSync(f, 'utf8').split(/\r?\n/)
  let cur = null
  lines.forEach((line, i) => {
    const pm = /^(\/\S+):\s*$/.exec(line)
    if (pm) { cur = pm[1]; return }
    const mm = /^\s{2}(get|post|put|patch|delete):\s*$/.exec(line)
    if (mm && cur) {
      let opId = '', aud = ''
      for (let j = i + 1; j < Math.min(i + 16, lines.length); j++) {
        if (/^\s{2}\w+:\s*$/.test(lines[j]) || /^\/\S+:\s*$/.test(lines[j])) break
        const o = /operationId:\s*(\S+)/.exec(lines[j]); if (o) opId = o[1]
        const a = /x-chaimir-audience:\s*(\S+)/.exec(lines[j]); if (a) aud = a[1]
      }
      yamlOps.push({ file: relative(ROOT, f).replace(/\\/g, '/'), line: i + 1, path: cur, method: mm[1].toUpperCase(), operationId: opId, audience: aud })
    }
  })
}
const openapiSrc = readFileSync(join(ROOT, 'backend/api/openapi.yaml'), 'utf8')
const indexedPaths = new Set([...openapiSrc.matchAll(/^\s{2}(\/[^\s:]+):/gm)].map((m) => m[1]))

// ── 比对 ─────────────────────────────────────────────────────────────────────
const norm = (p) => p.replace(/:([A-Za-z_]\w*)/g, '{$1}').replace(/\*(\w+)/g, '{$1}')
const backendSet = new Map()
for (const r of routes) if (!backendSet.has(`${r.method} ${norm(r.path)}`)) backendSet.set(`${r.method} ${norm(r.path)}`, r)
const yamlSet = new Map()
for (const o of yamlOps) yamlSet.set(`${o.method} ${o.path}`, o)

const onlyBackend = [...backendSet.keys()].filter((k) => !yamlSet.has(k)).sort()
const onlyYaml = [...yamlSet.keys()].filter((k) => !backendSet.has(k)).sort()
const yamlNotIndexed = [...new Set(yamlOps.map((o) => o.path))].filter((p) => !indexedPaths.has(p)).sort()
const indexedNotYaml = [...indexedPaths].filter((p) => !yamlOps.some((o) => o.path === p)).sort()

if (AS_JSON) {
  console.log(JSON.stringify({ routes, yamlOps, onlyBackend, onlyYaml, yamlNotIndexed, indexedNotYaml }, null, 0))
} else {
  console.log('=== 统计 ===')
  console.log(`后端注册 ${backendSet.size} 条 | paths/*.yaml ${yamlSet.size} 条 | openapi.yaml 索引 ${indexedPaths.size} 路径`)
  const unresolved = routes.filter((r) => !r.path.startsWith('/api/'))
  console.log(`\n=== 未解析出绝对路径(脚本盲区) ${unresolved.length} ===`)
  for (const r of unresolved) console.log(`  ${r.method} ${r.path} [${r.file}:${r.line}] fn=${r.fn} guards=${r.guards}`)
  console.log(`\n=== A. 后端有、openapi 无 (${onlyBackend.length}) ===`)
  for (const k of onlyBackend) { const r = backendSet.get(k); console.log(`  ${k}  |  ${r.file}:${r.line} guards=${r.guards}`) }
  console.log(`\n=== B. openapi 有、后端无 (${onlyYaml.length}) ===`)
  for (const k of onlyYaml) { const o = yamlSet.get(k); console.log(`  ${k}  op=${o.operationId} aud=${o.audience}  ${o.file}:${o.line}`) }
  console.log(`\n=== C. paths/*.yaml 有但 openapi.yaml 未索引 (${yamlNotIndexed.length}) ===`)
  for (const p of yamlNotIndexed) console.log('  ' + p)
  console.log(`\n=== D. openapi.yaml 索引了但 paths/*.yaml 无 (${indexedNotYaml.length}) ===`)
  for (const p of indexedNotYaml) console.log('  ' + p)
  const byHandler = new Map()
  for (const r of routes) {
    const key = `${dirname(r.file)}::${r.handler}`
    byHandler.set(key, [...(byHandler.get(key) || []), r])
  }
  console.log('\n=== E. 同一 handler 注册多次(双轨嫌疑) ===')
  let dup = 0
  for (const [k, v] of byHandler) {
    const uniq = [...new Set(v.map((r) => `${r.method} ${norm(r.path)}`))]
    if (uniq.length > 1) { dup++; console.log(`  ${k}\n      ${v.map((r) => `${r.method} ${norm(r.path)} @${r.line} guards=${r.guards}`).join('\n      ')}`) }
  }
  if (!dup) console.log('  (无)')
}
