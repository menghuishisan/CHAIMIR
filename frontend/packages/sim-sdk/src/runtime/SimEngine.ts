// 本文件实现前端确定性状态机,负责步进、交互事件、回放与单步回退。
// 引擎只处理纯状态转移,不直接操作 DOM 或访问外部网络。

import type { JsonObject, ReducerContext, SimEvent, SimInitParams, SimPackage, SimState } from '../types';
import { hashSeed, XorShiftRandom } from './deterministic';

export interface SimEngineOptions<TState extends SimState> {
  simPackage: SimPackage<TState>;
  initParams: SimInitParams;
  seed: number;
  stepDurationMs?: number;
  onStateChange?: (state: TState, tick: number) => void;
  onEvent?: (event: SimEvent) => void;
}

export interface SimSnapshot<TState extends SimState> {
  state: TState;
  tick: number;
  events: SimEvent[];
}

export class SimEngine<TState extends SimState = SimState> {
  private readonly simPackage: SimPackage<TState>;
  private readonly initParams: SimInitParams;
  private readonly seed: number;
  private readonly onStateChange?: (state: TState, tick: number) => void;
  private readonly onEvent?: (event: SimEvent) => void;
  private state: TState;
  private tick = 0;
  private seq = 1;
  private events: SimEvent[] = [];
  private intervalId: ReturnType<typeof setInterval> | undefined;
  private stepDurationMs: number;

  constructor(options: SimEngineOptions<TState>) {
    this.simPackage = options.simPackage;
    this.initParams = options.initParams;
    this.seed = options.seed;
    this.stepDurationMs = options.stepDurationMs ?? 1200;
    this.onStateChange = options.onStateChange;
    this.onEvent = options.onEvent;
    this.state = this.simPackage.initState(options.initParams, options.seed);
  }

  /**
   * 按当前步骤时长启动自动步进。
   */
  start(): void {
    if (this.intervalId) return;
    this.intervalId = setInterval(() => this.step(), this.stepDurationMs);
  }

  /**
   * 暂停自动步进,保留当前状态与操作序列。
   */
  pause(): void {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = undefined;
    }
  }

  /**
   * 更新单步时长,播放中会立即按新节奏重启计时器。
   */
  setStepDuration(durationMs: number): void {
    this.stepDurationMs = Math.max(250, Math.min(durationMs, 8000));
    if (this.intervalId) {
      this.pause();
      this.start();
    }
  }

  /**
   * 推进一个确定性 tick 事件。
   */
  step(): void {
    this.applyEvent({ type: 'tick', source: 'tick', payload: {}, target: undefined });
  }

  /**
   * 注入用户交互事件,由仿真包 reducer 统一消费。
   */
  inject(type: string, payload: JsonObject = {}, target?: string): void {
    this.applyEvent({ type, source: 'user', payload, target });
  }

  /**
   * 回退最近一次事件,通过重新播放操作序列复现上一状态。
   */
  back(): void {
    if (this.tick === 0 && this.events.length === 0) return;
    const previousEvents = this.events.slice(0, -1);
    this.replay(previousEvents);
  }

  /**
   * 清空事件并回到初始状态。
   */
  reset(): void {
    this.pause();
    this.tick = 0;
    this.seq = 1;
    this.events = [];
    this.state = this.simPackage.initState(this.initParams, this.seed);
    this.onStateChange?.(this.state, this.tick);
  }

  /**
   * 从初始参数与种子开始重放指定事件序列。
   */
  replay(events: SimEvent[]): void {
    this.pause();
    this.tick = 0;
    this.seq = 1;
    this.events = [];
    this.state = this.simPackage.initState(this.initParams, this.seed);
    for (const event of events) {
      this.reduceRecordedEvent(event);
    }
    this.onStateChange?.(this.state, this.tick);
  }

  /**
   * 返回当前状态、步进值和事件序列快照,供 UI 或上报层读取。
   */
  snapshot(): SimSnapshot<TState> {
    return {
      state: this.state,
      tick: this.tick,
      events: [...this.events],
    };
  }

  /**
   * 释放计时器资源。
   */
  destroy(): void {
    this.pause();
  }

  /**
   * 将输入事件补齐序号和步进上下文后交给 reducer,并同步记录事件。
   */
  private applyEvent(eventInput: Omit<SimEvent, 'seq' | 'atTick'>, notify = true): void {
    const event: SimEvent = {
      ...eventInput,
      atTick: this.tick,
      seq: this.seq,
    };
    const context: ReducerContext = {
      seed: this.seed,
      tick: this.tick,
      seq: this.seq,
      random: new XorShiftRandom(hashSeed(this.seed, `${this.simPackage.meta.code}:${this.tick}:${this.seq}`)),
    };

    this.state = this.simPackage.reducer(this.state, event, context);
    this.seq += 1;
    if (event.source === 'tick') {
      this.tick += 1;
    }
    this.events.push(event);
    if (notify) {
      this.onEvent?.(event);
      this.onStateChange?.(this.state, this.tick);
    }
  }

  /**
   * 重放已有事件时保留原始序号和发生步进,确保分享剧本与上报序列可复现。
   */
  private reduceRecordedEvent(event: SimEvent): void {
    const context: ReducerContext = {
      seed: this.seed,
      tick: event.atTick,
      seq: event.seq,
      random: new XorShiftRandom(hashSeed(this.seed, `${this.simPackage.meta.code}:${event.atTick}:${event.seq}`)),
    };

    this.state = this.simPackage.reducer(this.state, event, context);
    this.tick = event.source === 'tick' ? event.atTick + 1 : event.atTick;
    this.seq = event.seq + 1;
    this.events.push(event);
  }
}
