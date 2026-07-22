// 本文件提供仿真包作者使用的 manifest、审核摘要和校验工具,保持前端包描述与后端 M4 审核入口一致。

import type { CodeTraceDef, FieldDef, InteractionDef, NarrativeStep, PatternMode, SimPackage, SimState } from '../types';
import { validateSimPackage } from '../validation';

type RenderPatternRole = 'primary' | 'evidence' | 'timeline' | 'metrics' | 'trace' | 'checkpoints';

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
    protocol: 'teaching-frame';
    patterns: Array<{
      id: string;
      mode: PatternMode;
      roles: RenderPatternRole[];
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
    protocol: 'teaching-frame';
    patterns: Array<{
      id: string;
      mode: PatternMode;
      roles: RenderPatternRole[];
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

/**
 * defineSimPackage 是官方仿真包定义入口,会在开发期执行协议校验并返回原包对象。
 */
export function defineSimPackage<TState extends SimState>(simPackage: SimPackage<TState>): SimPackage<TState> {
  const result = validateSimPackage(simPackage);
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
      protocol: 'teaching-frame',
      patterns: view.patterns.map((pattern) => ({
        id: pattern.id,
        mode: pattern.mode,
        roles: patternRoles(view, pattern.id),
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
 * patternRoles 从 TeachingFrame 布局中提取 manifest 可审核的模式职责。
 */
function patternRoles(view: ReturnType<SimPackage['render']>, patternId: string): RenderPatternRole[] {
  const roles = new Set<RenderPatternRole>();
  if (view.layout.primary === patternId) roles.add('primary');
  if (view.layout.evidence?.includes(patternId)) roles.add('evidence');
  if (view.layout.timeline === patternId) roles.add('timeline');
  if (view.layout.metrics?.includes(patternId)) roles.add('metrics');
  if (view.layout.trace === patternId) roles.add('trace');
  if (view.layout.checkpoints?.includes(patternId)) roles.add('checkpoints');
  return [...roles];
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
