// 本文件提供仿真包作者使用的 manifest、审核摘要和校验工具,保持前端包描述与后端 M4 审核入口一致。

import type { CodeTraceDef, FieldDef, InteractionDef, NarrativeStep, PatternMode, SimPackage, SimState } from '../types';

export interface SimPackageManifest {
  meta: {
    code: string;
    name: string;
    category: string;
    version: string;
    compute: string;
    scale_limit: {
      nodes: number;
      max_tick: number;
      max_events: number;
    };
  };
  interactions: SimManifestInteraction[];
  render: {
    patterns: Array<{
      mode: PatternMode;
      region: string;
    }>;
  };
  narrative: SimManifestNarrativeStep[];
  codeTrace: CodeTraceDef;
  checkpoints: Array<{
    id: string;
    label: string;
  }>;
}

export interface SimManifestInteraction {
  id: string;
  kind: string;
  label: string;
  emits: string;
  params?: SimManifestField[];
  target?: string;
  element_filter?: string;
  label_tag?: string;
  cooldown_ms?: number;
}

export type SimManifestField = Omit<FieldDef, 'label'>;

export interface SimManifestNarrativeStep {
  id: string;
  title: string;
  highlight: string[];
  explain: string;
  question?: NarrativeStep['question'];
  defaultDurationMs: number;
}

export interface SimManifestSummary {
  meta: SimPackage['meta'];
  interactions: Array<{
    id: string;
    kind: string;
    emits: string;
    labelTag?: string;
    paramNames: string[];
  }>;
  render: {
    patterns: Array<{
      mode: PatternMode;
      region: string;
    }>;
  };
  narrative: Array<{
    id: string;
    title: string;
    hasQuestion: boolean;
  }>;
  codeTrace?: {
    language: string;
    lineCount: number;
    mappingCount: number;
    variableCount: number;
  };
}

export interface SimManifestValidationIssue {
  path: string;
  message: string;
}

export interface SimManifestValidationResult {
  ok: boolean;
  issues: SimManifestValidationIssue[];
}

const payloadKeyPattern = /^[A-Za-z][A-Za-z0-9_.:-]{0,63}$/;
const reservedPayloadParams = new Set(['target', 'active', 'phase', 'startX', 'startY', 'currentX', 'currentY', 'deltaX', 'deltaY']);

/**
 * defineSimPackage 是官方仿真包定义入口,会在开发期执行协议校验并返回原包对象。
 */
export function defineSimPackage<TState extends SimState>(simPackage: SimPackage<TState>): SimPackage<TState> {
  const result = validateSimPackageManifest(simPackage);
  if (!result.ok) {
    throw new Error(`仿真包协议不完整:${result.issues.map((issue) => `${issue.path}:${issue.message}`).join(';')}`);
  }
  return simPackage;
}

/**
 * createSimPackageManifest 生成上传审核使用的 sim-package.json 结构,不包含 reducer 等可执行函数。
 */
export function createSimPackageManifest<TState extends SimState>(simPackage: SimPackage<TState>): SimPackageManifest {
  const summary = createManifestSummary(simPackage);
  return {
    meta: {
      code: summary.meta.code,
      name: summary.meta.name,
      category: summary.meta.category,
      version: summary.meta.version,
      compute: summary.meta.compute,
      scale_limit: {
        nodes: summary.meta.scaleLimit.nodes,
        max_tick: summary.meta.scaleLimit.maxTick,
        max_events: summary.meta.scaleLimit.maxEvents,
      },
    },
    interactions: simPackage.interactions.map(toManifestInteraction),
    render: summary.render,
    narrative: (simPackage.narrative ?? []).map(toManifestNarrativeStep),
    codeTrace: simPackage.codeTrace,
    checkpoints: simPackage.checkpoints.map((checkpoint) => ({ id: checkpoint.id, label: checkpoint.label })),
  };
}

/**
 * 生成接近 sim-package.json 的审核摘要,帮助开发者在上传前检查协议字段是否完整。
 */
export function createManifestSummary<TState extends SimState>(simPackage: SimPackage<TState>): SimManifestSummary {
  const initialState = simPackage.initState({}, 1);
  const view = simPackage.render(initialState);
  const sourceCode = simPackage.codeTrace?.sourceCode ?? '';
  return {
    meta: simPackage.meta,
    interactions: simPackage.interactions.map((interaction) => ({
      id: interaction.id,
      kind: interaction.kind,
      emits: interaction.emits,
      labelTag: interaction.labelTag,
      paramNames: (interaction.params ?? []).map((field) => field.name),
    })),
    render: {
      patterns: view.patterns.map((pattern) => ({
        mode: pattern.mode,
        region: pattern.region,
      })),
    },
    narrative: (simPackage.narrative ?? []).map((step) => ({
      id: step.id,
      title: step.title,
      hasQuestion: Boolean(step.question),
    })),
    codeTrace: simPackage.codeTrace
      ? {
          language: simPackage.codeTrace.language,
          lineCount: sourceCode.split('\n').length,
          mappingCount: simPackage.codeTrace.lineMapping.length,
          variableCount: simPackage.codeTrace.variableWatch?.length ?? 0,
        }
      : undefined,
  };
}

/**
 * validateSimPackageManifest 在上传前校验仿真包是否满足 M4 前端协议的关键硬约束。
 */
export function validateSimPackageManifest<TState extends SimState>(simPackage: SimPackage<TState>): SimManifestValidationResult {
  const issues: SimManifestValidationIssue[] = [];
  validateMeta(simPackage, issues);
  validateInteractions(simPackage, issues);
  validateNarrative(simPackage, issues);
  validateCodeTrace(simPackage, issues);
  validateCheckpoints(simPackage, issues);
  validateInitialRender(simPackage, issues);
  return { ok: issues.length === 0, issues };
}

/**
 * validateMeta 校验包元数据和规模限制是否完整。
 */
function validateMeta<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimManifestValidationIssue[]): void {
  const meta = simPackage.meta;
  if (!meta?.code || !meta.name || !meta.category || !meta.version) {
    issues.push({ path: 'meta', message: '仿真包元数据不完整。' });
  }
  if (meta?.compute !== 'frontend' && meta?.compute !== 'backend') {
    issues.push({ path: 'meta.compute', message: '计算模式必须声明为 frontend 或 backend。' });
  }
  if (!meta?.scaleLimit || meta.scaleLimit.nodes <= 0 || meta.scaleLimit.maxTick <= 0 || meta.scaleLimit.maxEvents <= 0) {
    issues.push({ path: 'meta.scaleLimit', message: '必须声明正数规模上限。' });
  }
}

/**
 * validateInteractions 校验交互声明是否可由通用工作台渲染和注入。
 */
function validateInteractions<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimManifestValidationIssue[]): void {
  if (!Array.isArray(simPackage.interactions)) {
    issues.push({ path: 'interactions', message: '交互声明必须是数组。' });
    return;
  }
  if (simPackage.interactions.length === 0) {
    issues.push({ path: 'interactions', message: '至少声明一个可操作交互。' });
  }
  for (const [index, interaction] of simPackage.interactions.entries()) {
    if (!interaction.id || !interaction.kind || !interaction.emits || !interaction.label) {
      issues.push({ path: `interactions.${index}`, message: '交互必须包含 id、kind、emits 和 label。' });
    }
    if (interaction.kind === 'select-element' && interaction.target !== 'element') {
      issues.push({ path: `interactions.${index}.target`, message: '选择元素交互必须声明 target 为 element。' });
    }
    if ((interaction.target === 'element' || interaction.kind === 'select-element') && !interaction.elementFilter) {
      issues.push({ path: `interactions.${index}.elementFilter`, message: '元素交互必须声明 elementFilter。' });
    }
    for (const [fieldIndex, field] of (interaction.params ?? []).entries()) {
      if (!field.name || !field.label || !field.type) {
        issues.push({ path: `interactions.${index}.params.${fieldIndex}`, message: '交互参数必须包含 name、label 和 type。' });
      }
      if (!payloadKeyPattern.test(field.name) || reservedPayloadParams.has(field.name)) {
        issues.push({ path: `interactions.${index}.params.${fieldIndex}.name`, message: '交互参数名必须符合后端操作日志规则,且不能使用平台保留字段。' });
      }
    }
  }
}

/**
 * validateNarrative 校验教学叙事步骤是否可以被工作台展示。
 */
function validateNarrative<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimManifestValidationIssue[]): void {
  if (!simPackage.narrative?.length) {
    issues.push({ path: 'narrative', message: '仿真包必须提供教学叙事步骤。' });
    return;
  }
  for (const [index, step] of (simPackage.narrative ?? []).entries()) {
    if (!step.id || !step.title || typeof step.trigger !== 'function') {
      issues.push({ path: `narrative.${index}`, message: '叙事步骤必须包含 id、title 和 trigger。' });
    }
  }
}

/**
 * validateCodeTrace 校验代码追踪是否有源码、行映射和可选变量观察声明。
 */
function validateCodeTrace<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimManifestValidationIssue[]): void {
  if (!simPackage.codeTrace) {
    issues.push({ path: 'codeTrace', message: '仿真包必须提供代码追踪配置。' });
    return;
  }
  if (!simPackage.codeTrace.sourceCode || simPackage.codeTrace.lineMapping.length === 0) {
    issues.push({ path: 'codeTrace.lineMapping', message: '代码追踪必须包含源码和行映射。' });
  }
}

/**
 * validateCheckpoints 校验仿真包是否提供可判题的检查点。
 */
function validateCheckpoints<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimManifestValidationIssue[]): void {
  if (!simPackage.checkpoints?.length) {
    issues.push({ path: 'checkpoints', message: '仿真包必须提供至少一个检查点。' });
    return;
  }
  for (const [index, checkpoint] of simPackage.checkpoints.entries()) {
    if (!checkpoint.id || !checkpoint.label || typeof checkpoint.evaluate !== 'function') {
      issues.push({ path: `checkpoints.${index}`, message: '检查点必须包含 id、label 和 evaluate。' });
    }
  }
}

/**
 * validateInitialRender 执行初始渲染并校验封闭模式数量。
 */
function validateInitialRender<TState extends SimState>(simPackage: SimPackage<TState>, issues: SimManifestValidationIssue[]): void {
  try {
    const initialState = simPackage.initState({}, 1);
    const view = simPackage.render(initialState);
    if (!view.summary || view.patterns.length < 1 || view.patterns.length > 3) {
      issues.push({ path: 'render.patterns', message: '渲染结果必须包含摘要和 1 到 3 个封闭可视化模式。' });
    }
  } catch (error) {
    issues.push({ path: 'render', message: error instanceof Error ? `初始渲染失败:${error.message}` : '初始渲染失败。' });
  }
}

/**
 * toManifestInteraction 把运行时交互声明转换为上传 manifest 的可序列化字段。
 */
function toManifestInteraction(interaction: InteractionDef): SimManifestInteraction {
  return {
    id: interaction.id,
    kind: interaction.kind,
    label: interaction.label,
    emits: interaction.emits,
    params: interaction.params?.map(toManifestField),
    target: interaction.target,
    element_filter: interaction.elementFilter,
    label_tag: interaction.labelTag,
    cooldown_ms: interaction.cooldownMs,
  };
}

/**
 * toManifestField 移除前端控件文案,只保留后端可审核的参数约束。
 */
function toManifestField(field: FieldDef): SimManifestField {
  return {
    name: field.name,
    type: field.type,
    default: field.default,
    min: field.min,
    max: field.max,
    step: field.step,
    options: field.options,
    required: field.required,
  };
}

/**
 * toManifestNarrativeStep 移除 trigger 函数,保留可审核的叙事内容。
 */
function toManifestNarrativeStep(step: NarrativeStep): SimManifestNarrativeStep {
  return {
    id: step.id,
    title: step.title,
    highlight: step.highlight,
    explain: step.explain,
    question: step.question,
    defaultDurationMs: step.defaultDurationMs,
  };
}
