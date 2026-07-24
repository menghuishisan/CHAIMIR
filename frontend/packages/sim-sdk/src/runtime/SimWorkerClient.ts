// 本文件封装仿真 Worker 通信,主线程只接收纯数据快照,不执行仿真包代码。

import type {
  JsonObject,
  RuntimeSnapshot,
  SimEvent,
  SimInitParams,
  SimPackageDescriptor,
  SimState,
} from '../types';
import { fnv1aHex } from './deterministic';

type WorkerRequest =
  | { type: 'init'; requestId: number; moduleUrl?: string; builtinCode?: string; initParams: SimInitParams; seed: number }
  | { type: 'step'; requestId: number }
  | { type: 'inject'; requestId: number; eventType: string; payload: JsonObject; target?: string }
  | { type: 'sync-state'; requestId: number; tick: number; state: SimState }
  | { type: 'back'; requestId: number }
  | { type: 'reset'; requestId: number };

type WorkerResponse =
  | { type: 'ready'; requestId: number; descriptor: SimPackageDescriptor; snapshot: RuntimeSnapshot }
  | { type: 'snapshot'; requestId: number; snapshot: RuntimeSnapshot; event?: SimEvent }
  | { type: 'error'; requestId: number; message: string };

export interface SimWorkerClientOptions {
  moduleUrl?: string;
  builtinCode?: string;
  initParams: SimInitParams;
  seed: number;
  commandTimeoutMs: number;
  stepDurationMs?: number;
  onReady?: (descriptor: SimPackageDescriptor, snapshot: RuntimeSnapshot) => void;
  onSnapshot?: (snapshot: RuntimeSnapshot, event?: SimEvent) => void;
  onError?: (message: string) => void;
}

interface PendingRequest {
  resolve: (response: WorkerResponse) => void;
  reject: (error: Error) => void;
  timeoutId: ReturnType<typeof setTimeout>;
}

/**
 * SimWorkerClient 负责创建隔离 Worker、发送命令并维护自动播放计时器。
 */
export class SimWorkerClient {
  private readonly worker: Worker;
  private readonly options: SimWorkerClientOptions;
  private readonly pending = new Map<number, PendingRequest>();
  private requestId = 1;
  private intervalId: ReturnType<typeof setInterval> | undefined;
  private stepDurationMs: number;
  private failed = false;

  /**
   * constructor 创建模块 Worker 并绑定主线程消息处理器。
   */
  constructor(options: SimWorkerClientOptions) {
    this.options = options;
    this.stepDurationMs = options.stepDurationMs ?? 1200;
    this.worker = new Worker(new URL('./sim.worker.ts', import.meta.url), { type: 'module' });
    this.worker.onmessage = (event: MessageEvent<WorkerResponse>) => this.handleMessage(event.data);
    this.worker.onerror = (event) => this.failAll(this.userMessage('仿真运行环境异常,请刷新后重试', event.message));
  }

  /**
   * init 加载仿真包模块并生成初始快照。
   */
  async init(): Promise<void> {
    await this.post({
      type: 'init',
      requestId: 0,
      moduleUrl: this.options.moduleUrl,
      builtinCode: this.options.builtinCode,
      initParams: this.options.initParams,
      seed: this.options.seed,
    });
  }

  /**
   * start 按当前步长自动发送 tick,自动播放错误会显式进入失败路径。
   */
  start(): void {
    if (this.intervalId) {
      return;
    }
    this.intervalId = setInterval(() => {
      void this.step().catch((error: unknown) => {
        this.failAll(this.userMessage('仿真播放中断,请刷新后重试', error));
      });
    }, this.stepDurationMs);
  }

  /**
   * pause 暂停自动 tick。
   */
  pause(): void {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = undefined;
    }
  }

  /**
   * setStepDuration 更新自动播放间隔。
   */
  setStepDuration(durationMs: number): void {
    this.stepDurationMs = Math.max(250, Math.min(durationMs, 8000));
    if (this.intervalId) {
      this.pause();
      this.start();
    }
  }

  /**
   * step 推进一个 tick。
   */
  async step(): Promise<void> {
    await this.post({ type: 'step', requestId: 0 });
  }

  /**
   * inject 注入用户交互事件。
   */
  async inject(eventType: string, payload: JsonObject = {}, target?: string): Promise<void> {
    await this.post({ type: 'inject', requestId: 0, eventType, payload, target });
  }

  /**
   * syncState 把受信任后端适配器返回的模型状态交给 Worker 生成统一渲染快照。
   */
  async syncState(tick: number, state: SimState): Promise<void> {
    await this.post({ type: 'sync-state', requestId: 0, tick, state });
  }

  /**
   * back 回退最近一次事件。
   */
  async back(): Promise<void> {
    await this.post({ type: 'back', requestId: 0 });
  }

  /**
   * reset 重置到初始状态。
   */
  async reset(): Promise<void> {
    this.pause();
    await this.post({ type: 'reset', requestId: 0 });
  }

  /**
   * destroy 终止 Worker。
   */
  destroy(): void {
    this.pause();
    for (const pending of this.pending.values()) {
      clearTimeout(pending.timeoutId);
    }
    this.pending.clear();
    this.worker.terminate();
  }

  /**
   * post 发送带超时保护的 Worker 命令,并把响应关联回调用方。
   */
  private post(message: WorkerRequest): Promise<void> {
    if (this.failed) {
      return Promise.reject(new Error(this.userMessage('仿真运行环境异常,请刷新后重试')));
    }
    const requestId = this.requestId++;
    return new Promise((resolve, reject) => {
      const timeoutId = setTimeout(() => {
        this.pending.delete(requestId);
        reject(this.failAll(this.userMessage('仿真运行超时,请刷新后重试')));
      }, this.options.commandTimeoutMs);
      this.pending.set(requestId, {
        timeoutId,
        resolve: (response) => {
          clearTimeout(timeoutId);
          if (response.type === 'error') {
            const error = new Error(response.message);
            this.options.onError?.(response.message);
            reject(error);
            return;
          }
          resolve();
        },
        reject,
      });
      try {
        this.worker.postMessage({ ...message, requestId });
      } catch (error) {
        this.pending.delete(requestId);
        clearTimeout(timeoutId);
        const runtimeError = this.failAll(this.userMessage('仿真命令发送失败,请刷新后重试', error));
        reject(runtimeError);
      }
    });
  }

  /**
   * handleMessage 处理 Worker 响应并触发 ready、snapshot 或 error 回调。
   */
  private handleMessage(response: WorkerResponse): void {
    const pending = this.pending.get(response.requestId);
    if (pending) {
      this.pending.delete(response.requestId);
      pending.resolve(response);
      if (response.type === 'error') {
        return;
      }
    }
    if (response.type === 'ready') {
      this.options.onReady?.(response.descriptor, response.snapshot);
      return;
    }
    if (response.type === 'snapshot') {
      this.options.onSnapshot?.(response.snapshot, response.event);
      return;
    }
    if (response.type === 'error' && this.failed) {
      return;
    }
    this.failAll(response.message);
  }

  /**
   * failAll 进入失败态,终止 Worker 并拒绝所有等待中的命令。
   */
  private failAll(message: string): Error {
    this.failed = true;
    this.pause();
    this.worker.terminate();
    const error = new Error(message);
    for (const [requestId, pending] of this.pending.entries()) {
      clearTimeout(pending.timeoutId);
      this.pending.delete(requestId);
      pending.reject(error);
    }
    this.options.onError?.(message);
    return error;
  }

  /**
   * userMessage 生成带追踪编号的用户向错误文案,内部错误只进结构化控制台日志。
   */
  private userMessage(message: string, cause?: unknown): string {
    const traceId = fnv1aHex(`sim-client:${message}:${cause instanceof Error ? cause.message : String(cause ?? '')}`, 12);
    console.error('sim_client_error', {
      trace_id: traceId,
      tenant_id: 'frontend-local',
      operation: 'worker-command',
      error: cause instanceof Error ? cause.message : String(cause ?? ''),
    });
    return `${message}。如需帮助,请提供编号 ${traceId}`;
  }
}
