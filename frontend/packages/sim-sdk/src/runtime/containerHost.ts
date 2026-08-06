// 本文件是仿真包在后端隔离容器内的执行宿主,实现 stdio-json 协议。
//
// 为什么在 sim-sdk 而不在 images/ 下:它与浏览器 Worker 宿主(sim.worker.ts)共用同一个
// SimEngine —— 引擎有两份实现就必然漂移,而漂移的表现是"同一个 seed 在两个宿主跑出两条过程",
// 回放、分享与判分随之失效。故两个宿主同居 runtime/ 目录、共享引擎与协议校验;
// images/sim/package-runner/ 只保留 Dockerfile、manifest 与 README(镜像目录不放脚本)。
//
// 协议(与 SIM_BACKEND_STDIO_ADAPTERS_JSON 的 stdio-json 一致):
//   一次 exec = 读一行 JSON 命令 → 写一行 JSON 响应 → 退出。
//   后端为每个事件发起一次 exec,容器不常驻状态 —— 状态由后端在命令里带回,
//   这样容器崩溃或被重建都不丢过程,也不需要在容器里维护会话生命周期。
//
// 该文件刻意不从 sim-sdk 主入口导出:它 import 了 node:crypto/fs/zlib,
// 被浏览器包静态引用会污染前端产物。

import { installContainerGuards } from './containerGuards';
import { loadPackageFromBundle } from './containerBundle';
import { SimEngine } from './engine';
import type { JsonObject, SimEvent, SimInitParams, SimState } from '../types';

/** RunnerCommand 是后端经 stdin 下发的单条命令。 */
export type RunnerCommand =
  | {
      op: 'init';
      bundle_base64: string;
      bundle_hash: string;
      bundle_format: 'zip' | 'tar';
      entry: string;
      init_params: SimInitParams;
      seed: number;
    }
  | {
      op: 'apply';
      bundle_base64: string;
      bundle_hash: string;
      bundle_format: 'zip' | 'tar';
      entry: string;
      init_params: SimInitParams;
      seed: number;
      events: SimEvent[];
      next: { type: 'tick' } | { type: 'user'; event_type: string; payload: JsonObject; target?: string };
    }
  | {
      op: 'restore';
      bundle_base64: string;
      bundle_hash: string;
      bundle_format: 'zip' | 'tar';
      entry: string;
      init_params: SimInitParams;
      seed: number;
      events: SimEvent[];
    }
  | {
      op: 'verify';
      bundle_base64: string;
      bundle_hash: string;
      bundle_format: 'zip' | 'tar';
      entry: string;
      init_params: SimInitParams;
      seed: number;
      frame_count: number;
    };

/** RunnerSnapshot 是回传给后端的完整教学快照;扩展包的 render 不在浏览器执行,故帧在此产出。 */
interface RunnerSnapshot {
  tick: number;
  state: SimState;
  view: unknown;
  current_step?: unknown;
  interaction_availability: Record<string, boolean>;
  checkpoint_results: Record<string, unknown>;
  events: SimEvent[];
}

/** RunnerResponse 只有成功与失败两种形态;失败必须带可定位的原因,不静默返回空帧。 */
type RunnerResponse =
  | { ok: true; descriptor: unknown; snapshot: RunnerSnapshot }
  | { ok: true; determinism: 'passed' | 'failed'; frames: RunnerSnapshot[]; detail?: string }
  | { ok: false; error: string };

/**
 * runCommand 执行一条命令并返回响应。
 * 装配前先封锁容器能力:包的顶层代码在 import 那一刻就会执行,封锁晚一步就等于没封。
 */
export async function runCommand(command: RunnerCommand): Promise<RunnerResponse> {
  try {
    installContainerGuards();
    switch (command.op) {
      case 'init':
        return await runInit(command);
      case 'apply':
        return await runApply(command);
      case 'restore':
        return await runRestore(command);
      case 'verify':
        return await runVerify(command);
      default:
        return { ok: false, error: `未知命令: ${String((command as { op?: unknown }).op)}` };
    }
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
}

/**
 * runInit 装配仿真包并产出 tick=0 的首帧快照。
 */
async function runInit(command: Extract<RunnerCommand, { op: 'init' }>): Promise<RunnerResponse> {
  const engine = await createEngine(command);
  return { ok: true, descriptor: engine.describe(), snapshot: toSnapshot(engine) };
}

/**
 * runApply 从 seed + 已有事件重放到当前位置,再执行下一条事件。
 *
 * 为什么每次都重放而不在容器里保状态:容器是短命的(一次 exec 一个进程),而重放代价可控 ——
 * 事件数受包声明 max_events 约束。用重放换"无状态容器"是有意的取舍:它让容器崩溃、
 * Pod 重建、后端重启都不影响过程可复现性,也不需要在容器侧维护会话生命周期。
 */
async function runApply(command: Extract<RunnerCommand, { op: 'apply' }>): Promise<RunnerResponse> {
  const engine = await createEngine(command);
  engine.restore(command.events);
  if (command.next.type === 'tick') {
    engine.step();
  } else {
    engine.inject(command.next.event_type, command.next.payload ?? {}, command.next.target);
  }
  return { ok: true, descriptor: engine.describe(), snapshot: toSnapshot(engine) };
}

/**
 * runRestore 只把过程重放到给定事件位置,不再追加新事件。
 *
 * 后端在回退与「从头再来」时用它:回退是"基于确定性重算到上一 tick"(M4 需求 C2),
 * 不是就地反算 —— 少一条事件重放一遍即得到上一步的状态,与浏览器 Worker 的 back 同一口径。
 */
async function runRestore(command: Extract<RunnerCommand, { op: 'restore' }>): Promise<RunnerResponse> {
  const engine = await createEngine(command);
  engine.restore(command.events ?? []);
  return { ok: true, descriptor: engine.describe(), snapshot: toSnapshot(engine) };
}

/**
 * runVerify 是上架前的隔离预览:同 seed 同参数跑两遍逐帧比对,并回传样例教学帧。
 *
 * 两件事一次做完:确定性结论(reducer 是否纯函数)与样例帧(供平台管理员判断算法实现对不对)。
 * 自动校验回答不了"这个 PBFT 对不对",所以必须把帧摊给人看。
 */
async function runVerify(command: Extract<RunnerCommand, { op: 'verify' }>): Promise<RunnerResponse> {
  const count = Math.max(1, Math.min(32, Math.trunc(command.frame_count) || 1));
  const first = await collectFrames(command, count);
  const second = await collectFrames(command, count);
  const firstJSON = JSON.stringify(first);
  const secondJSON = JSON.stringify(second);
  if (firstJSON !== secondJSON) {
    return {
      ok: true,
      determinism: 'failed',
      frames: first,
      detail: '同一随机种子两次运行产生了不同过程,仿真包的状态推进不是纯函数。',
    };
  }
  return { ok: true, determinism: 'passed', frames: first };
}

/**
 * collectFrames 从初始状态连续推进,收集指定数量的教学帧。
 */
async function collectFrames(
  command: Extract<RunnerCommand, { op: 'verify' }>,
  count: number,
): Promise<RunnerSnapshot[]> {
  const engine = await createEngine(command);
  const frames: RunnerSnapshot[] = [toSnapshot(engine)];
  for (let index = 1; index < count; index += 1) {
    engine.step();
    frames.push(toSnapshot(engine));
  }
  return frames;
}

/**
 * createEngine 校验哈希、解包归档、装配入口模块并构造引擎。
 */
async function createEngine(command: {
  bundle_base64: string;
  bundle_hash: string;
  bundle_format: 'zip' | 'tar';
  entry: string;
  init_params: SimInitParams;
  seed: number;
}): Promise<SimEngine> {
  const simPackage = await loadPackageFromBundle({
    bundleBase64: command.bundle_base64,
    bundleHash: command.bundle_hash,
    format: command.bundle_format,
    entry: command.entry,
  });
  return new SimEngine(simPackage, command.init_params ?? {}, command.seed);
}

/**
 * toSnapshot 把引擎快照转成后端契约的下划线字段形态。
 */
function toSnapshot(engine: SimEngine): RunnerSnapshot {
  const snapshot = engine.snapshot();
  return {
    tick: snapshot.tick,
    state: snapshot.state,
    view: snapshot.view,
    current_step: snapshot.currentStep,
    interaction_availability: snapshot.interactionAvailability,
    checkpoint_results: snapshot.checkpointResults,
    events: snapshot.events,
  };
}
