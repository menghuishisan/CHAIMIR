// 一次性审查脚本:合规审查的统一入口,依次跑五项静态一致性检查并汇总判定。
// 用法:node scripts/audit/index.mjs
//
// 五项分别回答一个问题:
//   route-matrix          后端注册路由 ↔ openapi 契约是否四方一致、有无同 handler 双轨
//   sdk-matrix            api-client 每条路径是否命中真实后端路由、有无不可达方法
//   role-guard-crosscheck 各角色页面调用的接口守卫是否与该角色相容(静态检查发现不了的越权)
//   sim-catalog-drift     后端内置仿真包清单产物 ↔ 前端 sim-sdk 源码是否一致
//   bundle-boundary       入口包是否泄漏四端路由结构与栏目名(前端铁律 2 自证)
//
// bundle-boundary 依赖 frontend/apps/web/dist,需先跑 pnpm build。
import { execFileSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import { join } from 'node:path'

const ROOT = process.cwd()
const CHECKS = [
  { name: 'route-matrix', title: '后端路由 ↔ openapi 契约一致性' },
  { name: 'sdk-matrix', title: 'api-client ↔ 后端路由双向可达性' },
  { name: 'role-guard-crosscheck', title: '角色页面 ↔ 接口守卫相容性' },
  { name: 'sim-catalog-drift', title: '内置仿真包清单 ↔ sim-sdk 源码一致性' },
  { name: 'bundle-boundary', title: '前端打包边界自证', needsDist: true },
]

let failed = false
for (const check of CHECKS) {
  console.log(`\n${'='.repeat(72)}\n${check.title}  (${check.name})\n${'='.repeat(72)}`)
  if (check.needsDist && !existsSync(join(ROOT, 'frontend/apps/web/dist/assets'))) {
    console.log('跳过:未找到 frontend/apps/web/dist/assets,请先执行 cd frontend && pnpm build')
    failed = true
    continue
  }
  try {
    const out = execFileSync(process.execPath, [join(ROOT, 'scripts/audit', `${check.name}.mjs`)], {
      encoding: 'utf8',
      maxBuffer: 64 * 1024 * 1024,
    })
    console.log(out.trimEnd())
  } catch (error) {
    console.log(error.stdout?.trimEnd() ?? '')
    console.error(error.stderr?.trimEnd() ?? error.message)
    failed = true
  }
}

console.log(`\n${'='.repeat(72)}`)
console.log(failed ? '有检查未能完成,见上方输出。' : '五项检查全部执行完毕,逐项结论见上方输出。')
