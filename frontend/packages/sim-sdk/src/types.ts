// 本文件定义仿真包、确定性状态机、交互协议、可视化模式与教学叙事契约。

export type SimComputeMode = 'frontend' | 'backend';
export type SimCategory =
  | 'consensus'
  | 'cryptography'
  | 'network'
  | 'data-structure'
  | 'contract-security'
  | 'transaction-runtime'
  | 'cross-chain-system';

export type InteractionKind = 'button' | 'slider' | 'hold' | 'select-element' | 'drag' | 'form';
export type InteractionTag = 'normal' | 'perturb' | 'attack';
export type PatternMode = 'graph' | 'chain' | 'tree' | 'matrix' | 'pipeline' | 'lane' | 'chart';
export type FrameIntent = 'observe' | 'compare' | 'verify' | 'debug' | 'attack' | 'recover' | 'replay';

export interface SimPackage<TState extends SimState = SimState> {
  meta: SimMeta;
  initState(params: SimInitParams, seed: number): TState;
  reducer(state: TState, event: SimEvent, context: ReducerContext): TState;
  interactions: InteractionDef[];
  render(state: TState): TeachingFrame;
  narrative: NarrativeStep[];
  codeTrace: CodeTraceDef;
  checkpoints: CheckpointDef[];
}

export interface SimMeta {
  code: string;
  name: string;
  category: SimCategory;
  version: string;
  compute: SimComputeMode;
  summary: string;
  learningObjectives: string[];
  scaleLimit: {
    nodes: number;
    maxTick: number;
    maxEvents: number;
  };
}

export interface SimInitParams {
  [key: string]: JsonValue;
}

export interface SimState {
  tick: number;
  phase: string;
  selectedElementId?: string;
  explanation: StepExplanation;
  _trace?: TraceInfo;
  metrics: Record<string, number | string | boolean>;
  checkpointValues: Record<string, JsonValue>;
}

export interface StepExplanation {
  title: string;
  effect: string;
  reason: string;
  defaultDurationMs: number;
}

export interface TraceInfo {
  triggeredLines: number[];
  variables: Record<string, JsonValue>;
  executionPath?: string;
}

export interface SimEvent {
  type: string;
  source: 'tick' | 'user' | 'system';
  atTick: number;
  seq: number;
  payload: JsonObject;
  target?: string;
}

export interface ReducerContext {
  seed: number;
  tick: number;
  seq: number;
  random: DeterministicRandom;
}

export interface DeterministicRandom {
  next(): number;
  int(min: number, max: number): number;
  pick<T>(items: readonly T[]): T;
}

export interface InteractionDef {
  id: string;
  kind: InteractionKind;
  label: string;
  description: string;
  emits: string;
  params?: FieldDef[];
  target?: 'global' | 'element';
  elementFilter?: string;
  availableWhen?: (state: SimState) => boolean;
  labelTag?: InteractionTag;
  cooldownMs?: number;
}

export type InteractionDescriptor = Omit<InteractionDef, 'availableWhen'>;

export interface FieldDef {
  name: string;
  label: string;
  type: 'number' | 'string' | 'boolean' | 'select' | 'range';
  default: JsonValue;
  min?: number;
  max?: number;
  step?: number;
  options?: Array<{ label: string; value: JsonValue }>;
  required?: boolean;
}

export interface TeachingFrame {
  summary: string;
  phase: FramePhase;
  focus: FrameFocus;
  layout: FrameLayout;
  patterns: PatternBinding[];
  annotations?: FrameAnnotation[];
}

export interface FramePhase {
  id: string;
  title: string;
  intent: FrameIntent;
  explanation: {
    what: string;
    why: string;
    watch: string;
  };
}

export interface FrameFocus {
  primary: string[];
  secondary?: string[];
  muted?: string[];
}

export interface FrameLayout {
  primary: string;
  evidence?: string[];
  timeline?: string;
  metrics?: string[];
  trace?: string;
  checkpoints?: string[];
}

export interface FrameAnnotation {
  id: string;
  target: string;
  tone: 'info' | 'success' | 'warning' | 'danger';
  text: string;
}

export type PatternBinding =
  | GraphPattern
  | ChainPattern
  | TreePattern
  | MatrixPattern
  | PipelinePattern
  | LanePattern
  | ChartPattern;

export interface PatternBase<TMode extends PatternMode, TData> {
  id: string;
  mode: TMode;
  title: string;
  data: TData;
}

export interface GraphPattern
  extends PatternBase<
    'graph',
    {
      layout: 'ring' | 'grid' | 'layered';
      nodes: GraphNode[];
      edges: GraphEdge[];
    }
  > {}

export interface GraphNode {
  id: string;
  label: string;
  role: string;
  status: 'idle' | 'active' | 'success' | 'warning' | 'danger';
  value?: string;
  meta?: VisualElementMeta;
}

export interface GraphEdge {
  id: string;
  from: string;
  to: string;
  label: string;
  status: 'pending' | 'active' | 'success' | 'failed';
  process?: ProcessSpan;
  detail?: string;
  meta?: VisualElementMeta;
}

export interface ChainPattern
  extends PatternBase<
    'chain',
    {
      blocks: ChainBlock[];
      forks: ChainBlock[][];
      canonicalTip?: string;
    }
  > {}

export interface ChainBlock {
  id: string;
  height: number;
  hash: string;
  parentHash: string;
  label: string;
  status: 'genesis' | 'pending' | 'canonical' | 'orphaned' | 'attacker';
  meta?: VisualElementMeta;
}

export interface TreePattern
  extends PatternBase<'tree', { root: TreeNode; highlightedPath: string[] }> {}

export interface TreeNode {
  id: string;
  label: string;
  hash: string;
  children?: TreeNode[];
  meta?: VisualElementMeta;
}

export interface MatrixPattern
  extends PatternBase<
    'matrix',
    {
      rows: string[];
      columns: string[];
      cells: MatrixCell[][];
    }
  > {}

export interface MatrixCell {
  label: string;
  status: 'empty' | 'pending' | 'yes' | 'no' | 'fault';
  meta?: VisualElementMeta;
}

export interface PipelinePattern
  extends PatternBase<'pipeline', { steps: PipelineStep[]; currentStepId: string }> {}

export interface PipelineStep {
  id: string;
  label: string;
  status: 'pending' | 'running' | 'complete' | 'failed';
  detail: string;
  process?: ProcessSpan;
  meta?: VisualElementMeta;
}

export interface LanePattern
  extends PatternBase<'lane', { actors: string[]; messages: LaneMessage[]; currentTime: number }> {}

export interface LaneMessage {
  id: string;
  from: string;
  to: string;
  at: number;
  label: string;
  status: 'sent' | 'delivered' | 'dropped';
  endAt?: number;
  process?: ProcessSpan;
  detail?: string;
  meta?: VisualElementMeta;
}

export interface ProcessSpan {
  startedAt: number;
  endedAt: number;
  progress: number;
  label: string;
}

export interface VisualElementMeta {
  id: string;
  label: string;
  role?: string;
  lifecycle: {
    state: 'entering' | 'active' | 'settled' | 'leaving' | 'archived';
    fromTick: number;
    toTick?: number;
  };
  emphasis: 'focus' | 'context' | 'history' | 'ghost';
  explanation?: string;
}

export interface ChartPattern
  extends PatternBase<'chart', { series: ChartSeries[]; unit: string }> {}

export interface ChartSeries {
  label: string;
  points: Array<{ x: number; y: number }>;
}

export interface NarrativeStep {
  id: string;
  title: string;
  trigger: (state: SimState, event?: SimEvent) => boolean;
  highlight: string[];
  explain: string;
  question?: QuestionCheckpoint;
  defaultDurationMs: number;
}

export type NarrativeStepDescriptor = Omit<NarrativeStep, 'trigger'>;

export interface QuestionCheckpoint {
  prompt: string;
  options: string[];
  answer: string;
  checkpointId: string;
}

export interface CheckpointDef {
  id: string;
  label: string;
  evaluate: (state: SimState) => CheckpointResult;
}

export interface CheckpointResult {
  achieved: boolean;
  answer: JsonValue;
  explanation: string;
}

export interface CheckpointDescriptor {
  id: string;
  label: string;
}

export interface SimPackageDescriptor {
  meta: SimMeta;
  interactions: InteractionDescriptor[];
  narrative: NarrativeStepDescriptor[];
  codeTrace?: CodeTraceDef;
  checkpoints: CheckpointDescriptor[];
}

export interface RuntimeSnapshot {
  state: SimState;
  tick: number;
  events: SimEvent[];
  view: TeachingFrame;
  currentStep?: NarrativeStepDescriptor;
  interactionAvailability: Record<string, boolean>;
  checkpointResults: Record<string, CheckpointResult>;
}

export interface StageConfig {
  stage: number;
  title: string;
  description?: string;
  components: StageComponents;
  unlockCondition?: StageUnlockCondition;
  paramBindings?: ParamBinding[];
}

export interface StageComponents {
  envs?: string[];
  sims?: string[];
}

export interface StageUnlockCondition {
  type: 'checkpoint' | 'manual';
  checkpointId?: string;
  minScore?: number;
}

export interface ParamBinding {
  targetComponent: string;
  targetParam: string;
  sourceType: 'checkpoint' | 'constant';
  sourceRef?: string;
  sourcePath?: string;
  constantValue?: JsonValue;
}

export interface StageInjectedParam {
  _source: string;
  _value: JsonValue;
}

export interface CodeTraceDef {
  sourceCode: string;
  language: 'solidity' | 'rust' | 'go' | 'javascript' | 'pseudocode';
  lineMapping: LineMapping[];
  variableWatch?: VariableWatchDef[];
}

export interface LineMapping {
  line: number;
  triggerCondition: string;
  annotation?: string;
  highlightStyle?: 'normal' | 'success' | 'error';
}

export interface VariableWatchDef {
  name: string;
  extract: string;
  format?: 'hex' | 'number' | 'string' | 'bool';
}

export interface PlaybackSpeed {
  label: string;
  multiplier: number;
}

export type JsonPrimitive = string | number | boolean | null;
export type JsonValue = JsonPrimitive | JsonObject | JsonValue[];
export interface JsonObject {
  [key: string]: JsonValue;
}
