// Worker 与隔离容器共用确定性引擎；同一包、种子与事件序列必须得到相同快照。

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
import { assertValidSimPackage, assertValidTeachingFrame, isRuntimeFrame } from '../validation';

/** 持有运行状态并校验每次交互与输出，宿主负责消息传输及环境隔离。 */
export class SimEngine {
  private readonly simPackage: SimPackage;
  private readonly descriptor: SimPackageDescriptor;
  private readonly initParams: SimInitParams;
  private readonly seed: number;
  private state: SimState;
  private tick = 0;
  private seq = 1;
  private events: SimEvent[] = [];

  constructor(simPackage: unknown, initParams: SimInitParams, seed: number) {
    assertValidSimPackage(simPackage);
    this.simPackage = simPackage;
    this.initParams = initParams;
    this.seed = seed;
    this.descriptor = describePackage(simPackage);
    this.state = simPackage.initState(initParams, seed);
  }

  describe(): SimPackageDescriptor {
    return this.descriptor;
  }

  step(): void {
    const advance = this.simPackage.interactions.find((item) => item.emits === 'advance');
    if (advance && !this.isInteractionAvailable(advance)) return;
    this.applyEvent({ type: 'tick', source: 'tick', payload: {}, target: undefined });
  }

  /** 事件、目标与参数必须符合包声明，拒绝时不追加记录。 */
  inject(eventType: string, payload: JsonObject = {}, target?: string): void {
    const nextPayload: JsonObject = { ...(payload ?? {}) };
    if (target) {
      nextPayload.target = target;
    }
    this.applyEvent({ type: eventType, source: 'user', payload: nextPayload, target });
  }

  /** 从初始状态重放到上一条事件，避免要求算法实现逆运算。 */
  back(): void {
    this.replay(this.events.slice(0, -1));
  }

  /** 按宿主已验证的完整事件序列恢复；浏览器恢复与容器重算共用此入口。 */
  restore(events: SimEvent[]): void {
    this.replay(events);
  }

  reset(): void {
    this.tick = 0;
    this.seq = 1;
    this.events = [];
    this.state = this.simPackage.initState(this.initParams, this.seed);
  }

  lastEvent(): SimEvent | undefined {
    return this.events[this.events.length - 1];
  }

  /** 教学画面、状态和检查点全部通过协议校验后才交给宿主。 */
  snapshot(): RuntimeSnapshot {
    const view = this.simPackage.render(this.state);
    assertValidTeachingFrame(view, this.simPackage.meta.scaleLimit);
    const checkpointResults: RuntimeSnapshot['checkpointResults'] = {};
    for (const checkpoint of this.simPackage.checkpoints) {
      checkpointResults[checkpoint.id] = checkpoint.evaluate(this.state);
    }
    const interactionAvailability: Record<string, boolean> = {};
    for (const interaction of this.simPackage.interactions) {
      interactionAvailability[interaction.id] = this.isInteractionAvailable(interaction);
    }
    const snapshot = {
      state: this.state,
      tick: this.tick,
      events: [...this.events],
      view,
      currentStep: this.currentNarrativeStep(),
      interactionAvailability,
      checkpointResults,
    };
    if (!isRuntimeFrame(snapshot)) throw new Error('仿真运行画面不完整，请联系发布者处理');
    return snapshot;
  }

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

  // 每个事件派生独立随机流，回放不依赖宿主进程此前取过多少次随机数。
  private reducerContext(tick: number, seq: number): ReducerContext {
    return {
      seed: this.seed,
      tick,
      seq,
      random: new XorShiftRandom(hashSeed(this.seed, `${this.simPackage.meta.code}:${tick}:${seq}`)),
    };
  }

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

  private isInteractionAvailable(interaction: SimPackage['interactions'][number]): boolean {
    if (interaction.availableWhen && !interaction.availableWhen(this.state)) return false;
    // 恢复只能发生在尚未恢复的攻击之后；包的其他条件由 availableWhen 声明。
    if (interaction.emits === 'advance' && !this.wouldChangeState(interaction, 'user')) return false;
    if (interaction.emits !== 'recover') return true;
    const lastAttack = this.events.reduce((index, event, currentIndex) => event.type === 'attack' ? currentIndex : index, -1);
    const lastRecover = this.events.reduce((index, event, currentIndex) => event.type === 'recover' ? currentIndex : index, -1);
    return lastAttack > lastRecover;
  }

  // 预演业务状态以关闭终态推进，事件序号增加本身不代表算法有进展。
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

  private enforceEventLimit(eventInput: Omit<SimEvent, 'seq' | 'atTick'>): void {
    const limits = this.simPackage.meta.scaleLimit;
    if (eventInput.source === 'tick' && this.tick >= limits.maxTick) {
      throw new Error('仿真步骤数量超过限制,请调整场景规模');
    }
    if (this.events.length >= limits.maxEvents) {
      throw new Error('仿真事件数量超过限制,请调整场景规模');
    }
  }

  private currentNarrativeStep(): NarrativeStepDescriptor | undefined {
    const matched = this.simPackage.narrative.find((step) => step.trigger(this.state));
    return matched ? stripNarrativeStep(matched) : undefined;
  }
}

function describePackage(pkg: SimPackage): SimPackageDescriptor {
  return {
    meta: pkg.meta,
    interactions: pkg.interactions.map(({ availableWhen: _availableWhen, ...interaction }) => interaction),
    narrative: pkg.narrative.map(stripNarrativeStep),
    codeTrace: pkg.codeTrace,
    checkpoints: pkg.checkpoints.map<CheckpointDescriptor>((checkpoint) => ({
      id: checkpoint.id,
      label: checkpoint.label,
    })),
  };
}

function stripNarrativeStep({ trigger: _trigger, ...descriptor }: SimPackage['narrative'][number]): NarrativeStepDescriptor {
  return descriptor;
}

// 平台控件可生成这些固定字段，算法自定义参数仍必须声明在 params 中。
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

// select 与后端使用相同的标量文本比较规则。
function scalarPayloadString(value: JsonValue | undefined): string | undefined {
  if (typeof value === 'string') {
    return value.trim() || undefined;
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value);
  }
  return undefined;
}
