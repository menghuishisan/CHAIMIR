// 本文件只提供 SimPackage 协议装配与可视化语义数据辅助,不包含任何具体算法状态机逻辑。

import type {
  ChainBlock,
  ChartSeries,
  CheckpointDef,
  CodeTraceDef,
  GraphEdge,
  GraphNode,
  InteractionDef,
  LaneMessage,
  MatrixCell,
  NarrativeStep,
  PatternBinding,
  PipelineStep,
  ReducerContext,
  SimEvent,
  SimInitParams,
  SimPackage,
  SimState,
  TeachingFrame,
  TreeNode,
} from '../types';
import { narrativeQuestions, type PhaseNarrativeQuestion } from './narrativeQuestions';

export interface AlgorithmPackageSpec<TState extends SimState> {
  meta: SimPackage<TState>['meta'];
  init: (params: SimInitParams, seed: number) => TState;
  step: (state: TState, event: SimEvent, context: ReducerContext) => TState;
  attack: (state: TState, event: SimEvent, context: ReducerContext) => TState;
  recover: (state: TState, event: SimEvent, context: ReducerContext) => TState;
  select?: (state: TState, event: SimEvent) => TState;
  render: (state: TState) => TeachingFrame;
  interactions: InteractionDef[];
  narrative: NarrativeStep[];
  codeTrace: CodeTraceDef;
  checkpoints: CheckpointDef[];
}

export interface TeachingFrameInput {
  summary: string;
  phase: {
    id: string;
    title: string;
    intent?: TeachingFrame['phase']['intent'];
    what: string;
    why: string;
    watch: string;
  };
  patterns: PatternBinding[];
  focus: TeachingFrame['focus'];
  layout: TeachingFrame['layout'];
  annotations?: TeachingFrame['annotations'];
}

/**
 * createAlgorithmPackage 只把算法专属函数装配成 M4 SimPackage,不共享算法实现。
 */
export function createAlgorithmPackage<TState extends SimState>(spec: AlgorithmPackageSpec<TState>): SimPackage<TState> {
  return {
    meta: spec.meta,
    initState: spec.init,
    reducer: (state, event, context) => {
      if (event.type === 'advance' || event.type === 'tick') return spec.step(state, event, context);
      if (event.type === 'attack') return spec.attack(state, event, context);
      if (event.type === 'recover') return spec.recover(state, event, context);
      if (event.type === 'select' && spec.select) return spec.select(state, event);
      return state;
    },
    interactions: spec.interactions,
    render: spec.render,
    narrative: spec.narrative,
    codeTrace: spec.codeTrace,
    checkpoints: spec.checkpoints,
  };
}

/**
 * commonAlgorithmInteractions 返回通用控件声明,实际效果由每个算法自己的 reducer 决定。
 */
export function commonAlgorithmInteractions(targetFilter: string): InteractionDef[] {
  return [
    { id: 'select', kind: 'select-element', label: '选择对象', description: '选择画面中的对象,查看它在当前算法流程中的状态。', emits: 'select', target: 'element', elementFilter: targetFilter },
    { id: 'advance', kind: 'button', label: '推进阶段', description: '按当前算法规则推进一个阶段。', emits: 'advance', labelTag: 'normal' },
    { id: 'attack', kind: 'button', label: '注入异常', description: '注入该算法需要处理的异常输入或攻击路径。', emits: 'attack', labelTag: 'attack' },
    { id: 'recover', kind: 'button', label: '执行修复', description: '按该算法的恢复或防护规则处理异常。', emits: 'recover', labelTag: 'perturb' },
  ];
}

/**
 * phaseNarrative 从阶段说明生成教学叙事,并按检查点生成算法专属判断题。
 */
export function phaseNarrative(
  phases: ReadonlyArray<{ id: string; label: string; effect: string; reason: string }>,
  checkpointId: string,
  question: PhaseNarrativeQuestion = narrativeQuestions[checkpointId] ?? {
    prompt: '当前流程是否已经满足该算法的关键正确性条件?',
    options: ['满足', '不满足'],
    answer: '满足',
  }
): NarrativeStep[] {
  const { phaseId, ...questionPayload } = question;
  const questionPhaseId = phaseId ?? phases[phases.length - 1]?.id;
  return phases.map((phase) => ({
    id: phase.id,
    title: phase.label,
    trigger: (state) => state.phase === phase.label,
    highlight: [phase.id],
    explain: `${phase.effect} ${phase.reason}`,
    defaultDurationMs: 1200,
    question:
      phase.id === questionPhaseId
        ? {
            ...questionPayload,
            checkpointId,
          }
        : undefined,
  }));
}

/**
 * codeTrace 从伪代码和阶段行号生成代码追踪配置。
 */
export function codeTrace(sourceCode: string[], phases: Array<{ label: string; reason: string; line: number }>): CodeTraceDef {
  return {
    sourceCode: sourceCode.join('\n'),
    language: 'pseudocode',
    lineMapping: phases.map((phase) => ({ line: phase.line, triggerCondition: `phase == ${phase.label}`, annotation: phase.reason, highlightStyle: 'normal' })),
    variableWatch: [
      { name: '阶段', extract: 'state.phase', format: 'string' },
      { name: '结果', extract: 'state.metrics.result', format: 'string' },
      { name: '风险', extract: 'state.metrics.risk', format: 'number' },
    ],
  };
}

/**
 * teachingFrame 把算法状态映射为教学画面,由 frame 显式声明主舞台、辅助证据和当前关注对象。
 */
export function teachingFrame(input: TeachingFrameInput): TeachingFrame {
  return {
    summary: input.summary,
    phase: {
      id: input.phase.id,
      title: input.phase.title,
      intent: input.phase.intent ?? 'observe',
      explanation: {
        what: input.phase.what,
        why: input.phase.why,
        watch: input.phase.watch,
      },
    },
    focus: input.focus,
    layout: input.layout,
    patterns: input.patterns,
    annotations: input.annotations,
  };
}

/**
 * selectedOrFrameFocus 只处理交互选中优先级,默认关注对象必须由算法 view 显式传入。
 */
export function selectedOrFrameFocus(selectedElementId: string | undefined, fallbackIds: string[]): string[] {
  return selectedElementId ? [selectedElementId] : fallbackIds;
}

/**
 * graphPattern 创建图网络封闭模式绑定。
 */
export function graphPattern(id: string, title: string, nodes: GraphNode[], edges: GraphEdge[]): PatternBinding {
  return { id, mode: 'graph', title, data: { layout: 'ring', nodes, edges } };
}

/**
 * pipelinePattern 创建阶段流水线封闭模式绑定。
 */
export function pipelinePattern(id: string, title: string, steps: PipelineStep[], currentStepId: string): PatternBinding {
  return { id, mode: 'pipeline', title, data: { steps, currentStepId } };
}

/**
 * matrixPattern 创建矩阵封闭模式绑定。
 */
export function matrixPattern(id: string, title: string, rows: string[], columns: string[], cells: MatrixCell[][]): PatternBinding {
  return { id, mode: 'matrix', title, data: { rows, columns, cells } };
}

/**
 * lanePattern 创建参与方时序泳道封闭模式绑定。
 */
export function lanePattern(id: string, title: string, actors: string[], messages: LaneMessage[], currentTime: number): PatternBinding {
  return { id, mode: 'lane', title, data: { actors, messages, currentTime } };
}

/**
 * chartPattern 创建轻量趋势图封闭模式绑定。
 */
export function chartPattern(id: string, title: string, series: ChartSeries[], unit = '%'): PatternBinding {
  return { id, mode: 'chart', title, data: { series, unit } };
}

/**
 * chainPattern 创建主链和分叉封闭模式绑定。
 */
export function chainPattern(id: string, title: string, blocks: ChainBlock[], forks: ChainBlock[][] = []): PatternBinding {
  return { id, mode: 'chain', title, data: { blocks, forks, canonicalTip: blocks[blocks.length - 1]?.id } };
}

/**
 * treePattern 创建树结构和高亮路径封闭模式绑定。
 */
export function treePattern(id: string, title: string, root: TreeNode, highlightedPath: string[]): PatternBinding {
  return { id, mode: 'tree', title, data: { root, highlightedPath } };
}
