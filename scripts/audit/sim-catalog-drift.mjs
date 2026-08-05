// 一次性审查脚本:校验后端内置仿真包清单产物与前端 sim-sdk 源码一致。
//
// 为什么需要它:内置包的权威声明是前端 TS 源码,后端靠 go:embed 的 JSON 产物入库。
// 两者之间只有一次手动导出步骤(scripts/codegen/export-sim-builtin-catalog.mjs),漏跑的后果分两种:
//   新增包漏跑 → 生产库里没有这个包,学生的仿真实验室与实验编排都取不到它;
//   改交互漏跑 → 后端的 interaction_schema 白名单与前端声明不一致,学生的合法操作被拒。
// 两种都不会被构建、lint、type-check 拦住,故用本脚本做静态一致性校验。
//
// 用法:node scripts/audit/sim-catalog-drift.mjs
import { execFileSync } from 'node:child_process'
import { readFileSync, writeFileSync, copyFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'

const ROOT = process.cwd()
const EXPORTER = join(ROOT, 'scripts/codegen/export-sim-builtin-catalog.mjs')
const CATALOG = join(ROOT, 'backend/internal/modules/sim/builtin_catalog.json')
const BACKUP = join(tmpdir(), 'chaimir-builtin-catalog-backup.json')

if (!existsSync(CATALOG)) {
  console.error(`缺少内置仿真包清单产物:${CATALOG}\n请执行 node scripts/codegen/export-sim-builtin-catalog.mjs`)
  process.exit(1)
}

// 就地重跑导出后比对内容,再恢复原文件:脚本只做校验,不改工作区。
const committed = readFileSync(CATALOG, 'utf8')
copyFileSync(CATALOG, BACKUP)
try {
  execFileSync(process.execPath, [EXPORTER], { cwd: ROOT, encoding: 'utf8' })
} catch (error) {
  writeFileSync(CATALOG, committed, 'utf8')
  console.error('内置仿真包清单导出失败:')
  console.error(error.stdout?.trimEnd() ?? '')
  console.error(error.stderr?.trimEnd() ?? error.message)
  process.exit(1)
}
const regenerated = readFileSync(CATALOG, 'utf8')
writeFileSync(CATALOG, committed, 'utf8')

if (regenerated === committed) {
  const { packages } = JSON.parse(committed)
  console.log(`=== 内置仿真包清单一致(${packages.length} 个包) ===`)
  process.exit(0)
}

// 不逐行 diff 整份 JSON(它有 20 万字符),只报出包级差异,足以定位漏跑范围。
const keyOf = (item) => `${item.meta.code}@${item.meta.version}`
const before = new Map(JSON.parse(committed).packages.map((item) => [keyOf(item), JSON.stringify(item)]))
const after = new Map(JSON.parse(regenerated).packages.map((item) => [keyOf(item), JSON.stringify(item)]))

const added = [...after.keys()].filter((key) => !before.has(key))
const removed = [...before.keys()].filter((key) => !after.has(key))
const changed = [...after.keys()].filter((key) => before.has(key) && before.get(key) !== after.get(key))

console.error('=== 内置仿真包清单与源码不一致 ===')
if (added.length) console.error(`  源码新增未导出 (${added.length}): ${added.join(', ')}`)
if (removed.length) console.error(`  产物残留已删包 (${removed.length}): ${removed.join(', ')}`)
if (changed.length) console.error(`  协议声明已变更 (${changed.length}): ${changed.join(', ')}`)
console.error('\n请执行 node scripts/codegen/export-sim-builtin-catalog.mjs 并提交产物。')
process.exit(1)
