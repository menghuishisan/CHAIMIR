// validation 统一校验仿真包声明和教学画面，供 authoring 与 runtime 共同复用。

import type { SimPackage, SimState, TeachingFrame, TreeNode } from './types'

export interface SimPackageValidationIssue {
  path: string
  message: string
}

export interface SimPackageValidationResult {
  ok: boolean
  issues: SimPackageValidationIssue[]
}

const payloadKeyPattern = /^[A-Za-z][A-Za-z0-9_.:-]{0,63}$/
const reservedPayloadParams = new Set(['target', 'active', 'phase', 'startX', 'startY', 'currentX', 'currentY', 'deltaX', 'deltaY'])

/** validateSimPackage 校验仿真包声明、初始状态和首帧是否满足统一运行契约。 */
export function validateSimPackage<TState extends SimState>(simPackage: SimPackage<TState>): SimPackageValidationResult {
  const issues: SimPackageValidationIssue[] = []
  validateMeta(simPackage, issues)
  validateInteractions(simPackage, issues)
  validateNarrative(simPackage, issues)
  validateCodeTrace(simPackage, issues)
  validateCheckpoints(simPackage, issues)
  validateInitialRender(simPackage, issues)
  return { ok: issues.length === 0, issues }
}

/** assertValidSimPackage 在运行时拒绝结构缺失或违反统一契约的动态模块。 */
export function assertValidSimPackage(value: unknown): asserts value is SimPackage {
  if (!isSimPackageShape(value)) {
    throw new Error('仿真包内容不完整，请联系发布者处理')
  }
  const result = validateSimPackage(value)
  if (!result.ok) {
    throw new Error(`仿真包协议不完整:${result.issues.map((issue) => `${issue.path}:${issue.message}`).join(';')}`)
  }
}

/** assertValidTeachingFrame 对每个运行帧执行结构、引用和规模上限校验。 */
export function assertValidTeachingFrame(view: TeachingFrame, limits: SimPackage['meta']['scaleLimit']): void {
  const issues = validateTeachingFrame(view, limits)
  if (issues.length > 0) {
    throw new Error(issues[0]?.message ?? '仿真教学画面不完整，请联系发布者处理')
  }
}

/** validateTeachingFrame 返回教学画面的结构和容量问题，供开发期与运行时共用。 */
export function validateTeachingFrame(
  view: TeachingFrame,
  limits?: SimPackage['meta']['scaleLimit']
): SimPackageValidationIssue[] {
  const issues: SimPackageValidationIssue[] = []
  if (!view.summary || !view.phase?.id || !view.phase.title || !view.layout?.primary || !view.focus?.primary?.length) {
    issues.push({ path: 'render', message: '仿真教学画面缺少摘要、阶段、焦点或主视图声明。' })
  }
  if (!Array.isArray(view.patterns) || view.patterns.length < 1 || view.patterns.length > 3) {
    issues.push({ path: 'render.patterns', message: '仿真视图数量必须为 1 到 3 个。' })
    return issues
  }

  const patternIds = new Set<string>()
  for (const pattern of view.patterns) {
    if (!pattern.id || patternIds.has(pattern.id)) {
      issues.push({ path: 'render.patterns', message: '仿真教学画面包含空白或重复的视图编号。' })
    }
    patternIds.add(pattern.id)
  }
  const layoutIds = layoutPatternIds(view)
  for (const id of layoutIds) {
    if (!patternIds.has(id)) {
      issues.push({ path: 'render.layout', message: '仿真教学画面引用了不存在的视图。' })
    }
  }
  for (const pattern of view.patterns) {
    if (!layoutIds.has(pattern.id)) {
      issues.push({ path: `render.patterns.${pattern.id}`, message: '仿真教学画面存在未声明职责的视图。' })
    }
    if (limits) validatePatternLimit(pattern, limits, issues)
  }
  return issues
}

/** isSimPackageShape 在调用包内函数前完成最小安全形状检查。 */
function isSimPackageShape(value: unknown): value is SimPackage {
  if (!value || typeof value !== 'object') return false
  const pkg = value as Partial<SimPackage>
  return typeof pkg.initState === 'function'
    && typeof pkg.reducer === 'function'
    && typeof pkg.render === 'function'
    && Boolean(pkg.meta)
    && Array.isArray(pkg.interactions)
    && Array.isArray(pkg.narrative)
    && Array.isArray(pkg.checkpoints)
    && Boolean(pkg.codeTrace)
}

/** validateMeta 校验包元数据、教学说明和规模限制。 */
function validateMeta<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimPackageValidationIssue[]): void {
  const meta = simPackage.meta
  if (!meta?.code?.trim() || !meta.name?.trim() || !meta.category?.trim() || !meta.version?.trim() || !meta.summary?.trim()) {
    issues.push({ path: 'meta', message: '仿真包元数据不完整。' })
  }
  if (meta?.compute !== 'frontend' && meta?.compute !== 'backend') {
    issues.push({ path: 'meta.compute', message: '计算模式必须声明为 frontend 或 backend。' })
  }
  if (!Array.isArray(meta?.learningObjectives) || meta.learningObjectives.length === 0) {
    issues.push({ path: 'meta.learningObjectives', message: '仿真包必须声明教学目标。' })
  }
  const limits = meta?.scaleLimit
  if (!limits || !Number.isSafeInteger(limits.nodes) || !Number.isSafeInteger(limits.maxTick) || !Number.isSafeInteger(limits.maxEvents) || limits.nodes <= 0 || limits.maxTick <= 0 || limits.maxEvents <= 0) {
    issues.push({ path: 'meta.scaleLimit', message: '必须声明正整数规模上限。' })
  }
}

/** validateInteractions 校验交互声明、参数和唯一编号。 */
function validateInteractions<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimPackageValidationIssue[]): void {
  if (simPackage.interactions.length === 0) {
    issues.push({ path: 'interactions', message: '至少声明一个可操作交互。' })
  }
  const ids = new Set<string>()
  for (const [index, interaction] of simPackage.interactions.entries()) {
    if (!interaction.id?.trim() || !interaction.kind || !interaction.emits?.trim() || !interaction.label?.trim() || ids.has(interaction.id)) {
      issues.push({ path: `interactions.${index}`, message: '交互必须包含唯一 id、kind、emits 和 label。' })
    }
    ids.add(interaction.id)
    if (interaction.kind === 'select-element' && interaction.target !== 'element') {
      issues.push({ path: `interactions.${index}.target`, message: '选择元素交互必须声明 target 为 element。' })
    }
    if ((interaction.target === 'element' || interaction.kind === 'select-element') && !interaction.elementFilter) {
      issues.push({ path: `interactions.${index}.elementFilter`, message: '元素交互必须声明 elementFilter。' })
    }
    for (const [fieldIndex, field] of (interaction.params ?? []).entries()) {
      if (!field.name || !field.label || !field.type) {
        issues.push({ path: `interactions.${index}.params.${fieldIndex}`, message: '交互参数必须包含 name、label 和 type。' })
      }
      if (!payloadKeyPattern.test(field.name) || reservedPayloadParams.has(field.name)) {
        issues.push({ path: `interactions.${index}.params.${fieldIndex}.name`, message: '交互参数名不符合操作日志规则或使用了保留字段。' })
      }
    }
  }
}

/** validateNarrative 校验教学叙事步骤及其唯一编号。 */
function validateNarrative<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimPackageValidationIssue[]): void {
  if (!simPackage.narrative.length) {
    issues.push({ path: 'narrative', message: '仿真包必须提供教学叙事步骤。' })
    return
  }
  const ids = new Set<string>()
  for (const [index, step] of simPackage.narrative.entries()) {
    if (!step.id?.trim() || !step.title?.trim() || typeof step.trigger !== 'function' || ids.has(step.id)) {
      issues.push({ path: `narrative.${index}`, message: '叙事步骤必须包含唯一 id、title 和 trigger。' })
    }
    ids.add(step.id)
  }
}

/** validateCodeTrace 校验代码追踪源码和行映射。 */
function validateCodeTrace<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimPackageValidationIssue[]): void {
  if (!simPackage.codeTrace?.sourceCode || !simPackage.codeTrace.lineMapping.length) {
    issues.push({ path: 'codeTrace', message: '代码追踪必须包含源码和行映射。' })
  }
}

/** validateCheckpoints 校验可判题检查点及其唯一编号。 */
function validateCheckpoints<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimPackageValidationIssue[]): void {
  if (!simPackage.checkpoints.length) {
    issues.push({ path: 'checkpoints', message: '仿真包必须提供至少一个检查点。' })
    return
  }
  const ids = new Set<string>()
  for (const [index, checkpoint] of simPackage.checkpoints.entries()) {
    if (!checkpoint.id?.trim() || !checkpoint.label?.trim() || typeof checkpoint.evaluate !== 'function' || ids.has(checkpoint.id)) {
      issues.push({ path: `checkpoints.${index}`, message: '检查点必须包含唯一 id、label 和 evaluate。' })
    }
    ids.add(checkpoint.id)
  }
}

/** validateInitialRender 执行初始渲染并复用统一画面校验。 */
function validateInitialRender<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimPackageValidationIssue[]): void {
  try {
    const initialState = simPackage.initState({}, 1)
    issues.push(...validateTeachingFrame(simPackage.render(initialState), simPackage.meta.scaleLimit))
  } catch {
    issues.push({ path: 'render', message: '初始渲染失败，请检查初始状态和渲染声明。' })
  }
}

/** layoutPatternIds 汇总教学布局显式引用的视图编号。 */
function layoutPatternIds(view: TeachingFrame): Set<string> {
  return new Set([
    view.layout.primary,
    ...(view.layout.evidence ?? []),
    ...(view.layout.timeline ? [view.layout.timeline] : []),
    ...(view.layout.metrics ?? []),
    ...(view.layout.trace ? [view.layout.trace] : []),
    ...(view.layout.checkpoints ?? []),
  ].filter(Boolean))
}

/** validatePatternLimit 按视图语义校验容量和矩阵结构。 */
function validatePatternLimit(
  pattern: TeachingFrame['patterns'][number],
  limits: SimPackage['meta']['scaleLimit'],
  issues: SimPackageValidationIssue[]
): void {
  const check = (actual: number, limit: number, resource: string): void => {
    if (actual > limit) issues.push({ path: `render.patterns.${pattern.id}`, message: `仿真${resource}数量超过限制，请调整场景规模。` })
  }
  if (pattern.mode === 'graph') {
    check(pattern.data.nodes.length, limits.nodes, '网络节点')
    check(pattern.data.edges.length, limits.maxEvents, '网络连线')
  } else if (pattern.mode === 'chain') {
    check(pattern.data.blocks.length + pattern.data.forks.reduce((sum, fork) => sum + fork.length, 0), limits.nodes, '区块节点')
  } else if (pattern.mode === 'tree') {
    check(countTreeNodes(pattern.data.root), limits.nodes, '树节点')
  } else if (pattern.mode === 'matrix') {
    const { rows, columns, cells } = pattern.data
    if (cells.length !== rows.length || cells.some((row) => row.length !== columns.length)) {
      issues.push({ path: `render.patterns.${pattern.id}`, message: '仿真矩阵的行列数据不完整，请联系发布者处理。' })
    }
    check(rows.length, limits.nodes, '矩阵行')
    check(columns.length, limits.nodes, '矩阵列')
    check(rows.length * columns.length, limits.maxEvents, '矩阵单元')
  } else if (pattern.mode === 'pipeline') {
    check(pattern.data.steps.length, limits.nodes, '流程步骤')
  } else if (pattern.mode === 'lane') {
    check(pattern.data.actors.length, limits.nodes, '时序参与方')
    check(pattern.data.messages.length, limits.maxEvents, '时序消息')
  } else {
    check(pattern.data.series.length, limits.nodes, '图表序列')
    check(pattern.data.series.reduce((sum, series) => sum + series.points.length, 0), limits.maxEvents, '图表数据点')
  }
}

/** countTreeNodes 递归统计树视图节点数。 */
function countTreeNodes(node: TreeNode): number {
  return 1 + (node.children ?? []).reduce((total, child) => total + countTreeNodes(child), 0)
}
