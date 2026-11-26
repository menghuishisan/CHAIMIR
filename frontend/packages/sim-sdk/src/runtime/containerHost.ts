// 容器进程无会话状态，每条命令用已确认的事件重放到指定位置。

import { installContainerGuards } from './containerGuards';
import { loadPackageFromBundle } from './containerBundle';
import { SimEngine } from './engine';
import type { JsonObject, RuntimeFrame, SimEvent, SimInitParams, SimPackageDescriptor } from '../types';

export type RunnerCommand = {
  bundle_base64: string;
  bundle_hash: string;
  bundle_format: 'zip' | 'tar';
  entry: string;
  init_params: SimInitParams;
  seed: number;
} & (
  | { op: 'init' }
  | { op: 'apply'; events: SimEvent[]; next: { type: 'tick' } | { type: 'user'; event_type: string; payload: JsonObject; target?: string } }
  | { op: 'restore'; events: SimEvent[] }
  | { op: 'verify'; frame_count: number }
);

type RunnerResponse =
  | { ok: true; descriptor: SimPackageDescriptor; snapshot: ReturnType<typeof toSnapshot> }
  | { ok: true; descriptor: SimPackageDescriptor; determinism: 'passed' | 'failed'; frames: ReturnType<typeof toSnapshot>[]; detail?: string }
  | { ok: false; error: string };

/** 能力限制必须在装配前安装，因为导入会执行包的顶层代码。 */
export async function runCommand(command: RunnerCommand): Promise<RunnerResponse> {
  try {
    installContainerGuards();
    if (command.op === 'verify') return await runVerify(command);

    const engine = await createEngine(command);
    switch (command.op) {
      case 'init':
        break;
      case 'restore':
        engine.restore(command.events);
        break;
      case 'apply':
        engine.restore(command.events);
        if (command.next.type === 'tick') engine.step();
        else engine.inject(command.next.event_type, command.next.payload, command.next.target);
        break;
      default:
        return { ok: false, error: '未知仿真命令' };
    }
    return { ok: true, descriptor: engine.describe(), snapshot: toSnapshot(engine.snapshot()) };
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
}

async function runVerify(command: Extract<RunnerCommand, { op: 'verify' }>): Promise<RunnerResponse> {
  const count = Math.max(1, Math.min(32, Math.trunc(command.frame_count) || 1));
  const first = await collectFrames(command, count);
  const second = await collectFrames(command, count);
  if (JSON.stringify(first) === JSON.stringify(second)) {
    return { ok: true, ...first, determinism: 'passed' };
  }
  return {
    ok: true, ...first, determinism: 'failed',
    detail: '同一随机种子的两次预览产生了不同的包声明或教学画面。',
  };
}

async function collectFrames(command: RunnerCommand, count: number) {
  const engine = await createEngine(command);
  const descriptor = engine.describe();
  const frames = [toSnapshot(engine.snapshot())];
  for (let index = 1; index < count; index += 1) {
    engine.step();
    frames.push(toSnapshot(engine.snapshot()));
  }
  return { descriptor, frames };
}

async function createEngine(command: RunnerCommand): Promise<SimEngine> {
  const simPackage = await loadPackageFromBundle({
    bundleBase64: command.bundle_base64,
    bundleHash: command.bundle_hash,
    format: command.bundle_format,
    entry: command.entry,
  });
  return new SimEngine(simPackage, command.init_params, command.seed);
}

function toSnapshot(snapshot: RuntimeFrame) {
  return {
    tick: snapshot.tick,
    state: snapshot.state,
    view: snapshot.view,
    current_step: snapshot.currentStep,
    interaction_availability: snapshot.interactionAvailability,
    checkpoint_results: snapshot.checkpointResults,
  };
}
