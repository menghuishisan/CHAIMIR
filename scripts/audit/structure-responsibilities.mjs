// 工程职责静态门禁:按总-工程目录设计检查后端模块分层与前端目录边界。
// 用法:node scripts/audit/structure-responsibilities.mjs
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const violations = []
const backendModules = path.join(ROOT, 'backend/internal/modules')
const expectedDomains = [
  'admin',
  'content',
  'contest',
  'experiment',
  'grade',
  'identity',
  'judge',
  'notify',
  'sandbox',
  'sim',
  'teaching',
]
const contractMethodsByDomain = {
  content: new Set(['GetContentFace', 'GetContentFull', 'BatchGetContentFace', 'ReplaceUsageRefs', 'GetJudgeSpec', 'SystemImportContent']),
  contest: new Set(['Stats', 'ListStudentAchievements']),
  experiment: new Set(['GetInstanceScore', 'Stats']),
  identity: new Set(['GetAccount', 'BatchGetAccounts', 'HasRole', 'ListClassStudents', 'ListTenants', 'GetTenant', 'PlatformStats', 'TenantStats', 'QueryAuditLogs']),
  judge: new Set(['SubmitJudgeTask', 'ValidateJudgeMode', 'GetJudgeTask', 'CancelJudgeTask', 'Rejudge', 'RejudgeBySourceRef', 'FindExactMatch', 'FindSimilarity']),
  notify: new Set(['Send', 'Push']),
  sandbox: new Set(['ValidateSandboxTemplate', 'CreateSandbox', 'GetSandbox', 'PauseSandbox', 'ResumeSandbox', 'DestroySandbox', 'RecycleBySourceRef', 'PutSandboxFile', 'PutSandboxPrivateArchive', 'RestoreSandboxArchive', 'SaveSandboxFiles', 'ExecSandboxCommand', 'ChainDeploy', 'ChainSendTx', 'ChainQuery', 'ChainReset', 'Stats']),
  sim: new Set(['CreateSession', 'GetReplay', 'ReportCheckpoint', 'DestroySession', 'RecycleBySourceRef']),
  teaching: new Set(['GetCourse', 'GetCourseGrade', 'IsCourseMember', 'ListCourseGrades', 'ListStudentGrades', 'Stats']),
}

/** collectFiles 递归收集指定扩展名的文件。 */
function collectFiles(directory, extensions) {
  if (!fs.existsSync(directory)) return []
  const files = []
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) files.push(...collectFiles(fullPath, extensions))
    else if (extensions.has(path.extname(entry.name))) files.push(fullPath)
  }
  return files
}

/** relative 返回用于审计输出的仓库相对路径。 */
function relative(file) {
  return path.relative(ROOT, file).split(path.sep).join('/')
}

/** sortedDirectoryNames 返回目录下一级子目录的稳定名称集合。 */
function sortedDirectoryNames(directory) {
  return fs
    .readdirSync(directory, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort()
}

for (const moduleName of expectedDomains) {
  const moduleDir = path.join(backendModules, moduleName)
  for (const file of collectFiles(moduleDir, new Set(['.go']))) {
    const rel = relative(file)
    if (rel.includes('/internal/sqlcgen/')) continue
    const name = path.basename(file)
    const source = fs.readFileSync(file, 'utf8')

    for (const match of source.matchAll(/^type ([A-Z][A-Za-z0-9]*DTO)\b/gm)) {
      if (name !== 'dto.go') violations.push(`${rel}: 公共 DTO ${match[1]} 必须放 dto.go`)
    }

    for (const match of source.matchAll(/^func \(s \*Service\) ([A-Za-z0-9_]+)\(/gm)) {
      if (contractMethodsByDomain[moduleName]?.has(match[1]) && name !== 'service_contract.go') {
        violations.push(`${rel}: contracts 方法 ${match[1]} 必须放 service_contract.go`)
      }
    }

    if (
      /func \(s \*Service\)/.test(source) &&
      !/^service(?:_.+)?\.go$/.test(name) &&
      !['audit.go', 'events.go'].includes(name)
    ) {
      violations.push(`${rel}: Service 方法必须放 service*.go、audit.go 或 events.go`)
    }
    if (
      /internal\/modules\/.+\/internal\/sqlcgen/.test(source) &&
      !/^repo(?:_.+)?\.go$/.test(name) &&
      name !== 'row_convert.go'
    ) {
      violations.push(`${rel}: sqlcgen 只能由 repo*.go 或 row_convert.go 使用`)
    }
    if (
      /^api(?:_.+)?\.go$/.test(name) &&
      /internal\/platform\/(?:db|eventbus)|internal\/sqlcgen/.test(source)
    ) {
      violations.push(`${rel}: API 层不得直接依赖 db、eventbus 或 sqlcgen`)
    }
    if (['enum.go', 'dto.go'].includes(name) && /^func /m.test(source)) {
      violations.push(`${rel}: ${name} 只放枚举或 DTO,不得放顶层函数`)
    }
    if (/^func (?:cloneMap|cloneAnyMap|cloneMapStrict|formatTime|formatOptionalTime|stringMapFromAny|mapAny|stringFromAny|stringValue|sliceValue)\(/m.test(source)) {
      violations.push(`${rel}: 不得复制 jsonx/timex 小工具,应直接复用共享能力`)
    }
    if (/strconv\.FormatInt\(/.test(source)) {
      violations.push(`${rel}: 雪花 ID 字符串必须复用 ids.Format`)
    }
    for (const match of source.matchAll(/chaimir\/internal\/modules\/([^/'"]+)/g)) {
      if (match[1] !== moduleName) {
        violations.push(`${rel}: 禁止直接 import 其他业务模块 ${match[1]}`)
      }
    }
  }
}

for (const file of collectFiles(path.join(ROOT, 'backend'), new Set(['.go']))) {
  const rel = relative(file)
  if (rel.includes('/internal/sqlcgen/') || rel.endsWith('_test.go') || rel.endsWith('/internal/platform/timex/timex.go')) continue
  const source = fs.readFileSync(file, 'utf8')
  if (/\.Format\(time\.RFC3339\)/.test(source) || /time\.Now\(\)\.UTC\(\)/.test(source)) {
    violations.push(`${rel}: UTC/RFC3339 边界必须复用 timex`)
  }
}

const actualBackendDomains = sortedDirectoryNames(backendModules)
if (actualBackendDomains.join(',') !== [...expectedDomains].sort().join(',')) {
  violations.push(`backend/internal/modules: 模块目录应为 ${expectedDomains.join(',')}`)
}

const webSource = path.join(ROOT, 'frontend/apps/web/src')
const routesSource = path.join(webSource, 'routes')
const featuresSource = path.join(webSource, 'features')
const attachmentDownloadHelper = path.join(webSource, 'utils/downloadAttachment.ts')
for (const file of collectFiles(routesSource, new Set(['.ts', '.tsx']))) {
  const source = fs.readFileSync(file, 'utf8')
  if (/from ['"].*app\/api['"]|\bapi\.[A-Za-z]/.test(source)) {
    violations.push(`${relative(file)}: routes 只做路由与权限装配,不得调用业务 API`)
  }
}

for (const file of collectFiles(webSource, new Set(['.ts', '.tsx']))) {
  const source = fs.readFileSync(file, 'utf8')
  if (/\bfetch\(|\baxios\.|XMLHttpRequest/.test(source)) {
    violations.push(`${relative(file)}: 页面不得绕过 api-client 直接发起 HTTP 请求`)
  }
  if (
    file !== attachmentDownloadHelper &&
    (/URL\.createObjectURL\(/.test(source) || /\.download\s*=/.test(source))
  ) {
    violations.push(`${relative(file)}: 浏览器附件下载必须复用 utils/downloadAttachment`)
  }
}

for (const file of collectFiles(featuresSource, new Set(['.ts', '.tsx']))) {
  const source = fs.readFileSync(file, 'utf8')
  if (/['\"`]\/api\/v1\//.test(source)) {
    violations.push(`${relative(file)}: 页面不得散落 /api/v1 路径,应复用 api-client 契约常量或模块方法`)
  }
}

const uiSource = path.join(ROOT, 'frontend/packages/ui/src')
for (const file of collectFiles(uiSource, new Set(['.ts', '.tsx']))) {
  const source = fs.readFileSync(file, 'utf8')
  if (/@chaimir\/api-client|frontend\/apps\/web|\/api\/v1/.test(source)) {
    violations.push(`${relative(file)}: packages/ui 不得依赖应用层或后端 API`)
  }
}

const featureDomains = sortedDirectoryNames(path.join(webSource, 'features'))
const expectedFeatureDomains = [...expectedDomains, 'transfer'].sort()
if (featureDomains.join(',') !== expectedFeatureDomains.join(',')) {
  violations.push(`frontend/apps/web/src/features: 应为 11 个业务域加 transfer,当前 ${featureDomains.join(',')}`)
}

const labelDomains = fs
  .readdirSync(path.join(webSource, 'utils/labels'))
  .filter((name) => name.endsWith('.ts'))
  .map((name) => path.basename(name, '.ts'))
  .sort()
if (labelDomains.join(',') !== expectedFeatureDomains.join(',')) {
  violations.push(`frontend/apps/web/src/utils/labels: 必须按业务域拆分,当前 ${labelDomains.join(',')}`)
}

if (violations.length > 0) {
  console.error('工程职责检查失败:')
  for (const violation of violations) console.error(`- ${violation}`)
  process.exitCode = 1
} else {
  console.log('工程职责检查通过:后端模块分层、跨模块依赖、前端路由/HTTP/UI 包边界和领域目录均符合规范。')
}
