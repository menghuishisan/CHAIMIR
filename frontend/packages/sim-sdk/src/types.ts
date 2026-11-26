// 仿真包与执行输出的权威类型；schema 注解同步生成两端结构校验器。

export type SimCategory =
  | 'consensus'
  | 'cryptography'
  | 'network'
  | 'data-structure'
  | 'contract-security'
  | 'transaction-runtime'
  | 'cross-chain-system'

export type InteractionKind = 'button' | 'slider' | 'hold' | 'select-element' | 'drag' | 'form'
// 攻击与恢复分别表达破坏和防护，控件不得共用攻击色。
export type InteractionTag = 'normal' | 'recover' | 'perturb' | 'attack'
export type PatternMode = 'graph' | 'chain' | 'tree' | 'matrix' | 'pipeline' | 'lane' | 'chart'
export type FrameIntent =
  | 'observe'
  | 'compare'
  | 'verify'
  | 'debug'
  | 'attack'
  | 'recover'
  | 'replay'

export interface SimPackage<TState extends SimState = SimState> {
  meta: SimMeta
  initState(params: SimInitParams, seed: number): TState
  reducer(state: TState, event: SimEvent, context: ReducerContext): TState
  interactions: InteractionDef[]
  render(state: TState): TeachingFrame
  narrative: NarrativeStep[]
  codeTrace: CodeTraceDef
  checkpoints: CheckpointDef[]
}

// 执行位置由后端按代码来源派生，作者只声明包的能力和教学内容。
export interface SimMeta {
  /** @pattern \S */
  code: string
  /** @pattern \S */
  name: string
  category: SimCategory
  /** @pattern \S */
  version: string
  /** entry 是归档内入口模块相对路径,供隔离容器装配;内置包由 registry 按 code 装配故省略。 */
  entry?: string
  /** @pattern \S */
  summary: string
  /** @minItems 1 */
  learningObjectives: string[]
  scaleLimit: {
    /** @asType integer @minimum 1 @maximum 10000 */
    nodes: number
    /** @asType integer @minimum 1 @maximum 100000 */
    maxTick: number
    /** @asType integer @minimum 1 @maximum 100000 */
    maxEvents: number
  }
}

export interface SimInitParams {
  [key: string]: JsonValue
}

export interface SimState {
  /** @asType integer @minimum 0 */
  tick: number
  phase: string
  /** 阶段型内置仿真用此索引声明有限流程的终态;非阶段模型可省略。 */
  phaseIndex?: number
  selectedElementId?: string
  explanation: StepExplanation
  _trace?: TraceInfo
  metrics: Record<string, number | string | boolean>
  checkpointValues: Record<string, JsonValue>
}

export interface StepExplanation {
  title: string
  effect: string
  reason: string
  defaultDurationMs: number
}

export interface TraceInfo {
  triggeredLines: number[]
  variables: Record<string, JsonValue>
  executionPath?: string
}

export interface SimEvent {
  type: string
  source: 'tick' | 'user' | 'system'
  atTick: number
  seq: number
  payload: JsonObject
  target?: string
}

export interface ReducerContext {
  seed: number
  tick: number
  seq: number
  random: DeterministicRandom
}

export interface DeterministicRandom {
  next(): number
  int(min: number, max: number): number
  pick<T>(items: readonly T[]): T
}

export interface InteractionDescriptor {
  /** @pattern \S */
  id: string
  kind: InteractionKind
  /** @pattern \S */
  label: string
  description: string
  /** @pattern \S */
  emits: string
  params?: FieldDef[]
  target?: 'global' | 'element'
  elementFilter?: string
  labelTag?: InteractionTag
  cooldownMs?: number
}

export interface InteractionDef extends InteractionDescriptor {
  availableWhen?: (state: SimState) => boolean
}

export interface FieldDef {
  name: string
  label: string
  type: 'number' | 'string' | 'boolean' | 'select' | 'range'
  default: JsonValue
  min?: number
  max?: number
  step?: number
  options?: Array<{ label: string; value: JsonValue }>
  required?: boolean
}

export interface TeachingFrame {
  /** @minLength 1 */
  summary: string
  phase: FramePhase
  focus: FrameFocus
  layout: FrameLayout
  /** @minItems 1 @maxItems 3 */
  patterns: PatternBinding[]
  annotations?: FrameAnnotation[]
}

export interface FramePhase {
  /** @minLength 1 */
  id: string
  /** @minLength 1 */
  title: string
  intent: FrameIntent
  // 观察目标由 focus 表达，这里只解释动作与意义。
  explanation: {
    what: string
    why: string
  }
}

export interface FrameFocus {
  /** @minItems 1 */
  primary: string[]
  secondary?: string[]
  muted?: string[]
}

export interface FrameLayout {
  /** @minLength 1 */
  primary: string
  evidence?: string[]
  timeline?: string
  metrics?: string[]
  trace?: string
  checkpoints?: string[]
}

export interface FrameAnnotation {
  id: string
  target: string
  tone: 'info' | 'success' | 'warning' | 'danger'
  text: string
}

export type PatternBinding =
  | GraphPattern
  | ChainPattern
  | TreePattern
  | MatrixPattern
  | PipelinePattern
  | LanePattern
  | ChartPattern

export interface PatternBase<TMode extends PatternMode, TData> {
  /** @minLength 1 */
  id: string
  mode: TMode
  title: string
  data: TData
}

export type GraphPattern = PatternBase<
  'graph',
  {
    layout: 'ring' | 'grid' | 'layered'
    nodes: GraphNode[]
    edges: GraphEdge[]
  }
>

export interface GraphNode {
  id: string
  label: string
  role: string
  status: 'idle' | 'active' | 'success' | 'warning' | 'danger'
  value?: string
  meta?: VisualElementMeta
}

export interface GraphEdge {
  id: string
  from: string
  to: string
  label: string
  status: 'pending' | 'active' | 'success' | 'failed'
  process?: ProcessSpan
  detail?: string
  meta?: VisualElementMeta
}

export type ChainPattern = PatternBase<
  'chain',
  {
    blocks: ChainBlock[]
    forks: ChainBlock[][]
    canonicalTip?: string
  }
>

export interface ChainBlock {
  id: string
  height: number
  hash: string
  parentHash: string
  label: string
  status: 'genesis' | 'pending' | 'canonical' | 'orphaned' | 'attacker'
  meta?: VisualElementMeta
}

export type TreePattern = PatternBase<'tree', { root: TreeNode; highlightedPath: string[] }>

export interface TreeNode {
  id: string
  label: string
  hash: string
  children?: TreeNode[]
  meta?: VisualElementMeta
}

export type MatrixPattern = PatternBase<
  'matrix',
  {
    rows: string[]
    columns: string[]
    cells: MatrixCell[][]
  }
>

export interface MatrixCell {
  label: string
  status: 'empty' | 'pending' | 'yes' | 'no' | 'fault'
  meta?: VisualElementMeta
}

export type PipelinePattern = PatternBase<
  'pipeline',
  { steps: PipelineStep[]; currentStepId: string }
>

export interface PipelineStep {
  id: string
  label: string
  status: 'pending' | 'running' | 'complete' | 'failed'
  detail: string
  process?: ProcessSpan
  meta?: VisualElementMeta
}

export type LanePattern = PatternBase<
  'lane',
  { actors: string[]; messages: LaneMessage[]; currentTime: number }
>

export interface LaneMessage {
  id: string
  from: string
  to: string
  at: number
  label: string
  status: 'sent' | 'delivered' | 'dropped'
  endAt?: number
  process?: ProcessSpan
  detail?: string
  meta?: VisualElementMeta
}

export interface ProcessSpan {
  startedAt: number
  endedAt: number
  progress: number
  // 可选过程名只补充元素名、状态和 detail 未表达的信息。
  label?: string
}

export interface VisualElementMeta {
  id: string
  label: string
  role?: string
  lifecycle: {
    state: 'entering' | 'active' | 'settled' | 'leaving' | 'archived'
    fromTick: number
    toTick?: number
  }
  emphasis: 'focus' | 'context' | 'history' | 'ghost'
  explanation?: string
}

export type ChartPattern = PatternBase<'chart', { series: ChartSeries[]; unit: string }>

export interface ChartSeries {
  label: string
  points: Array<{ x: number; y: number }>
}

export interface NarrativeStepDescriptor {
  /** @pattern \S */
  id: string
  /** @pattern \S */
  title: string
  highlight: string[]
  explain: string
  question?: QuestionCheckpoint
  defaultDurationMs: number
}

export interface NarrativeStep extends NarrativeStepDescriptor {
  trigger: (state: SimState, event?: SimEvent) => boolean
}

export interface QuestionCheckpoint {
  prompt: string
  options: string[]
  answer: string
  checkpointId: string
}

export interface CheckpointDef {
  id: string
  label: string
  evaluate: (state: SimState) => CheckpointResult
}

export interface CheckpointResult {
  achieved: boolean
  answer: JsonValue
  explanation: string
}

export interface CheckpointDescriptor {
  /** @pattern \S */
  id: string
  /** @pattern \S */
  label: string
}

export interface SimPackageDescriptor {
  meta: SimMeta
  /** @minItems 1 @maxItems 64 */
  interactions: InteractionDescriptor[]
  /** @minItems 1 @maxItems 64 */
  narrative: NarrativeStepDescriptor[]
  codeTrace?: CodeTraceDef
  /** @minItems 1 @maxItems 64 */
  checkpoints: CheckpointDescriptor[]
}

export interface RuntimeSnapshot {
  state: SimState
  /** @asType integer @minimum 0 */
  tick: number
  events: SimEvent[]
  view: TeachingFrame
  currentStep?: NarrativeStepDescriptor
  interactionAvailability: Record<string, boolean>
  checkpointResults: Record<string, CheckpointResult>
}

/** 执行宿主共享的画面数据；事件历史由各宿主独立持有。 */
export type RuntimeFrame = Omit<RuntimeSnapshot, 'events'>

export interface StageConfig {
  stage: number
  title: string
  description?: string
  components: StageComponents
  unlockCondition?: StageUnlockCondition
  paramBindings?: ParamBinding[]
}

export interface StageComponents {
  envs?: string[]
  sims?: string[]
}

export interface StageUnlockCondition {
  type: 'checkpoint' | 'manual'
  checkpointId?: string
  minScore?: number
}

export interface ParamBinding {
  targetComponent: string
  targetParam: string
  sourceType: 'checkpoint' | 'constant'
  sourceRef?: string
  sourcePath?: string
  constantValue?: JsonValue
}

export interface StageInjectedParam {
  _source: string
  _value: JsonValue
}

export interface CodeTraceDef {
  /** @minLength 1 */
  sourceCode: string
  language: 'solidity' | 'rust' | 'go' | 'javascript' | 'pseudocode'
  /** @minItems 1 */
  lineMapping: LineMapping[]
  variableWatch?: VariableWatchDef[]
}

export interface LineMapping {
  line: number
  triggerCondition: string
  annotation?: string
  highlightStyle?: 'normal' | 'success' | 'error'
}

export interface VariableWatchDef {
  name: string
  extract: string
  format?: 'hex' | 'number' | 'string' | 'bool'
}

export type JsonPrimitive = string | number | boolean | null
export type JsonValue = JsonPrimitive | JsonObject | JsonValue[]
export interface JsonObject {
  [key: string]: JsonValue
}
