// 本文件是仿真包在浏览器 Web Worker 内的执行宿主,主线程不得直接运行仿真代码。
//
// 它只做三件事:封锁 Worker 能力、按 code 装配平台内置包、把主线程命令转成引擎调用。
// 状态机推进、事件白名单校验、快照求值与回退重放全部在共享的 SimEngine 内
// (见 runtime/engine.ts)—— 引擎有两份实现就必然与容器宿主漂移,
// 而漂移的表现是"同一个 seed 在两个宿主跑出两条过程",回放、分享与判分随之失效。
//
// 只装配平台内置包:教师与第三方扩展包在后端隔离容器内运行,归档字节从不下发浏览器
// (理由见 docs/04-仿真可视化引擎/07-安全设计.md §2)。

import type { JsonObject, SimInitParams, SimState } from '../types';
import { SimEngine } from './engine';
import { blockedCapability, installDeterminismGuards, sealGlobal } from './guards';
import { getBuiltinSimulation } from '../registry/builtinRegistry';

type WorkerRequest =
  | { type: 'init'; requestId: number; builtinCode: string; initParams: SimInitParams; seed: number }
  | { type: 'step'; requestId: number }
  | { type: 'inject'; requestId: number; eventType: string; payload: JsonObject; target?: string }
  | { type: 'sync-state'; requestId: number; tick: number; state: SimState }
  | { type: 'back'; requestId: number }
  | { type: 'reset'; requestId: number };

let engine: SimEngine | undefined;
const postToMain = self.postMessage.bind(self);

self.addEventListener('message', (event: MessageEvent<WorkerRequest>) => {
  handleRequest(event.data);
});
installRuntimeGuards();

/**
 * handleRequest 分发主线程命令,并把所有异常统一转成用户向错误响应。
 */
function handleRequest(request: WorkerRequest): void {
  try {
    switch (request.type) {
      case 'init':
        init(request);
        return;
      case 'step':
        readyEngine().step();
        postSnapshot(request.requestId, true);
        return;
      case 'inject':
        readyEngine().inject(request.eventType, request.payload, request.target);
        postSnapshot(request.requestId, true);
        return;
      case 'sync-state':
        readyEngine().syncState(request.tick, request.state);
        postSnapshot(request.requestId);
        return;
      case 'back':
        readyEngine().back();
        postSnapshot(request.requestId);
        return;
      case 'reset':
        readyEngine().reset();
        postSnapshot(request.requestId);
        return;
    }
  } catch (error) {
    reportRuntimeError(request, error);
  }
}

/**
 * init 装配平台内置包并生成首个运行快照。
 */
function init(request: Extract<WorkerRequest, { type: 'init' }>): void {
  const builtinPackage = getBuiltinSimulation(request.builtinCode);
  if (!builtinPackage) {
    throw new Error('当前仿真场景尚未在平台内置库中上架');
  }
  engine = new SimEngine(builtinPackage, request.initParams, request.seed);
  postToMain({
    type: 'ready',
    requestId: request.requestId,
    descriptor: engine.describe(),
    snapshot: engine.snapshot(),
  });
}

/**
 * readyEngine 返回已装配的引擎,未初始化时明确失败而不静默继续。
 */
function readyEngine(): SimEngine {
  if (!engine) {
    throw new Error('仿真运行环境尚未准备好,请稍后重试');
  }
  return engine;
}

/**
 * postSnapshot 把最新纯数据快照发送给主线程;推进类命令附带刚产生的事件。
 */
function postSnapshot(requestId: number, withEvent = false): void {
  const current = readyEngine();
  postToMain({
    type: 'snapshot',
    requestId,
    snapshot: current.snapshot(),
    event: withEvent ? current.lastEvent() : undefined,
  });
}

/**
 * installRuntimeGuards 禁止仿真包访问网络、嵌套 Worker、真实时间和非确定性随机源。
 *
 * 随机与时间走共享口径(见 runtime/guards.ts):容器宿主必须与这里完全一致,
 * 否则同一个 seed 在两处跑出两条过程。本函数只声明浏览器这一侧特有的宿主入口。
 * `fetch` 给一个拒绝的 Promise 而不是 undefined:包里常见写法是 `await fetch(...)`,
 * 拒绝能让它拿到明确原因,而 undefined 会抛 TypeError,排查时看不出是被平台封锁的。
 */
function installRuntimeGuards(): void {
  const scope = self;
  installDeterminismGuards(scope);
  sealGlobal(scope, 'fetch', () => Promise.reject(new Error('仿真包网络访问不被允许')));
  for (const name of ['XMLHttpRequest', 'WebSocket', 'EventSource', 'Worker', 'SharedWorker', 'BroadcastChannel', 'indexedDB', 'caches']) {
    sealGlobal(scope, name, undefined);
  }
  for (const name of ['importScripts', 'eval', 'Function', 'postMessage', 'addEventListener']) {
    sealGlobal(scope, name, blockedCapability);
  }
}

/**
 * reportRuntimeError 记录可定位的运行错误,只把友好提示返回给主线程。
 * 仿真在浏览器本地执行,报障编号只能由后端签发;前端自造编号在运维侧查不到,
 * 因此技术原因只进控制台结构化日志,不进用户可见文案。
 */
function reportRuntimeError(request: WorkerRequest, error: unknown): void {
  console.error('sim_worker_error', {
    operation: request.type,
    request_id: request.requestId,
    error: error instanceof Error ? error.message : String(error),
  });
  postToMain({ type: 'error', requestId: request.requestId, message: runtimeErrorMessage(error) });
}

/** runtimeErrorMessage 将可恢复的规模边界转成用户可执行的提示,其他错误仍保持通用文案。 */
function runtimeErrorMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  if (message === '仿真步骤数量超过限制,请调整场景规模') {
    return '仿真已达到规模上限,请重新开始或调整场景规模。';
  }
  return '仿真运行失败,请刷新后重试';
}
