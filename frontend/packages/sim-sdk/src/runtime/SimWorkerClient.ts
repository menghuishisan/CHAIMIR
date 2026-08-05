// 本文件封装仿真 Worker 通信,主线程只接收纯数据快照,不执行仿真包代码。

import type {
  JsonObject,
  RuntimeSnapshot,
  SimEvent,
  SimInitParams,
  SimPackageDescriptor,
  SimState,
} from '../types';

type WorkerRequest =
  | { type: 'init'; requestId: number; builtinCode: string; initParams: SimInitParams; seed: number }
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
  /** 平台内置包 code(带 builtin__ 前缀);扩展包运行路径见 docs/总-前端设计规范.md §9,未定契约前不接受 */
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

/**
 * SimWorkerClient 负责创建隔离 Worker 并发送带超时保护的命令。
 *
 * 播放节奏不在此处:自动推进必须等上一步的快照回到主线程后再排下一步,慢帧才不会让命令堆积,
 * 而 client 无从得知快照何时到达。故节奏、速度档与调度统一由消费端的播放控制持有
 * (见 apps/web 的 features/sim/playback.ts),client 只提供单次命令。
 */
export class SimWorkerClient {
  private readonly worker: Worker;
  private readonly options: SimWorkerClientOptions;
  private readonly pending = new Map<number, PendingRequest>();
  private requestId = 1;
  private failed = false;

  /**
   * constructor 创建模块 Worker 并绑定主线程消息处理器。
   */
  constructor(options: SimWorkerClientOptions) {
    this.options = options;
    this.worker = new Worker(new URL('./sim.worker.ts', import.meta.url), { type: 'module' });
    this.worker.onmessage = (event: MessageEvent<WorkerResponse>) => this.handleMessage(event.data);
    this.worker.onerror = (event) => this.failAll(this.userMessage('仿真运行环境异常,请刷新后重试', event.message));
  }

  /**
   * init 在 Worker 内装配内置仿真包并生成初始快照。
   */
  async init(): Promise<void> {
    await this.post({
      type: 'init',
      requestId: 0,
      builtinCode: this.options.builtinCode,
      initParams: this.options.initParams,
      seed: this.options.seed,
    });
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
    await this.post({ type: 'reset', requestId: 0 });
  }

  /**
   * destroy 终止 Worker。
   */
  destroy(): void {
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
      this.worker.postMessage({ ...message, requestId });
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
   * userMessage 记录内部错误原因并返回纯用户向文案。
   * 仿真在浏览器本地运行,没有后端签发的报障编号;前端不自造编号——它在运维侧查不到,
   * 展示出来只会把用户挡在报障流程之外,所以技术原因只进控制台结构化日志。
   */
  private userMessage(message: string, cause?: unknown): string {
    console.error('sim_client_error', {
      operation: 'worker-command',
      reason: message,
      error: cause instanceof Error ? cause.message : String(cause ?? ''),
    });
    return message;
  }
}
