// 一次性审查脚本:前端打包边界自证(工程目录设计 §4 前端铁律 2)。
// 取 dist/assets 里最大的 index-*.js 作为入口包,逐项 grep 四端路径前缀与中文栏目名,
// 并报告各 <Role>Section-*.js 是否独立成块、该区页面块是否随其加载。
// 用法:node scripts/audit/bundle-boundary.mjs
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

const DIST = join(process.cwd(), 'frontend/apps/web/dist/assets')
const files = readdirSync(DIST).filter((f) => f.endsWith('.js'))

// 入口包 = 最大的 index-*.js(Vite 把应用入口命名为 index-<hash>.js)
const entryName = files
  .filter((f) => /^index-[^.]+\.js$/.test(f))
  .map((f) => ({ f, size: statSync(join(DIST, f)).size }))
  .sort((a, b) => b.size - a.size)[0]
if (!entryName) {
  console.error('未找到入口包 index-*.js,请先执行 pnpm build')
  process.exit(1)
}
const entry = readFileSync(join(DIST, entryName.f), 'utf8')
console.log(`入口包: ${entryName.f}  (${(entryName.size / 1024).toFixed(1)} kB)`)

// 只匹配「作为前端路由字面量出现」的角色前缀:必须在引号/反引号后紧跟,
// 否则会把 SDK 的后端 API 路径(如 /contest/student/contests)误判为路由泄漏。
const ROLE_PREFIXES = ['/student/', '/teacher/', '/school-admin/', '/platform-admin/']
const routeLiteralRe = (prefix) =>
  new RegExp(`["'\`]${prefix.replace(/\//g, '\\/')}`, 'g')
const NAV_LABELS = [
  // 学生
  '我的课程', '实验实训', '仿真实验室', '竞赛参赛', '竞赛战绩', '成绩中心', '学业预警',
  // 教师
  '课程管理', '批改中心', '实验编排', '赛事组织', '实时监控', '题库内容', '试卷组卷', '仿真场景', '共享资源库', '成绩报送', '组织查看',
  // 校管
  '账号管理', '组织架构', '学校看板', '成绩审核', '申诉处理', '成绩配置', '租户配置', '认证配置', '系统公告', '审计日志', '学校告警',
  // 平台
  '学校管理', '入驻申请', '平台看板', '链运行时', '沙箱工具', '判题器', '仿真治理', '漏洞题源', '告警中心', '系统配置', '监控面板', '备份记录', '平台审计',
  // 分组标题
  '学习区', '学业区', '教学', '实践', '资源', '组织与成绩', '用户与组织', '概览', '教务与成绩', '系统配置', '租户', '运营', '底层资源',
]

function countOccurrences(haystack, needle) {
  let n = 0, i = 0
  for (;;) {
    const at = haystack.indexOf(needle, i)
    if (at === -1) return n
    n++
    i = at + needle.length
  }
}

function contextsOf(haystack, needle, limit = 3) {
  const out = []
  let i = 0
  for (;;) {
    const at = haystack.indexOf(needle, i)
    if (at === -1 || out.length >= limit) return out
    out.push(haystack.slice(Math.max(0, at - 90), at + needle.length + 60).replace(/\n/g, ' '))
    i = at + needle.length
  }
}

console.log('\n=== 入口包中的四端路由字面量 ===')
let leaked = 0
for (const p of ROLE_PREFIXES) {
  const all = countOccurrences(entry, p)
  const asRoute = (entry.match(routeLiteralRe(p)) || []).length
  console.log(`  ${p.padEnd(18)} 路由字面量 ${asRoute} 次(该子串总出现 ${all} 次,含后端 API 路径)`)
  if (asRoute > 0) {
    leaked += asRoute
    for (const ctx of contextsOf(entry, p)) console.log(`      … ${ctx}`)
  }
}

console.log('\n=== 入口包中的中文栏目名 ===')
let labelHits = 0
for (const label of NAV_LABELS) {
  const n = countOccurrences(entry, label)
  if (n > 0) {
    labelHits += n
    console.log(`  「${label}」 ${n} 次`)
    for (const ctx of contextsOf(entry, label, 2)) console.log(`      … ${ctx}`)
  }
}
if (!labelHits) console.log('  (0 命中)')

console.log('\n=== 各角色区块与该区页面块 ===')
for (const f of files.filter((x) => /Section-[^.]+\.js$/.test(x)).sort()) {
  const size = statSync(join(DIST, f)).size
  const src = readFileSync(join(DIST, f), 'utf8')
  const prefixHits = ROLE_PREFIXES.map((p) => `${p}=${countOccurrences(src, p)}`).join(' ')
  const labelCount = NAV_LABELS.filter((l) => src.includes(l)).length
  // 区块内静态 import 的兄弟块数量(懒加载页面块由动态 import 引入)
  const dynImports = [...src.matchAll(/import\(\s*["']\.\/([^"']+\.js)["']\s*\)/g)].map((m) => m[1])
  const staticImports = [...src.matchAll(/from\s*["']\.\/([^"']+\.js)["']/g)].map((m) => m[1])
  console.log(`  ${f}  (${(size / 1024).toFixed(1)} kB)`)
  console.log(`      路径前缀: ${prefixHits}`)
  console.log(`      本区栏目名命中: ${labelCount} 个`)
  console.log(`      动态引入页面块: ${dynImports.length} 个 | 静态引入: ${staticImports.length} 个`)
}

console.log('\n=== 判定 ===')
console.log(`入口包路由字面量命中合计 ${leaked} 次;中文栏目名命中合计 ${labelHits} 次。`)
console.log('唯一允许的例外是 utils/roleRouting.ts 的 homePath(见工程目录设计 §4 铁律 2)。')
