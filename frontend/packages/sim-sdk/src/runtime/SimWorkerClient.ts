// 本文件封装仿真 Worker 通信,主线程只接收纯数据快照,不执行仿真包代码。

import type {
  JsonObject,
  RuntimeSnapshot,
  SimEvent,
  SimInitParams,
  SimPackageDescriptor,
} from '../types';

import type { WorkerRequest, WorkerResponse } from './workerProtocol';

export interface SimWorkerClientOptions {
  /** 只装配平台内置包；扩展包在服务端隔离执行。 */
  builtinCode: string;
  initParams: SimInitParams;
  seed: number;
  commandTimeoutMs: number;
  onReady?: (descriptor: SimPackageDescriptor, snapshot: RuntimeSnapshot) => void;
  onSnapshot?: (snapshot: RuntimeSnapshot, event?: SimEvent) => void;
  onError?: (message: string) => void;
}

interface PendingRequest {
  resolve: (response: WorkerResponse) => void;
  reject: (error: Error) => void;
  timeoutId: ReturnType<typeof setTimeout>;
}

/** Worker 命令以响应确认并受超时约束；播放节奏由消费者按快照调度。 */
export class SimWorkerClient {
  private readonly worker: Worker;
  private readonly options: SimWorkerClientOptions;
  private readonly pending = new Map<number, PendingRequest>();
  private requestId = 1;
  private failed = false;

  constructor(options: SimWorkerClientOptions) {
    this.options = options;
    this.worker = new Worker(new URL('./sim.worker.ts', import.meta.url), { type: 'module' });
    this.worker.onmessage = (event: MessageEvent<WorkerResponse>) => this.handleMessage(event.data);
    this.worker.onerror = (event) => this.failAll(this.userMessage('仿真运行环境异常,请刷新后重试', event.message));
  }

  /** 装配内置包并生成初始快照。 */
  async init(): Promise<void> {
    await this.post({
      type: 'init',
      requestId: 0,
      builtinCode: this.options.builtinCode,
      initParams: this.options.initParams,
      seed: this.options.seed,
    });
  }

  /** 推进一个推演时刻。 */
  async step(): Promise<void> {
    await this.post({ type: 'step', requestId: 0 });
  }

  /** 只返回已执行的用户事件，供调用方按其真实时刻记录。 */
  async inject(eventType: string, payload: JsonObject = {}, target?: string): Promise<SimEvent> {
    const response = await this.post({ type: 'inject', requestId: 0, eventType, payload, target });
    if (response.type !== 'snapshot' || !response.event || response.event.source !== 'user') {
      throw this.failAll('仿真操作未能确认,请重新进入后重试');
    }
    return response.event;
  }

  /** 回退最近一次事件。 */
  async back(): Promise<void> {
    await this.post({ type: 'back', requestId: 0 });
  }

  /** 重置到初始状态。 */
  async reset(): Promise<void> {
    await this.post({ type: 'reset', requestId: 0 });
  }

  /** 终止 Worker 并拒绝尚未完成的命令。 */
  destroy(): void {
    this.failed = true;
    for (const pending of this.pending.values()) {
      clearTimeout(pending.timeoutId);
      pending.reject(new Error('仿真运行环境已关闭'));
    }
    this.pending.clear();
    this.worker.terminate();
  }

  private post(message: WorkerRequest): Promise<WorkerResponse> {
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
          resolve(response);
        },
        reject,
      });
      this.worker.postMessage({ ...message, requestId });
    });
  }

  private handleMessage(response: WorkerResponse): void {
    const pending = this.pending.get(response.requestId);
    if (!pending) return;
    this.pending.delete(response.requestId);
    pending.resolve(response);
    if (response.type === 'ready') {
      this.options.onReady?.(response.descriptor, response.snapshot);
      return;
    }
    if (response.type === 'snapshot') {
      this.options.onSnapshot?.(response.snapshot, response.event);
      return;
    }
  }

  private failAll(message: string): Error {
    this.failed = true;
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

  // 本地执行没有服务端报障编号，内部原因只进入诊断日志。
  private userMessage(message: string, cause?: unknown): string {
    console.error('sim_client_error', {
      operation: 'worker-command',
      reason: message,
      error: { kind: cause instanceof Error ? cause.name : typeof cause },
    });
    return message;
  }
}
