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

/**
 * 边框圈盒规则的豁免组件(规范 §6.5.1):
 * - 浮层(Modal/Drawer/Menu/Popover/Toast)不在光面容器层次里,可用发丝边把自己从未知背景上分离;
 * - 输入控件(Input/Textarea/Select/Checkbox/Radio/Switch/SegmentedControl)的边框是控件自身的形,
 *   规则本来就只禁止「用边框圈出一个容器」。
 */
const BORDER_EXEMPT_COMPONENTS = [
  'Modal',
  'Drawer',
  'Menu',
  'Popover',
  'Toast',
  'Input',
  'Textarea',
  'Select',
  'Checkbox',
  'Radio',
  'Switch',
  'SegmentedControl',
]

/** isBorderExempt 判断文件是否为上述豁免组件的实现。 */
function isBorderExempt(file) {
  const normalized = file.split(path.sep).join('/')
  return BORDER_EXEMPT_COMPONENTS.some((name) => normalized.includes(`/components/${name}/`))
}

/* ---------- 令牌类拼错检查(FE-1) ----------
   Tailwind v4 对认不出的颜色类**不生成任何 CSS**,于是 `text-on-dark-accent`(令牌其实叫 accent)
   或 `bg-terminal-bg`(令牌其实叫 terminal)这类拼错会静默失效:类名还在、颜色没了,
   静态检查与构建都不报错,只能靠眼睛在深色页面上发现「图标怎么是灰的」。故在此比对令牌表。 */

/** 项目令牌族前缀:只校验这些前缀开头的类名,避免误伤 Tailwind 内置调色板(如 text-red-500)。 */
const TOKEN_FAMILIES = [
  'ink',
  'on-dark',
  'on-solid',
  'dark',
  'paper',
  'jade',
  'cinnabar',
  'seal',
  'primary',
  'accent',
  'danger',
  'warning',
  'info',
  'success',
  'line',
  'surface',
  'canvas',
  'substrate',
  'terminal',
]

/** Tailwind 的通用颜色关键字,不来自令牌表。 */
const COLOR_KEYWORDS = new Set(['transparent', 'current', 'inherit', 'white', 'black', 'none'])

/** 可接受颜色令牌的工具类前缀。 */
const COLOR_UTILITY_RE =
  /\b(?:text|bg|border|fill|stroke|divide|ring|outline|from|to|via|accent|placeholder|caret|shadow)-([a-z][a-z0-9-]*)\b/g

/** 从令牌源文件收集 --color-* 名称;它是颜色类合法性的唯一真相源。 */
function collectColorTokens() {
  const themeFile = path.join(ROOT, 'frontend/packages/ui/src/tokens/theme.css')
  if (!fs.existsSync(themeFile)) return new Set()
  const source = fs.readFileSync(themeFile, 'utf8')
  return new Set([...source.matchAll(/--color-([a-z0-9-]+):/g)].map((match) => match[1]))
}

const colorTokens = collectColorTokens()

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
      // 规范 §1.2 explicitly permits the auth surface's documented halo; all other radial/conic gradients remain forbidden.
      const isDocumentedAuthHalo = file.endsWith(`${path.sep}tokens${path.sep}base.css`) && lines.slice(Math.max(0, index - 6), index + 1).some((contextLine) => contextLine.includes('.auth-intro-glow'))
      if (!isDocumentedAuthHalo && /radial-gradient|conic-gradient|\bbg-(?:radial|conic)\b/i.test(line)) addViolation(file, lineNumber, '禁止径向/锥形渐变', line)
      if (/\btracking-(?:tighter|tight|normal|wide|wider|widest)\b/.test(line)) addViolation(file, lineNumber, '字距必须由正文令牌统一控制', line)
      if (/(?:^|["'`\s])#[0-9a-f]{3,8}\b/i.test(line)) addViolation(file, lineNumber, '业务源码不得直接写颜色值', line)
      if (/\b(?:[a-z][a-z0-9-]*)-\[[^\]]+\]/i.test(line) && !/\b(?:data|aria|group|peer)-\[[^\]]+\]/i.test(line)) {
        addViolation(file, lineNumber, '不得使用 Tailwind 任意值类', line)
      }
      if (/transition\s*:\s*all\b/i.test(line)) addViolation(file, lineNumber, '禁止 transition: all', line)
      if (/[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u.test(line)) addViolation(file, lineNumber, '界面源码不得使用 emoji', line)
      // 规范 §6.5.1:光面内的容器不画边框,层次靠底色与落影;border 只用于分隔线与输入控件。
      // 判据是「圆角 + 四边边框」同时出现 —— 那就是在圈一个盒子;分隔线用的是 border-b/t/l/r,不命中。
      if (
        !isBorderExempt(file) &&
        /\brounded-(?:sm|md|lg|xl|2xl|3xl|full|pane)\b/.test(line) &&
        /\bborder\s+border-(?:line|line-strong)\b/.test(line)
      ) {
        addViolation(file, lineNumber, '光面内的容器不得用边框圈盒(改用 well 或 bg-surface+shadow-xs)', line)
      }
      // 令牌类必须真的存在,否则 Tailwind 静默不出样式(见文件上方说明)
      for (const match of line.matchAll(COLOR_UTILITY_RE)) {
        const name = match[1]
        if (COLOR_KEYWORDS.has(name) || colorTokens.has(name)) continue
        if (!TOKEN_FAMILIES.some((family) => name === family || name.startsWith(`${family}-`))) continue
        addViolation(file, lineNumber, `颜色令牌不存在,该类不会生成任何样式: ${match[0]}`, line)
      }
    })
  }
}

if (violations.length > 0) {
  console.error('前端规范检查失败:')
  for (const violation of violations) console.error(`- ${violation}`)
  process.exitCode = 1
} else {
  console.log(
    '前端规范检查通过:未发现禁用渐变、字距类、裸色值、任意值类、transition: all、emoji、边框圈盒或不存在的颜色令牌类。',
  )
}
