/**
 * 前端规范静态门禁:检查业务源码中的高风险样式和开发者文案残留。
 * 设计令牌源文件允许原始色值;交互状态选择器(data-/aria-)不属于任意样式值。
 * 脚本固定归属 scripts/audit,并从文件位置推导仓库根,不依赖当前工作目录。
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const roots = [
  path.join(ROOT, 'frontend/apps/web/src'),
  path.join(ROOT, 'frontend/packages/ui/src'),
  path.join(ROOT, 'frontend/packages/ide/src'),
]
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

/** CSS 变量读取也必须命中同一令牌源,避免引擎包绕开 Tailwind 类名门禁后静默读到空值。 */
function checkColorVariableReferences(file, line, lineNumber) {
  for (const match of line.matchAll(/['"]--color-([a-z0-9-]+)['"]/g)) {
    if (!colorTokens.has(match[1])) {
      addViolation(file, lineNumber, `颜色令牌不存在: ${match[0]}`, line)
    }
  }
}

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

/** checkLabelsResponsibility 保证 labels 目录只维护用户向文案映射。 */
function checkLabelsResponsibility(file, lines) {
  const labelsRoot = `${path.sep}apps${path.sep}web${path.sep}src${path.sep}utils${path.sep}labels${path.sep}`
  if (!file.includes(labelsRoot)) return

  lines.forEach((line, index) => {
    const lineNumber = index + 1
    if (/from ['"]@chaimir\/ui['"]/.test(line)) {
      addViolation(file, lineNumber, 'labels 不得依赖 UI 呈现类型', line)
    }
    if (/\b(?:StatusTone|BadgeTone|CoverAccent|ReadonlySet|new Set)\b/.test(line)) {
      addViolation(file, lineNumber, 'labels 只维护文案,不得承载呈现或业务规则', line)
    }
    if (/^export\s+function\s+(?:is[A-Z]|[A-Za-z0-9]+Tone\b)/.test(line)) {
      addViolation(file, lineNumber, 'labels 不得导出业务判断或 tone 函数', line)
    }
    if (/^(?:export\s+)?(?:type|interface)\b/.test(line)) {
      addViolation(file, lineNumber, 'labels 只维护用户向文案,不得定义业务或表单类型', line)
    }
    if (/^export\s+const\s+[A-Z0-9_]+\s*=\s*\[/.test(line)) {
      addViolation(file, lineNumber, 'labels 不得导出表单选项数组', line)
    }
  })
}

for (const root of roots) {
  for (const file of collectFiles(root)) {
    if (file.endsWith(`${path.sep}tokens${path.sep}theme.css`)) continue
    const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/)
    checkLabelsResponsibility(file, lines)
    lines.forEach((line, index) => {
      const lineNumber = index + 1
      checkColorVariableReferences(file, line, lineNumber)
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
      // 复合工具类的底色由规范锁定,不接受覆盖(§6.5.1 井色、§4 骨架微光)。
      // 它们是多属性复合类,tailwind-merge 无法只摘掉其中的 background —— 若登记冲突会把整个类移除,
      // 导致「只想改圆角」的调用连底色一起丢。故在此拦截:同一 className 里不得与 bg-* 并存。
      // 判据只看同一行:这两个类都是就地书写,不经跨行拼装(见 packages/ui/src/lib/cn.ts 说明)。
      for (const composite of ['well', 'skeleton-shimmer']) {
        const hasComposite = new RegExp(`(?:^|["'\`\\s])${composite}(?=["'\`\\s]|$)`).test(line)
        if (hasComposite && /\bbg-[a-z][a-z0-9-]*\b/.test(line)) {
          addViolation(
            file,
            lineNumber,
            `${composite} 的底色由规范锁定,不得同时写 bg-*(需要别的底色就别用这个工具类)`,
            line,
          )
        }
      }
      // 令牌类必须真的存在,否则 Tailwind 静默不出样式(见文件上方说明)
      for (const match of line.matchAll(COLOR_UTILITY_RE)) {
        const name = match[1]
        if (COLOR_KEYWORDS.has(name) || colorTokens.has(name)) continue
        if (!TOKEN_FAMILIES.some((family) => name === family || name.startsWith(`${family}-`))) continue
        addViolation(file, lineNumber, `颜色令牌不存在,该类不会生成任何样式: ${match[0]}`, line)
      }
    })

    checkPageFamilyRules(file, lines)
  }
}

/**
 * checkPageFamilyRules 落两条页面族谱门禁(规范 §6.5.0 通则 1 / §6.5.2)。
 * 逐文件而非逐行:两条规则都要看整份文件的结构。
 */
/**
 * filterBarOccurrences 定位每一处 `<FilterBar`,并判断它是否 bare、是否落在 DataPanel 内部。
 *
 * 「落在 DataPanel 内部」用 `<DataPanel` 与 `</DataPanel>` 的配对深度判定 ——
 * 逐处判定才拦得住「同页一处合规、另一处摆在光面上」的情形。
 * bare 只认写在这一处 FilterBar 开标签里的 `bare`,不认文件里别处出现的同名词。
 */
function filterBarOccurrences(source) {
  const out = []
  let depth = 0
  let cursor = 0
  const token = /<DataPanel\b|<\/DataPanel>|<FilterBar\b/g
  let match = token.exec(source)
  while (match !== null) {
    cursor = match.index
    if (match[0] === '<DataPanel') depth += 1
    else if (match[0] === '</DataPanel>') depth = Math.max(0, depth - 1)
    else {
      // 取这一处 FilterBar 开标签的属性文本:到该元素第一个顶层 > 为止
      let braces = 0
      let end = cursor + '<FilterBar'.length
      for (; end < source.length; end += 1) {
        const ch = source[end]
        if (ch === '{') braces += 1
        else if (ch === '}') braces -= 1
        else if (ch === '>' && braces === 0) break
      }
      const attrs = source.slice(cursor, end + 1)
      out.push({ index: cursor, bare: /\bbare\b/.test(attrs), insideDataPanel: depth > 0 })
    }
    match = token.exec(source)
  }
  return out
}

/**
 * floatingLayerViolations 定位浮层(Modal / Drawer)内部出现的分页与指标带(§6.5.5 A 禁止)。
 * 用配对深度判定所处层级:深度 > 0 即在浮层内。
 */
function floatingLayerViolations(source) {
  const out = []
  let depth = 0
  const token = /<(?:Modal|Drawer)\b|<\/(?:Modal|Drawer)>|<(Pagination|MetricStrip|Stat)\b/g
  let match = token.exec(source)
  while (match !== null) {
    if (match[1] !== undefined) {
      if (depth > 0) out.push({ index: match.index, tag: match[1] })
    } else if (match[0].startsWith('</')) {
      depth = Math.max(0, depth - 1)
    } else {
      depth += 1
    }
    match = token.exec(source)
  }
  return out
}

/**
 * elementAttrs 取出每个 `<Name …>` 开标签的属性文本(按顶层花括号深度找该元素自己的收尾)。
 * 不能简单用 `[^>]*`:`rowKey={() => ''}` 这类属性里就带 `>`。
 */
function elementAttrs(source, name) {
  const out = []
  const opener = new RegExp(`<${name}(?=[\\s/>])`, 'g')
  let match = opener.exec(source)
  while (match !== null) {
    let braces = 0
    let end = match.index + name.length + 1
    for (; end < source.length; end += 1) {
      const ch = source[end]
      if (ch === '{') braces += 1
      else if (ch === '}') braces -= 1
      else if (ch === '>' && braces === 0) break
    }
    out.push({ index: match.index, attrs: source.slice(match.index, end + 1) })
    opener.lastIndex = end
    match = opener.exec(source)
  }
  return out
}

/** enclosedBy 判断某位置是否落在给定容器的配对深度内(同文件范围)。 */
function enclosedBy(source, index, names) {
  const token = new RegExp(`<(?:${names.join('|')})\\b|</(?:${names.join('|')})>`, 'g')
  let depth = 0
  let match = token.exec(source)
  while (match !== null && match.index < index) {
    if (match[0].startsWith('</')) depth = Math.max(0, depth - 1)
    else depth += 1
    match = token.exec(source)
  }
  return depth > 0
}

function checkPageFamilyRules(file, lines) {
  // 只查业务页面,设计系统自身不适用(FilterBar/DataPanel 的定义就在包里)
  if (!file.includes(`${path.sep}apps${path.sep}web${path.sep}`)) return
  const source = lines.join('\n')

  // ① §6.5.2:FilterBar 的井必须落在抬起片内部 —— 表格型列表页经 `DataPanel` 的 filter 槽位。
  //    卡片网格型列表页的数据区本身就是一排 Card(抬起片),塞进 DataPanel 会成为片里套片,
  //    故那类页面走 §6.5.1 红线的另一条出路:`bare` 无底形态直接排在光面上(不带井色)。
  //    两条出路之外的写法(井色摆在光面上)才是违规。
  //
  //    判定逐处而不是逐文件:同一页可能既有落在 DataPanel 里的筛选,又有另一条摆在光面上的 ——
  //    只看「文件里出现过 DataPanel」会把后者放过去(实时监控页就曾这样漏掉一处)。
  for (const bar of filterBarOccurrences(source)) {
    if (bar.bare || bar.insideDataPanel) continue
    const lineNumber = source.slice(0, bar.index).split('\n').length
    addViolation(
      file,
      lineNumber,
      'FilterBar 的井不得摆在光面上(§6.5.2):表格型进 DataPanel 的 filter 槽位,卡片网格型用 bare 无底形态',
      '<FilterBar',
    )
  }

  // ② §6.5.5 A:浮层里不出现分页与指标带 —— 需要这两样说明它其实是一页,应当改成整页并归族。
  //    典型误用是给弹窗里的下拉配一条分页(等于让用户先翻页才能选到人),
  //    以及把分页历史塞进弹窗。判定按 Modal/Drawer 的配对深度逐处做。
  for (const hit of floatingLayerViolations(source)) {
    const lineNumber = source.slice(0, hit.index).split('\n').length
    addViolation(
      file,
      lineNumber,
      `浮层内不得出现 ${hit.tag}(§6.5.5 A):需要分页或指标带说明它其实是一页,应改成整页并归族`,
      `<${hit.tag}`,
    )
  }

  // ③ §6.5.1「不出现第三级」:落在抬起片内部的 Table 必须传 elevated={false},
  //    否则它自己再画一层底色与落影 —— 片里套片。判定按同文件的配对深度做;
  //    「组件根节点是井、而包住它的 ModalBody 在调用方」那种跨文件情形静态判不了,留走查
  //    (本轮 ImportPreviewPanel 就是这么查出来的)。
  for (const t of elementAttrs(source, 'Table')) {
    if (/elevated=\{false\}/.test(t.attrs)) continue
    if (!enclosedBy(source, t.index, ['CardBody', 'ModalBody', 'DataPanel'])) continue
    const lineNumber = source.slice(0, t.index).split('\n').length
    addViolation(
      file,
      lineNumber,
      'Table 落在抬起片内部必须传 elevated={false}(§6.5.1 不出现第三级)',
      '<Table',
    )
  }

  // ④ §6.5.0 通则 1:面包屑末节与 h1 同名等于白占一行,面包屑只到父级。
  //    两种同名都要抓:字面量相等(`label: '成绩审核'` vs `title="成绩审核"`),
  //    以及**同一个表达式**(`label: copy.title` vs `title={copy.title}`)——
  //    后者曾漏掉学校/平台告警页与实验编排向导两处,故不再只比字面量。
  //    真正比不了的是「两个不同表达式恰好求出同一个值」,那只能走查。
  const crumbMatch = source.match(/<Breadcrumb\s+items=\{\[([\s\S]*?)\]\}/)
  if (crumbMatch) {
    const labels = [...crumbMatch[1].matchAll(/label:\s*([^,}\n]+)/g)].map((m) => m[1].trim())
    const lastLabel = labels.length > 0 ? labels[labels.length - 1] : undefined
    // 标题两种写法:字面量 title="X" 与表达式 title={expr}
    const literalTitle = source.match(/\btitle="([^"]+)"/)
    const exprTitle = source.match(/\btitle=\{([^\n]+?)\}\s*$/m)
    const titleForms = [
      literalTitle ? `'${literalTitle[1]}'` : undefined,
      exprTitle ? exprTitle[1].trim() : undefined,
    ].filter(Boolean)

    if (lastLabel !== undefined && titleForms.includes(lastLabel)) {
      const lineNumber = lines.findIndex((line) => line.includes('<Breadcrumb')) + 1
      addViolation(
        file,
        lineNumber,
        `面包屑末节 ${lastLabel} 与页面标题同名(§6.5.0 通则 1),面包屑只到父级`,
        crumbMatch[0].split('\n')[0],
      )
    }
  }
}

if (violations.length > 0) {
  console.error('前端规范检查失败:')
  for (const violation of violations) console.error(`- ${violation}`)
  process.exitCode = 1
} else {
  console.log(
    '前端规范检查通过:未发现禁用样式、错误颜色令牌或 labels 目录职责越界。',
  )
}
