// 一次性审查脚本:按前端页面所属角色目录,核对该页调用的 api-client 方法命中的后端守卫组是否相容。
// 例如 features/*/pages/student/* 调用了只挂 [Teacher,SchoolAdmin] 的路由 => 运行期必被拒,静态检查发现不了。
// 用法:node scripts/audit/role-guard-crosscheck.mjs
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, basename } from 'node:path'
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

// 1) 后端路由 + 守卫
const matrix = JSON.parse(execFileSync(process.execPath, [join(ROOT, 'scripts/audit/route-matrix.mjs'), '--json'], { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 }))

// 2) SDK 方法 -> (verb, path) 对。verb 与 path 必须取自同一次调用,否则
// 同路径不同方法(GET /courses 与 POST /courses)会互相污染,产生假阳性。
const VERB = { get: 'GET', getPaged: 'GET', getAttachment: 'GET', post: 'POST', put: 'PUT', patch: 'PATCH', delete: 'DELETE', upload: 'POST', wsUrl: 'GET', webSocketUrl: 'GET' }
const sdkFiles = walk(join(ROOT, 'frontend/packages/api-client/src/modules'), (p) => p.endsWith('.ts'))
const sdkMethod = new Map() // "module.name" -> { module, name, file, line, calls:[{verb,path}] }
for (const f of sdkFiles) {
  const lines = readFileSync(f, 'utf8').split(/\r?\n/)
  let cur = null
  lines.forEach((line, i) => {
    const dm = /^\s{2}(?:async\s+)?([a-zA-Z_]\w*)\s*(?:<[^>]*>)?\(/.exec(line)
    if (dm && !['constructor', 'if', 'for', 'while', 'switch', 'return', 'catch'].includes(dm[1])) {
      cur = { name: dm[1], module: basename(f, '.ts'), file: relative(ROOT, f).replace(/\\/g, '/'), line: i + 1, calls: [] }
      sdkMethod.set(`${cur.module}.${cur.name}`, cur)
      return
    }
    if (!cur) return
    // this.client.<verb>(<path>, ...) —— verb 与 path 同一行成对取出
    for (const cm of line.matchAll(/\.(get|post|put|patch|delete|getPaged|getAttachment|upload|wsUrl|webSocketUrl)(?:<[^>]*>)?\(\s*[`'"](\/[^`'"]*)[`'"]/g)) {
      cur.calls.push({ verb: VERB[cm[1]], path: cm[2] })
    }
  })
}

const normSdk = (p) => p.replace(/\$\{[^}]*\}/g, '{}').replace(/\/+$/, '')
const normBe = (p) => p.replace(/:([A-Za-z_]\w*)/g, '{}').replace(/\*(\w+)/g, '{}')
const beByKey = new Map()
for (const r of matrix.routes) {
  const k = `${r.method} ${normBe(r.path)}`
  beByKey.set(k, [...(beByKey.get(k) || []), r])
}
// 命中同一 (verb, path) 的后端注册;SDK 路径不含 /api/v1 前缀(由 client 拼)
function routesFor(verb, p) {
  for (const c of [normSdk(p), '/api/v1' + normSdk(p)]) {
    const k = `${verb} ${c}`
    if (beByKey.has(k)) return beByKey.get(k)
  }
  return []
}

// 3) 前端页面 -> 使用的 SDK 方法
// 角色目录判定:.../pages/<role>/... 或路径含 /student/ /teacher/ /school-admin/ /platform/
const ROLE_DIRS = { student: 'Student', teacher: 'Teacher', 'school-admin': 'SchoolAdmin', schoolAdmin: 'SchoolAdmin', platform: 'Platform', 'platform-admin': 'Platform' }
const pageFiles = walk(join(ROOT, 'frontend/apps/web/src'), (p) => /\.(ts|tsx)$/.test(p))

const problems = []
const rows = []
for (const f of pageFiles) {
  const rel = relative(ROOT, f).replace(/\\/g, '/')
  const segs = rel.split('/')
  let role = null
  for (const s of segs) if (ROLE_DIRS[s]) { role = ROLE_DIRS[s]; break }
  const src = readFileSync(f, 'utf8')
  // api.<module>.<method>(  或  解构后的裸方法名调用
  const used = new Set()
  for (const m of src.matchAll(/\bapi\.(\w+)\.(\w+)\s*\(/g)) used.add(`${m[1]}.${m[2]}`)
  for (const m of src.matchAll(/\b(\w+)\.(\w+)\s*\(/g)) {
    const key = `${m[1]}.${m[2]}`
    if (sdkMethod.has(key)) used.add(key)
  }
  for (const key of used) {
    const sm = sdkMethod.get(key)
    if (!sm) continue
    for (const call of sm.calls) {
      const hits = routesFor(call.verb, call.path)
      for (const h of hits) {
        rows.push({ page: rel, role, key, method: h.method, path: h.path, guards: h.guards })
        if (!role) continue
        const rm = /\[([A-Za-z,]+)\]/.exec(h.guards)
        const isPlatformOnly = /RequirePlatformIdentity/.test(h.guards) && !/RequirePlatformOrAnyRole/.test(h.guards)
        if (role === 'Platform') {
          if (rm && !isPlatformOnly && !/RequirePlatformOrAnyRole/.test(h.guards)) {
            problems.push({ page: rel, role, key, route: `${h.method} ${h.path}`, guards: h.guards, why: '平台身份调用租户角色组接口' })
          }
        } else if (isPlatformOnly) {
          problems.push({ page: rel, role, key, route: `${h.method} ${h.path}`, guards: h.guards, why: '租户角色调用仅平台身份接口' })
        } else if (rm && !rm[1].split(',').includes(role)) {
          problems.push({ page: rel, role, key, route: `${h.method} ${h.path}`, guards: h.guards, why: `该角色不在守卫枚举 [${rm[1]}] 内` })
        }
      }
    }
  }
}

// 已人工确认的例外:共用实现由调用方显式声明能力(对齐清单 §6.11),
// 动作藏在 prop 门控分支后,静态扫描看不见门控,故在此登记原因而不是放宽判定。
const VERIFIED_EXCEPTIONS = new Map([
  [
    'frontend/apps/web/src/features/grade/pages/teacher/grade-appeals.tsx|grade.recomputeStudentGrade',
    '「重算绩点」由 canRecompute prop 门控,只有校管区壳传 true(§6.11);教师侧渲染不出该动作',
  ],
])

console.log(`=== 页面-守卫核对:共比对 ${rows.length} 条(页面,方法,路由)组合 ===`)
const unresolved = problems.filter((p) => !VERIFIED_EXCEPTIONS.has(`${p.page}|${p.key}`))
console.log(`\n=== 角色与守卫不相容 (${unresolved.length}) ===`)
const seen = new Set()
for (const p of unresolved) {
  const k = `${p.page}|${p.key}|${p.route}`
  if (seen.has(k)) continue
  seen.add(k)
  console.log(`  [${p.role}] ${p.page}\n      ${p.key} -> ${p.route}\n      guards=${p.guards}  (${p.why})`)
}
if (!unresolved.length) console.log('  (无)')

const excepted = problems.filter((p) => VERIFIED_EXCEPTIONS.has(`${p.page}|${p.key}`))
if (excepted.length) {
  console.log('\n=== 已登记的人工确认例外 ===')
  const shown = new Set()
  for (const p of excepted) {
    const key = `${p.page}|${p.key}`
    if (shown.has(key)) continue
    shown.add(key)
    console.log(`  [${p.role}] ${p.page}\n      ${p.key} -> ${p.route}\n      ${VERIFIED_EXCEPTIONS.get(key)}`)
  }
}
