// 本文件在 Web Worker 中执行仿真包状态机,主线程不得直接运行第三方仿真代码。

import type {
  FieldDef,
  CheckpointDescriptor,
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
  TreeNode,
  ViewSpec,
} from '../types';
import { fnv1aHex, hashSeed, XorShiftRandom } from './deterministic';
import { getBuiltinSimulation } from '../registry/builtinRegistry';

type WorkerRequest =
  | { type: 'init'; requestId: number; moduleUrl?: string; builtinCode?: string; initParams: SimInitParams; seed: number }
  | { type: 'step'; requestId: number }
  | { type: 'inject'; requestId: number; eventType: string; payload: JsonObject; target?: string }
  | { type: 'back'; requestId: number }
  | { type: 'reset'; requestId: number };

let simPackage: SimPackage | undefined;
let descriptor: SimPackageDescriptor | undefined;
let initParams: SimInitParams = {};
let seed = 0;
let state: SimState | undefined;
let tick = 0;
let seq = 1;
let events: SimEvent[] = [];
const postToMain = self.postMessage.bind(self);

self.addEventListener('message', (event: MessageEvent<WorkerRequest>) => {
  void handleRequest(event.data);
});
installRuntimeGuards();

/**
 * handleRequest 分发主线程命令,并把所有异常统一转成用户向错误响应。
 */
async function handleRequest(request: WorkerRequest): Promise<void> {
  try {
    switch (request.type) {
      case 'init':
        await init(request);
        return;
      case 'step':
        ensureReady();
        applyEvent({ type: 'tick', source: 'tick', payload: {}, target: undefined });
        postSnapshot(request.requestId, events[events.length - 1]);
        return;
      case 'inject':
        ensureReady();
        applyEvent(userEventInput(request.eventType, request.payload, request.target));
        postSnapshot(request.requestId, events[events.length - 1]);
        return;
      case 'back':
        ensureReady();
        replay(events.slice(0, -1));
        postSnapshot(request.requestId);
        return;
      case 'reset':
        ensureReady();
        resetState();
        postSnapshot(request.requestId);
        return;
    }
  } catch (error) {
    reportRuntimeError(request, error);
  }
}

/**
 * init 动态加载仿真包,校验协议结构并生成首个运行快照。
 */
async function init(request: Extract<WorkerRequest, { type: 'init' }>): Promise<void> {
  simPackage = await loadPackage(request);
  assertPackage(simPackage);
  initParams = request.initParams;
  seed = request.seed;
  descriptor = describePackage(simPackage);
  resetState();
  postToMain({ type: 'ready', requestId: request.requestId, descriptor, snapshot: snapshot() });
}

/**
 * loadPackage 统一装配平台内置包和外部 bundle 包；内置包只在 SDK Worker 内部解析,业务页面不复制包清单。
 */
async function loadPackage(request: Extract<WorkerRequest, { type: 'init' }>): Promise<SimPackage> {
  if (request.builtinCode) {
    const builtinPackage = getBuiltinSimulation(request.builtinCode);
    if (!builtinPackage) {
      throw new Error('当前内置仿真包尚未完成平台装配');
    }
    return builtinPackage;
  }
  if (!request.moduleUrl) {
    throw new Error('当前仿真包缺少可运行模块地址');
  }
  const loaded = (await import(/* @vite-ignore */ request.moduleUrl)) as { default?: SimPackage; simPackage?: SimPackage };
  const modulePackage = loaded.default ?? loaded.simPackage;
  if (!modulePackage) {
    throw new Error('当前仿真包无法运行,请联系管理员检查包配置');
  }
  return modulePackage;
}

/**
 * userEventInput 统一构造用户事件:reducer 读顶层 target,操作日志按后端契约在 payload.target 中持久化。
 */
function userEventInput(eventType: string, payload: JsonObject, target?: string): Omit<SimEvent, 'seq' | 'atTick'> {
  const nextPayload: JsonObject = { ...(payload ?? {}) };
  if (target) {
    nextPayload.target = target;
  }
  return { type: eventType, source: 'user', payload: nextPayload, target };
}

/**
 * ensureReady 统一校验 worker 是否完成初始化,供只需要状态校验的消息分支使用。
 */
function ensureReady(): void {
  void readyPackage();
}

/**
 * readyPackage 返回已初始化的仿真包,让后续状态机逻辑获得明确的类型收窄结果。
 */
function readyPackage(): SimPackage {
  if (!simPackage || !descriptor) {
    throw new Error('仿真运行环境尚未准备好，请稍后重试');
  }
  return simPackage;
}

/**
 * currentState 返回当前状态快照,避免未初始化时静默继续执行。
 */
function currentState(): SimState {
  if (!state) {
    throw new Error('仿真状态尚未初始化，请重新进入仿真工作台');
  }
  return state;
}

/**
 * resetState 按初始参数和 seed 重建确定性初始状态。
 */
function resetState(): void {
  const pkg = readyPackage();
  tick = 0;
  seq = 1;
  events = [];
  state = pkg.initState(initParams, seed);
}

/**
 * applyEvent 构造带 seq 和 tick 的事件,并用确定性上下文推进 reducer。
 */
function applyEvent(eventInput: Omit<SimEvent, 'seq' | 'atTick'>): void {
  const pkg = readyPackage();
  const previousState = currentState();
  enforceEventLimit(eventInput);
  enforceEventSchema(pkg, eventInput);
  const event: SimEvent = { ...eventInput, atTick: tick, seq };
  const context: ReducerContext = {
    seed,
    tick,
    seq,
    random: new XorShiftRandom(hashSeed(seed, `${pkg.meta.code}:${tick}:${seq}`)),
  };
  state = pkg.reducer(previousState, event, context);
  seq += 1;
  if (event.source === 'tick') {
    tick += 1;
  }
  events.push(event);
}

/**
 * enforceEventSchema 复用仿真包交互声明校验事件,避免前端可运行但后端动作日志拒绝。
 */
function enforceEventSchema(pkg: SimPackage, eventInput: Omit<SimEvent, 'seq' | 'atTick'>): void {
  if (eventInput.source !== 'user') {
    return;
  }
  const interaction = pkg.interactions.find((item) => item.emits === eventInput.type);
  if (!interaction) {
    throw new Error('当前仿真包不支持这个操作');
  }
  if ((interaction.target === 'element' || interaction.kind === 'select-element') && !eventInput.target) {
    throw new Error('请先选择要操作的仿真对象');
  }
  if (interaction.target !== 'element' && interaction.kind !== 'select-element' && eventInput.target) {
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
      throw new Error('操作参数不完整，请检查后重试');
    }
  }
  for (const field of interaction.params ?? []) {
    if (field.required && payload[field.name] === undefined) {
      throw new Error('请补全操作参数后再继续');
    }
  }
}

/**
 * platformPayloadValueMatchesInteraction 校验平台通用控件自动生成的固定字段,算法字段仍必须走 params。
 */
function platformPayloadValueMatchesInteraction(key: string, value: JsonValue | undefined, interaction: SimPackage['interactions'][number]): boolean {
  if (key === 'target') {
    return (interaction.target === 'element' || interaction.kind === 'select-element') && typeof value === 'string' && value.trim().length > 0 && value.length <= 128;
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
  if (key === 'startX' || key === 'startY' || key === 'currentX' || key === 'currentY' || key === 'deltaX' || key === 'deltaY') {
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
    return valueText !== undefined && Boolean(field.options?.some((option) => scalarPayloadString(option.value) === valueText));
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

/**
 * replay 从初始状态重放事件日志,用于回退时保持状态可复现。
 */
function replay(nextEvents: SimEvent[]): void {
  const pkg = readyPackage();
  tick = 0;
  seq = 1;
  events = [];
  let replayState = pkg.initState(initParams, seed);
  for (const event of nextEvents) {
    const context: ReducerContext = {
      seed,
      tick: event.atTick,
      seq: event.seq,
      random: new XorShiftRandom(hashSeed(seed, `${pkg.meta.code}:${event.atTick}:${event.seq}`)),
    };
    replayState = pkg.reducer(replayState, event, context);
    tick = event.source === 'tick' ? event.atTick + 1 : event.atTick;
    seq = event.seq + 1;
    events.push(event);
  }
  state = replayState;
}

/**
 * snapshot 汇总状态、视图、叙事、交互可用性和检查点结果。
 */
function snapshot(): RuntimeSnapshot {
  const pkg = readyPackage();
  const current = currentState();
  const currentStep = currentNarrativeStep();
  const view = pkg.render(current);
  enforceViewLimit(view);
  const checkpointResults: RuntimeSnapshot['checkpointResults'] = {};
  for (const checkpoint of pkg.checkpoints ?? []) {
    checkpointResults[checkpoint.id] = checkpoint.evaluate(current);
  }
  const interactionAvailability: Record<string, boolean> = {};
  for (const interaction of pkg.interactions) {
    interactionAvailability[interaction.id] = interaction.availableWhen ? interaction.availableWhen(current) : true;
  }
  return {
    state: current,
    tick,
    events: [...events],
    view,
    currentStep,
    interactionAvailability,
    checkpointResults,
  };
}

/**
 * postSnapshot 把最新纯数据快照发送给主线程。
 */
function postSnapshot(requestId: number, event?: SimEvent): void {
  postToMain({ type: 'snapshot', requestId, snapshot: snapshot(), event });
}

/**
 * currentNarrativeStep 根据当前状态选择正在触发的叙事步骤。
 */
function currentNarrativeStep(): NarrativeStepDescriptor | undefined {
  const pkg = readyPackage();
  const current = currentState();
  const steps = pkg.narrative ?? [];
  const matched = steps.find((step) => step.trigger(current));
  return stripNarrativeStep(matched ?? steps[0]);
}

/**
 * describePackage 把含函数的 SimPackage 收窄成可发送给主线程的描述符。
 */
function describePackage(pkg: SimPackage): SimPackageDescriptor {
  return {
    meta: pkg.meta,
    interactions: pkg.interactions.map(({ availableWhen: _availableWhen, ...interaction }) => interaction),
    narrative: (pkg.narrative ?? []).map(stripNarrativeStep).filter((step): step is NarrativeStepDescriptor => Boolean(step)),
    codeTrace: pkg.codeTrace,
    checkpoints: (pkg.checkpoints ?? []).map<CheckpointDescriptor>((checkpoint) => ({ id: checkpoint.id, label: checkpoint.label })),
  };
}

/**
 * stripNarrativeStep 移除叙事触发函数,只保留可序列化的教学描述。
 */
function stripNarrativeStep(step?: NonNullable<SimPackage['narrative']>[number]): NarrativeStepDescriptor | undefined {
  if (!step) {
    return undefined;
  }
  const descriptorStep = { ...step } as NarrativeStepDescriptor & { trigger?: unknown };
  delete descriptorStep.trigger;
  return descriptorStep;
}

/**
 * installRuntimeGuards 禁止仿真包访问网络、嵌套 Worker、真实时间和非确定性随机源。
 */
function installRuntimeGuards(): void {
  const scope = self as unknown as Record<string, unknown>;
  const blocked = (): never => {
    throw new Error('仿真包能力不被允许');
  };
  Math.random = blocked;
  Date.now = blocked;
  blockWorkerGlobal(scope, 'fetch', () => Promise.reject(new Error('仿真包网络访问不被允许')));
  blockWorkerGlobal(scope, 'XMLHttpRequest', undefined);
  blockWorkerGlobal(scope, 'WebSocket', undefined);
  blockWorkerGlobal(scope, 'EventSource', undefined);
  blockWorkerGlobal(scope, 'Worker', undefined);
  blockWorkerGlobal(scope, 'SharedWorker', undefined);
  blockWorkerGlobal(scope, 'BroadcastChannel', undefined);
  blockWorkerGlobal(scope, 'indexedDB', undefined);
  blockWorkerGlobal(scope, 'caches', undefined);
  blockWorkerGlobal(scope, 'importScripts', blocked);
  blockWorkerGlobal(scope, 'eval', blocked);
  blockWorkerGlobal(scope, 'Function', blocked);
  blockWorkerGlobal(scope, 'postMessage', blocked);
  blockWorkerGlobal(scope, 'addEventListener', blocked);
}

/**
 * blockWorkerGlobal 覆盖 Worker 全局能力,只接受可重新定义的属性。
 */
function blockWorkerGlobal(scope: Record<string, unknown>, key: string, value: unknown): void {
  const descriptor = findPropertyDescriptor(scope, key);
  if (descriptor && descriptor.configurable === false) {
    throw new Error('仿真运行环境无法封锁必要浏览器能力');
  }
  Object.defineProperty(scope, key, {
    value,
    configurable: true,
    writable: false,
  });
}

/**
 * findPropertyDescriptor 沿原型链查找 Worker 全局属性描述符。
 */
function findPropertyDescriptor(scope: Record<string, unknown>, key: string): PropertyDescriptor | undefined {
  let cursor: object | null = scope;
  while (cursor) {
    const descriptor = Object.getOwnPropertyDescriptor(cursor, key);
    if (descriptor) {
      return descriptor;
    }
    cursor = Object.getPrototypeOf(cursor);
  }
  return undefined;
}

/**
 * assertPackage 校验加载模块是否满足最小 SimPackage 协议。
 */
function assertPackage(pkg: SimPackage | undefined): asserts pkg is SimPackage {
  if (
    !pkg ||
    typeof pkg.initState !== 'function' ||
    typeof pkg.reducer !== 'function' ||
    typeof pkg.render !== 'function' ||
    !pkg.meta ||
    !Array.isArray(pkg.interactions)
  ) {
    throw new Error('仿真包内容不完整，请联系发布者处理');
  }
}

/**
 * enforceEventLimit 执行仿真包声明的 tick 和事件规模上限。
 */
function enforceEventLimit(eventInput: Omit<SimEvent, 'seq' | 'atTick'>): void {
  const pkg = readyPackage();
  if (eventInput.source === 'tick' && tick >= pkg.meta.scaleLimit.maxTick) {
    throw new Error('仿真步骤数量超过限制，请调整场景规模');
  }
  if (events.length >= pkg.meta.scaleLimit.maxEvents) {
    throw new Error('仿真事件数量超过限制，请调整场景规模');
  }
}

/**
 * enforceViewLimit 执行封闭模式数量和节点规模约束,防止仿真包撑破渲染器。
 */
function enforceViewLimit(view: ViewSpec): void {
  const pkg = readyPackage();
  if (view.patterns.length < 1 || view.patterns.length > 3) {
    throw new Error('仿真视图数量超过限制，请调整场景规模');
  }
  if (countRenderableNodes(view) > pkg.meta.scaleLimit.nodes) {
    throw new Error('仿真节点数量超过限制，请调整场景规模');
  }
}

/**
 * countRenderableNodes 统计所有封闭模式中的可渲染元素数量。
 */
function countRenderableNodes(view: ViewSpec): number {
  return view.patterns.reduce((total, pattern) => {
    if (pattern.mode === 'graph') {
      return total + pattern.data.nodes.length;
    }
    if (pattern.mode === 'chain') {
      return total + pattern.data.blocks.length + pattern.data.forks.reduce((sum, fork) => sum + fork.length, 0);
    }
    if (pattern.mode === 'tree') {
      return total + countTreeNodes(pattern.data.root);
    }
    if (pattern.mode === 'matrix') {
      return total + pattern.data.rows.length * pattern.data.columns.length;
    }
    if (pattern.mode === 'pipeline') {
      return total + pattern.data.steps.length;
    }
    if (pattern.mode === 'lane') {
      return total + pattern.data.actors.length + pattern.data.messages.length;
    }
    return total + pattern.data.series.reduce((sum, series) => sum + series.points.length, 0);
  }, 0);
}

/**
 * countTreeNodes 递归统计树模式节点数。
 */
function countTreeNodes(node: TreeNode): number {
  return 1 + (node.children ?? []).reduce((total, child) => total + countTreeNodes(child), 0);
}

/**
 * reportRuntimeError 记录可定位的运行错误,只把友好提示和追踪编号返回给前端。
 */
function reportRuntimeError(request: WorkerRequest, error: unknown): void {
  const traceId = fnv1aHex(`sim-worker:${request.type}:${request.requestId}:${error instanceof Error ? error.message : String(error)}`, 12);
  console.error('sim_worker_error', {
    trace_id: traceId,
    tenant_id: 'frontend-local',
    operation: request.type,
    error: error instanceof Error ? error.message : String(error),
  });
  postToMain({ type: 'error', requestId: request.requestId, message: `仿真运行失败,请刷新后重试。如需帮助,请提供编号 ${traceId}` });
}
