/**
 * 前端规范静态门禁:检查业务源码中的高风险样式和开发者文案残留。
 * 设计令牌源文件允许原始色值;交互状态选择器(data-/aria-)不属于任意样式值。
 * 脚本固定归属 scripts/audit,并从文件位置推导仓库根,不依赖当前工作目录。
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const roots = [path.join(ROOT, 'frontend/apps/web/src'), path.join(ROOT, 'frontend/packages/ui/src')]
const extensions = new Set(['.ts', '.tsx', '.js', '.jsx', '.css'])
const violations = []

function collectFiles(directory) {
  if (!fs.existsSync(directory)) return []
  const files = []
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) files.push(...collectFiles(fullPath))
    else if (extensions.has(path.extname(entry.name))) files.push(fullPath)
  }
  return files
}

function addViolation(file, lineNumber, rule, match) {
  violations.push(`${path.relative(ROOT, file)}:${lineNumber} ${rule}: ${match.trim()}`)
}

for (const root of roots) {
  for (const file of collectFiles(root)) {
    if (file.endsWith(`${path.sep}tokens${path.sep}theme.css`)) continue
    const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/)
    lines.forEach((line, index) => {
      const lineNumber = index + 1
      if (/radial-gradient|conic-gradient|\bbg-(?:radial|conic)\b/i.test(line)) addViolation(file, lineNumber, '禁止径向/锥形渐变', line)
      if (/\btracking-(?:tighter|tight|normal|wide|wider|widest)\b/.test(line)) addViolation(file, lineNumber, '字距必须由正文令牌统一控制', line)
      if (/(?:^|["'`\s])#[0-9a-f]{3,8}\b/i.test(line)) addViolation(file, lineNumber, '业务源码不得直接写颜色值', line)
      if (/\b(?:[a-z][a-z0-9-]*)-\[[^\]]+\]/i.test(line) && !/\b(?:data|aria|group|peer)-\[[^\]]+\]/i.test(line)) {
        addViolation(file, lineNumber, '不得使用 Tailwind 任意值类', line)
      }
      if (/transition\s*:\s*all\b/i.test(line)) addViolation(file, lineNumber, '禁止 transition: all', line)
      if (/[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u.test(line)) addViolation(file, lineNumber, '界面源码不得使用 emoji', line)
    })
  }
}

if (violations.length > 0) {
  console.error('前端规范检查失败:')
  for (const violation of violations) console.error(`- ${violation}`)
  process.exitCode = 1
} else {
  console.log('前端规范检查通过:未发现禁用渐变、字距类、裸色值、任意值类、transition: all 或 emoji。')
}
