// 本文件是仿真包的确定性执行引擎,不含任何宿主协议。
//
// 为什么独立成文件:同一个仿真包有两个执行宿主 —— 浏览器 Worker(平台内置包)与后端隔离容器
// (教师/第三方扩展包,见 docs/04-仿真可视化引擎/02-架构设计.md §8)。两个宿主必须算出
// 逐位相同的快照:同一份操作序列会被回放、分享与判分,引擎只要有两份实现就必然漂移,
// 而漂移的表现是"同一个 seed 在两个地方跑出两条过程",回放和判分随之失去意义。
//
// 故这里只做状态机推进与快照求值,宿主差异(消息通道 / stdio 协议、能力封锁方式)留在各自宿主文件。

import type {
  CheckpointDescriptor,
  FieldDef,
  JsonObject,
  JsonValue,
  NarrativeStepDescriptor,
  ReducerContext,
  RuntimeSnapshot,
  SimEvent,
  SimInitParams,
  SimPackage,
  SimPackageDescriptor,
  SimState,
} from '../types';
import { hashSeed, XorShiftRandom } from './deterministic';
import { assertValidSimPackage, assertValidTeachingFrame } from '../validation';

/**
 * SimEngine 持有一个仿真包的确定性运行状态,并按命令推进。
 * 它不知道自己跑在 Worker 还是容器里 —— 宿主只负责把命令送进来、把快照取出去。
 */
export class SimEngine {
  private readonly simPackage: SimPackage;
  private readonly descriptor: SimPackageDescriptor;
  private readonly initParams: SimInitParams;
  private readonly seed: number;
  private state: SimState;
  private tick = 0;
  private seq = 1;
  private events: SimEvent[] = [];

  /**
   * constructor 校验包协议后构造初始状态;协议不完整即拒绝装配,不进入半可用态。
   */
  constructor(simPackage: unknown, initParams: SimInitParams, seed: number) {
    assertValidSimPackage(simPackage);
    this.simPackage = simPackage;
    this.initParams = initParams;
    this.seed = seed;
    this.descriptor = describePackage(simPackage);
    this.state = simPackage.initState(initParams, seed);
  }

  /** describe 返回可序列化的包描述符,交互控件与代码追踪面板据此渲染。 */
  describe(): SimPackageDescriptor {
    return this.descriptor;
  }

  /** step 推进一个 tick。 */
  step(): void {
    const advance = this.simPackage.interactions.find((item) => item.emits === 'advance');
    if (advance && !this.isInteractionAvailable(advance)) return;
    this.applyEvent({ type: 'tick', source: 'tick', payload: {}, target: undefined });
  }

  /** inject 注入一次用户交互;事件与参数必须落在包声明的交互白名单内。 */
  inject(eventType: string, payload: JsonObject = {}, target?: string): void {
    const nextPayload: JsonObject = { ...(payload ?? {}) };
    if (target) {
      nextPayload.target = target;
    }
    this.applyEvent({ type: eventType, source: 'user', payload: nextPayload, target });
  }

  /** back 回退最近一次事件:从初始状态重放到上一条,状态可复现而非就地反算。 */
  back(): void {
    this.replay(this.events.slice(0, -1));
  }

  /**
   * restore 按给定事件日志把引擎恢复到对应位置。
   * 容器宿主每次 exec 都是新进程,状态由后端在命令里带回后经此恢复(见 containerHost.ts);
   * 浏览器宿主恢复现场时同样走它,不各写一套重放。
   */
  restore(events: SimEvent[]): void {
    this.replay(events);
  }

  /** reset 回到初始状态。 */
  reset(): void {
    this.tick = 0;
    this.seq = 1;
    this.events = [];
    this.state = this.simPackage.initState(this.initParams, this.seed);
  }

  /**
   * syncState 用受信任来源给出的模型状态替换当前状态,并保持 tick 对齐。
   * 仅供受控后端算法适配器回传状态时使用;它绕过 reducer,故不进事件日志。
   */
  syncState(tick: number, state: SimState): void {
    if (!Number.isSafeInteger(tick) || tick < 0 || !state || typeof state !== 'object') {
      throw new Error('云端仿真状态不完整,请稍后重试');
    }
    this.tick = tick;
    this.state = state;
  }

  /** lastEvent 返回最近一条事件,供宿主随快照回传。 */
  lastEvent(): SimEvent | undefined {
    return this.events[this.events.length - 1];
  }

  /**
   * snapshot 汇总状态、教学画面、叙事命中、交互可用性与检查点结果。
   * 每帧都过 TeachingFrame 协议校验:非法帧在这里失败,不流向渲染端。
   */
  snapshot(): RuntimeSnapshot {
    const view = this.simPackage.render(this.state);
    assertValidTeachingFrame(view, this.simPackage.meta.scaleLimit);
    const checkpointResults: RuntimeSnapshot['checkpointResults'] = {};
    for (const checkpoint of this.simPackage.checkpoints ?? []) {
      checkpointResults[checkpoint.id] = checkpoint.evaluate(this.state);
    }
    const interactionAvailability: Record<string, boolean> = {};
    for (const interaction of this.simPackage.interactions) {
      interactionAvailability[interaction.id] = this.isInteractionAvailable(interaction);
    }
    return {
      state: this.state,
      tick: this.tick,
      events: [...this.events],
      view,
      currentStep: this.currentNarrativeStep(),
      interactionAvailability,
      checkpointResults,
    };
  }

  /**
   * applyEvent 构造带 seq 与 tick 的事件,用确定性上下文推进 reducer。
   */
  private applyEvent(eventInput: Omit<SimEvent, 'seq' | 'atTick'>): void {
    this.enforceEventLimit(eventInput);
    this.enforceEventSchema(eventInput);
    const event: SimEvent = { ...eventInput, atTick: this.tick, seq: this.seq };
    this.state = this.simPackage.reducer(this.state, event, this.reducerContext(this.tick, this.seq));
    this.seq += 1;
    if (event.source === 'tick') {
      this.tick += 1;
    }
    this.events.push(event);
  }

  /**
   * replay 从初始状态重放事件日志,用于回退时保持状态可复现。
   */
  private replay(nextEvents: SimEvent[]): void {
    this.tick = 0;
    this.seq = 1;
    this.events = [];
    let replayState = this.simPackage.initState(this.initParams, this.seed);
    for (const event of nextEvents) {
      replayState = this.simPackage.reducer(
        replayState,
        event,
        this.reducerContext(event.atTick, event.seq),
      );
      this.tick = event.source === 'tick' ? event.atTick + 1 : event.atTick;
      this.seq = event.seq + 1;
      this.events.push(event);
    }
    this.state = replayState;
  }

  /**
   * reducerContext 构造确定性上下文:随机源按「包编号 + tick + seq」派生种子,
   * 因此同一事件在任何宿主、任何次运行中都取到同一串随机数。
   */
  private reducerContext(tick: number, seq: number): ReducerContext {
    return {
      seed: this.seed,
      tick,
      seq,
      random: new XorShiftRandom(hashSeed(this.seed, `${this.simPackage.meta.code}:${tick}:${seq}`)),
    };
  }

  /**
   * enforceEventSchema 按包内交互声明校验用户事件,避免前端可运行但服务端动作白名单拒绝。
   */
  private enforceEventSchema(eventInput: Omit<SimEvent, 'seq' | 'atTick'>): void {
    if (eventInput.source !== 'user') {
      return;
    }
    const interaction = this.simPackage.interactions.find((item) => item.emits === eventInput.type);
    if (!interaction) {
      throw new Error('当前仿真包不支持这个操作');
    }
    if (!this.isInteractionAvailable(interaction)) {
      throw new Error('当前阶段不允许这个操作');
    }
    const needsElement = interaction.target === 'element' || interaction.kind === 'select-element';
    if (needsElement && !eventInput.target) {
      throw new Error('请先选择要操作的仿真对象');
    }
    if (!needsElement && eventInput.target) {
      throw new Error('当前对象不能执行这个操作');
    }
    const payload = eventInput.payload ?? {};
    const allowed = new Map((interaction.params ?? []).map((field) => [field.name, field]));
    for (const key of Object.keys(payload)) {
      if (platformPayloadValueMatchesInteraction(key, payload[key], interaction)) {
        continue;
      }
      const field = allowed.get(key);
      if (!field || !payloadValueMatchesField(payload[key], field)) {
        throw new Error('操作参数不完整,请检查后重试');
      }
    }
    for (const field of interaction.params ?? []) {
      if (field.required && payload[field.name] === undefined) {
        throw new Error('请补全操作参数后再继续');
      }
    }
  }

  /**
   * isInteractionAvailable 统一计算声明条件与通用攻击-恢复顺序,让控件状态与引擎校验使用同一份契约。
   */
  private isInteractionAvailable(interaction: SimPackage['interactions'][number]): boolean {
    if (interaction.availableWhen && !interaction.availableWhen(this.state)) return false;
    // 每个包都必须把 attack -> recover 作为完整教学链;顺序之外的语义由包自身 availableWhen 定义。
    if (interaction.emits === 'advance' && !this.wouldChangeState(interaction, 'user')) return false;
    if (interaction.emits !== 'recover') return true;
    const lastAttack = this.events.reduce((index, event, currentIndex) => event.type === 'attack' ? currentIndex : index, -1);
    const lastRecover = this.events.reduce((index, event, currentIndex) => event.type === 'recover' ? currentIndex : index, -1);
    return lastAttack > lastRecover;
  }

  /**
   * wouldChangeState 预演一次推进动作,只比较业务 state,不把事件日志当成画面变化。
   * 终态 reducer 若返回同一份业务状态,控件必须禁用,否则用户只能不断追加无效事件直到撞上规模上限。
   */
  private wouldChangeState(interaction: SimPackage['interactions'][number], source: 'tick' | 'user'): boolean {
    const probeEvent: SimEvent = {
      type: interaction.emits,
      source,
      payload: {},
      target: undefined,
      atTick: this.tick,
      seq: this.seq,
    };
    const before = JSON.stringify(this.state);
    const after = this.simPackage.reducer(this.state, probeEvent, this.reducerContext(this.tick, this.seq));
    return JSON.stringify(after) !== before;
  }

  /**
   * enforceEventLimit 执行包声明的 tick 与事件规模上限。
   */
  private enforceEventLimit(eventInput: Omit<SimEvent, 'seq' | 'atTick'>): void {
    const limits = this.simPackage.meta.scaleLimit;
    if (eventInput.source === 'tick' && this.tick >= limits.maxTick) {
      throw new Error('仿真步骤数量超过限制,请调整场景规模');
    }
    if (this.events.length >= limits.maxEvents) {
      throw new Error('仿真事件数量超过限制,请调整场景规模');
    }
  }

  /**
   * currentNarrativeStep 按当前状态选择正在触发的叙事步骤。
   */
  private currentNarrativeStep(): NarrativeStepDescriptor | undefined {
    const matched = (this.simPackage.narrative ?? []).find((step) => step.trigger(this.state));
    return stripNarrativeStep(matched);
  }
}

/**
 * describePackage 把含函数的 SimPackage 收窄成可序列化描述符。
 */
function describePackage(pkg: SimPackage): SimPackageDescriptor {
  return {
    meta: pkg.meta,
    interactions: pkg.interactions.map(({ availableWhen: _availableWhen, ...interaction }) => interaction),
    narrative: (pkg.narrative ?? [])
      .map(stripNarrativeStep)
      .filter((step): step is NarrativeStepDescriptor => Boolean(step)),
    codeTrace: pkg.codeTrace,
    checkpoints: (pkg.checkpoints ?? []).map<CheckpointDescriptor>((checkpoint) => ({
      id: checkpoint.id,
      label: checkpoint.label,
    })),
  };
}

/**
 * stripNarrativeStep 移除叙事触发函数,只保留可序列化的教学描述。
 */
function stripNarrativeStep(
  step?: NonNullable<SimPackage['narrative']>[number],
): NarrativeStepDescriptor | undefined {
  if (!step) {
    return undefined;
  }
  const descriptorStep = { ...step } as NarrativeStepDescriptor & { trigger?: unknown };
  delete descriptorStep.trigger;
  return descriptorStep;
}

/**
 * platformPayloadValueMatchesInteraction 校验平台通用控件自动生成的固定字段,
 * 算法自定义字段仍必须走 params 声明。
 */
function platformPayloadValueMatchesInteraction(
  key: string,
  value: JsonValue | undefined,
  interaction: SimPackage['interactions'][number],
): boolean {
  if (key === 'target') {
    return (
      (interaction.target === 'element' || interaction.kind === 'select-element') &&
      typeof value === 'string' &&
      value.trim().length > 0 &&
      value.length <= 128
    );
  }
  if (interaction.kind === 'hold' && key === 'active') {
    return typeof value === 'boolean';
  }
  if (interaction.kind !== 'drag') {
    return false;
  }
  if (key === 'phase') {
    return value === 'start' || value === 'move' || value === 'end';
  }
  if (
    key === 'startX' ||
    key === 'startY' ||
    key === 'currentX' ||
    key === 'currentY' ||
    key === 'deltaX' ||
    key === 'deltaY'
  ) {
    return typeof value === 'number' && Number.isFinite(value);
  }
  return false;
}

/**
 * payloadValueMatchesField 校验用户载荷是否落在字段声明范围内。
 */
function payloadValueMatchesField(value: JsonValue | undefined, field: FieldDef): boolean {
  if (value === undefined) {
    return !field.required;
  }
  if (field.type === 'number' || field.type === 'range') {
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      return false;
    }
    if (field.min !== undefined && value < field.min) {
      return false;
    }
    if (field.max !== undefined && value > field.max) {
      return false;
    }
    return true;
  }
  if (field.type === 'boolean') {
    return typeof value === 'boolean';
  }
  if (field.type === 'string') {
    return typeof value === 'string' && value.trim().length > 0 && value.length <= 512;
  }
  if (field.type === 'select') {
    const valueText = scalarPayloadString(value);
    return (
      valueText !== undefined &&
      Boolean(field.options?.some((option) => scalarPayloadString(option.value) === valueText))
    );
  }
  return false;
}

/**
 * scalarPayloadString 复刻后端公开标量枚举比较规则,保证 select 参数前后端一致。
 */
function scalarPayloadString(value: JsonValue | undefined): string | undefined {
  if (typeof value === 'string') {
    return value.trim() || undefined;
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value);
  }
  return undefined;
}
