// 代码生成脚本:把 @chaimir/sim-sdk 的内置仿真包导出为后端可入库的 sim-package.json 清单。
//
// 为什么需要它:内置仿真包的权威声明(meta / interactions / render / narrative / codeTrace /
// checkpoints)是 TypeScript 源码,只有前端能求值;而 `GET /sim/packages` 读的是数据库
// sim_package 表 —— 平台内置包必须在部署期入库,否则生产环境的仿真实验室、实验编排与课时
// 仿真形态全都取不到场景。手写第二份 Go 清单会立刻与 package.ts 漂移,故用本脚本从唯一
// 真相源导出,由后端 go:embed 后在 seed 阶段幂等入库。
//
// 导出形态就是上传扩展包时的 `sim-package.json`:后端用同一个 parseBundleManifest 解析,
// 内置包与扩展包走完全相同的校验口径,不存在第二套解析逻辑。
//
// 用法(在仓库根):node scripts/codegen/export-sim-builtin-catalog.mjs
// 产物已提交入库,改动内置包后必须重跑本脚本并提交产物;
// `scripts/audit/sim-catalog-drift.mjs` 会校验产物与源码一致。
import { createRequire } from 'node:module'
import { writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const REPO_ROOT = resolve(fileURLToPath(new URL('../..', import.meta.url)))
const SDK_ROOT = join(REPO_ROOT, 'frontend/packages/sim-sdk')
const OUTPUT = join(REPO_ROOT, 'backend/internal/modules/sim/builtin_catalog.json')

/** BUILTIN_CODE_PREFIX 与 sim-sdk 的 BUILTIN_SIM_CODE_PREFIX、后端 builtinSimCodePrefix 同值。 */
const BUILTIN_CODE_PREFIX = 'builtin__'

/**
 * loadVite 从 Web 应用工作区解析 Vite 的 ESM 入口。
 * 本脚本住在仓库根的 scripts/(全仓脚本唯一落点),而 Vite 是 apps/web 的构建依赖,
 * 故按 frontend/apps/web/package.json 的解析上下文取包路径;
 * 直接 `import('vite')` 会命中它的 CJS 入口(exports.require 分支),那个入口不导出 createServer。
 */
async function loadVite() {
  const requireFromWebApp = createRequire(join(REPO_ROOT, 'frontend/apps/web/package.json'))
  const manifest = requireFromWebApp.resolve('vite/package.json')
  const esmEntry = join(dirname(manifest), 'dist/node/index.js')
  return import(pathToFileURL(esmEntry).href)
}

/**
 * loadSdk 在 Vite 的 SSR 模块图里求值 sim-sdk 源码。
 * 内置包内核是 TS 且互相 import,直接用 node 跑不了;借 Vite 转译比自建 tsc 流程更短。
 */
async function loadSdk() {
  const { createServer } = await loadVite()
  const server = await createServer({ configFile: false, logLevel: 'error', root: SDK_ROOT })
  try {
    const catalog = await server.ssrLoadModule('./src/builtin/catalog.ts')
    const manifest = await server.ssrLoadModule('./src/authoring/manifest.ts')
    const validation = await server.ssrLoadModule('./src/validation.ts')
    return {
      packages: catalog.builtinSimulations,
      createSimPackageManifest: manifest.createSimPackageManifest,
      validateSimPackage: validation.validateSimPackage,
    }
  } finally {
    await server.close()
  }
}

/**
 * main 校验并导出全部内置包清单,按 code 排序写盘。
 *
 * 先跑与运行时同一套 validateSimPackage:Worker 装配内置包时会调 assertValidSimPackage,
 * 协议不合格的包在学生打开场景那一刻才抛错(曾漏出 3 个"视图未声明职责"的包)。
 * 导出是内置包唯一的必经关口,把这层校验放在这里,让协议缺陷在提交前就暴露。
 * 排序保证产物字节稳定,漂移校验才能只看内容不看顺序。
 */
async function main() {
  const { packages, createSimPackageManifest, validateSimPackage } = await loadSdk()
  if (!Array.isArray(packages) || packages.length === 0) {
    throw new Error('内置仿真包清单为空,请检查 frontend/packages/sim-sdk/src/builtin/catalog.ts')
  }

  const protocolIssues = []
  for (const pkg of packages) {
    const result = validateSimPackage(pkg)
    if (!result.ok) {
      protocolIssues.push(
        `${pkg.meta.code}: ${result.issues.map((issue) => `${issue.path} → ${issue.message}`).join(' | ')}`,
      )
    }
  }
  if (protocolIssues.length > 0) {
    throw new Error(`内置仿真包协议校验未通过(这些包在浏览器里装配即失败):\n  ${protocolIssues.join('\n  ')}`)
  }

  const manifests = packages.map((pkg) => createSimPackageManifest(pkg))
  const codes = new Set()
  for (const item of manifests) {
    if (!item.meta.code.startsWith(BUILTIN_CODE_PREFIX)) {
      // 内置标准库的命名空间是入库时 author_type=平台内置 的依据(后端 SyncBuiltinPackages),
      // 前缀不符会让包以错误的作者类型入库,进而落到错误的执行位置。
      throw new Error(`内置仿真包 ${item.meta.code} 缺少 ${BUILTIN_CODE_PREFIX} 前缀`)
    }
    if (item.meta.entry !== undefined) {
      // entry 只对扩展包有意义(隔离容器据此装配);内置包由 sim-sdk registry 按 code 装配,
      // 声明 entry 会让人误以为内置包也走归档装配路径。
      throw new Error(`内置仿真包 ${item.meta.code} 不应声明 entry`)
    }
    if (codes.has(item.meta.code)) {
      throw new Error(`内置仿真包 code 重复: ${item.meta.code}`)
    }
    codes.add(item.meta.code)
  }
  manifests.sort((a, b) => (a.meta.code < b.meta.code ? -1 : a.meta.code > b.meta.code ? 1 : 0))

  const payload = {
    source: 'frontend/packages/sim-sdk/src/builtin/catalog.ts',
    generator: 'scripts/codegen/export-sim-builtin-catalog.mjs',
    packages: manifests,
  }
  writeFileSync(OUTPUT, `${JSON.stringify(payload, null, 2)}\n`, 'utf8')
  console.log(`已导出 ${manifests.length} 个内置仿真包 → ${OUTPUT}`)
}

await main()
