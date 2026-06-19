// 本文件在 Web Worker 中执行仿真包状态机,主线程不得直接运行第三方仿真代码。

import type {
  CheckpointDescriptor,
  JsonObject,
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
import { hashSeed, XorShiftRandom } from './deterministic';

type WorkerRequest =
  | { type: 'init'; requestId: number; moduleUrl: string; initParams: SimInitParams; seed: number }
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
        applyEvent({ type: request.eventType, source: 'user', payload: request.payload, target: request.target });
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
  } catch {
    postToMain({ type: 'error', requestId: request.requestId, message: '仿真运行失败,请刷新后重试' });
  }
}

async function init(request: Extract<WorkerRequest, { type: 'init' }>): Promise<void> {
  const loaded = (await import(/* @vite-ignore */ request.moduleUrl)) as { default?: SimPackage; simPackage?: SimPackage };
  simPackage = loaded.default ?? loaded.simPackage;
  assertPackage(simPackage);
  initParams = request.initParams;
  seed = request.seed;
  descriptor = describePackage(simPackage);
  resetState();
  postToMain({ type: 'ready', requestId: request.requestId, descriptor, snapshot: snapshot() });
}

// ensureReady 统一校验 worker 是否完成初始化,供只需要状态校验的消息分支使用。
function ensureReady(): void {
  void readyPackage();
}

// readyPackage 返回已初始化的仿真包,让后续状态机逻辑获得明确的类型收窄结果。
function readyPackage(): SimPackage {
  if (!simPackage || !descriptor) {
    throw new Error('sim worker not initialized');
  }
  return simPackage;
}

function currentState(): SimState {
  if (!state) {
    throw new Error('sim worker state not initialized');
  }
  return state;
}

function resetState(): void {
  const pkg = readyPackage();
  tick = 0;
  seq = 1;
  events = [];
  state = pkg.initState(initParams, seed);
}

function applyEvent(eventInput: Omit<SimEvent, 'seq' | 'atTick'>): void {
  const pkg = readyPackage();
  const previousState = currentState();
  enforceEventLimit(eventInput);
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

function postSnapshot(requestId: number, event?: SimEvent): void {
  postToMain({ type: 'snapshot', requestId, snapshot: snapshot(), event });
}

function currentNarrativeStep(): NarrativeStepDescriptor | undefined {
  const pkg = readyPackage();
  const current = currentState();
  const steps = pkg.narrative ?? [];
  const matched = steps.find((step) => step.trigger(current));
  return stripNarrativeStep(matched ?? steps[0]);
}

function describePackage(pkg: SimPackage): SimPackageDescriptor {
  return {
    meta: pkg.meta,
    interactions: pkg.interactions.map(({ availableWhen: _availableWhen, ...interaction }) => interaction),
    narrative: (pkg.narrative ?? []).map(stripNarrativeStep).filter((step): step is NarrativeStepDescriptor => Boolean(step)),
    codeTrace: pkg.codeTrace,
    checkpoints: (pkg.checkpoints ?? []).map<CheckpointDescriptor>((checkpoint) => ({ id: checkpoint.id, label: checkpoint.label })),
  };
}

function stripNarrativeStep(step?: NonNullable<SimPackage['narrative']>[number]): NarrativeStepDescriptor | undefined {
  if (!step) {
    return undefined;
  }
  const { trigger: _trigger, ...descriptorStep } = step;
  return descriptorStep;
}

function installRuntimeGuards(): void {
  const scope = self as unknown as Record<string, unknown>;
  const blocked = (): never => {
    throw new Error('仿真包能力不被允许');
  };
  scope.fetch = () => Promise.reject(new Error('仿真包网络访问不被允许'));
  scope.XMLHttpRequest = undefined;
  scope.WebSocket = undefined;
  scope.EventSource = undefined;
  scope.Worker = undefined;
  scope.SharedWorker = undefined;
  scope.BroadcastChannel = undefined;
  scope.indexedDB = undefined;
  scope.caches = undefined;
  scope.importScripts = blocked;
  scope.eval = blocked;
  scope.Function = blocked;
  scope.postMessage = blocked;
  scope.addEventListener = blocked;
}

function assertPackage(pkg: SimPackage | undefined): asserts pkg is SimPackage {
  if (
    !pkg ||
    typeof pkg.initState !== 'function' ||
    typeof pkg.reducer !== 'function' ||
    typeof pkg.render !== 'function' ||
    !pkg.meta ||
    !Array.isArray(pkg.interactions)
  ) {
    throw new Error('invalid sim package');
  }
}

function enforceEventLimit(eventInput: Omit<SimEvent, 'seq' | 'atTick'>): void {
  const pkg = readyPackage();
  if (eventInput.source === 'tick' && tick >= pkg.meta.scaleLimit.maxTick) {
    throw new Error('sim package tick limit exceeded');
  }
  if (events.length >= pkg.meta.scaleLimit.maxEvents) {
    throw new Error('sim package event limit exceeded');
  }
}

function enforceViewLimit(view: ViewSpec): void {
  const pkg = readyPackage();
  if (countRenderableNodes(view) > pkg.meta.scaleLimit.nodes) {
    throw new Error('sim package node limit exceeded');
  }
}

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

function countTreeNodes(node: TreeNode): number {
  return 1 + (node.children ?? []).reduce((total, child) => total + countTreeNodes(child), 0);
}
