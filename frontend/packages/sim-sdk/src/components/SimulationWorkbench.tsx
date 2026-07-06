// 本文件实现仿真可视化沉浸式工作台,主线程只渲染 Worker 返回的纯数据快照。

import React, { useEffect, useRef, useState } from 'react';
import { AlertTriangle, ArrowLeft, CheckCircle2, ChevronLeft, ChevronRight, Clock3, Pause, Play, RotateCcw, ShieldAlert, SkipBack, StepForward } from 'lucide-react';
import { Button, SandboxStatus, triggerHaptic } from '@chaimir/ui';
import type { SandboxStatusKind } from '@chaimir/ui';
import type {
  CheckpointResult,
  CodeTraceDef,
  FieldDef,
  InteractionDescriptor,
  JsonObject,
  JsonValue,
  NarrativeStepDescriptor,
  PatternBinding,
  PlaybackSpeed,
  RuntimeSnapshot,
  SimEvent,
  SimInitParams,
  SimPackageDescriptor,
} from '../types';
import { SimWorkerClient } from '../runtime/SimWorkerClient';
import { PatternRenderer } from '../renderers/PatternRenderer';
import { usePrefersReducedMotion } from '../hooks/usePrefersReducedMotion';
import './SimulationWorkbench.css';

export interface SimulationWorkbenchProps {
  moduleUrl?: string;
  builtinCode?: string;
  initParams: SimInitParams;
  seed: number;
  workerCommandTimeoutMs: number;
  onActionLog?: (event: SimEvent) => void;
  onCheckpoint?: (checkpointId: string, result: RuntimeSnapshot['checkpointResults'][string]) => void;
  onExit?: () => void;
}

const speedOptions: PlaybackSpeed[] = [
  { label: '0.5x', multiplier: 0.5 },
  { label: '1x', multiplier: 1 },
  { label: '1.5x', multiplier: 1.5 },
  { label: '2x', multiplier: 2 },
];

/**
 * 渲染完整仿真工作台,并通过 Worker 隔离执行仿真包的状态机、叙事触发与检查点判定。
 */
export function SimulationWorkbench({
  moduleUrl,
  builtinCode,
  initParams,
  seed,
  workerCommandTimeoutMs,
  onActionLog,
  onCheckpoint,
  onExit,
}: SimulationWorkbenchProps): React.ReactElement {
  const clientRef = useRef<SimWorkerClient | null>(null);
  const [descriptor, setDescriptor] = useState<SimPackageDescriptor | undefined>();
  const [snapshot, setSnapshot] = useState<RuntimeSnapshot | undefined>();
  const [runtimeMessage, setRuntimeMessage] = useState<string | undefined>();
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [stepDuration, setStepDuration] = useState<number | undefined>();
  const [selectedElementId, setSelectedElementId] = useState<string | undefined>();
  const [selectedElementType, setSelectedElementType] = useState<string | undefined>();
  const [questionAnswers, setQuestionAnswers] = useState<Record<string, string>>({});
  const [inspectorCollapsed, setInspectorCollapsed] = useState(true);
  const reducedMotion = usePrefersReducedMotion();

  useEffect(() => {
    setDescriptor(undefined);
    setSnapshot(undefined);
    setRuntimeMessage(undefined);
    setPlaying(false);
    setStepDuration(undefined);
    setSelectedElementId(undefined);
    setSelectedElementType(undefined);
    setQuestionAnswers({});
    setInspectorCollapsed(true);

    const client = new SimWorkerClient({
      moduleUrl,
      builtinCode,
      initParams,
      seed,
      commandTimeoutMs: workerCommandTimeoutMs,
      onReady: (nextDescriptor, nextSnapshot) => {
        setDescriptor(nextDescriptor);
        setSnapshot(nextSnapshot);
        setStepDuration(nextSnapshot.state.explanation.defaultDurationMs);
      },
      onSnapshot: (nextSnapshot, event) => {
        setSnapshot(nextSnapshot);
        if (nextSnapshot.state.selectedElementId) {
          setSelectedElementId(nextSnapshot.state.selectedElementId);
        }
        if (event) {
          onActionLog?.(event);
        }
      },
      onError: (message) => {
        setRuntimeMessage(userRuntimeMessage(message));
        setPlaying(false);
      },
    });
    clientRef.current = client;
    void client.init().catch((error: Error) => {
      setRuntimeMessage(userRuntimeMessage(error));
      setPlaying(false);
    });
    return () => {
      client.destroy();
      clientRef.current = null;
    };
  }, [moduleUrl, builtinCode, initParams, seed, workerCommandTimeoutMs, onActionLog]);

  useEffect(() => {
    if (stepDuration === undefined) {
      return;
    }
    clientRef.current?.setStepDuration(stepDuration / speed);
  }, [stepDuration, speed]);

  useEffect(() => {
    if (!reducedMotion || !playing) {
      return;
    }
    clientRef.current?.pause();
    setPlaying(false);
  }, [playing, reducedMotion]);

  const activeElementId = snapshot?.state.selectedElementId ?? selectedElementId;
  const currentStep = snapshot?.currentStep;
  const arrangedPatterns = snapshot ? splitViewPatterns(snapshot.view.patterns) : { main: [], support: [] };

  /**
   * handleRuntimeError 把命令错误转为工作台可见提示并停止播放。
   */
  function handleRuntimeError(error: unknown): void {
    setRuntimeMessage(userRuntimeMessage(error));
    setPlaying(false);
  }

  /**
   * togglePlay 在自动播放和暂停之间切换,不让未初始化工作台发送命令。
   */
  function togglePlay(): void {
    triggerHaptic();
    const client = clientRef.current;
    if (!client || !snapshot) {
      return;
    }
    if (playing) {
      client.pause();
      setPlaying(false);
      return;
    }
    if (reducedMotion) {
      client.pause();
      setPlaying(false);
      return;
    }
    client.start();
    setPlaying(true);
  }

  /**
   * stepForward 手动推进一个事件周期。
   */
  function stepForward(): void {
    triggerHaptic();
    void clientRef.current?.step().catch(handleRuntimeError);
  }

  /**
   * stepBack 回退最近一次事件并由 Worker 重放状态。
   */
  function stepBack(): void {
    triggerHaptic();
    void clientRef.current?.back().catch(handleRuntimeError);
  }

  /**
   * reset 重置 Worker 状态和本地选择状态。
   */
  function reset(): void {
    triggerHaptic();
    void clientRef.current?.reset().catch(handleRuntimeError);
    setPlaying(false);
    setSelectedElementId(undefined);
    setSelectedElementType(undefined);
    setQuestionAnswers({});
  }

  /**
   * submitCheckpoint 把学习者的实际选择转为检查点结果,避免答案按钮只提交 Worker 既有状态。
   */
  function submitCheckpoint(step: NarrativeStepDescriptor, selectedAnswer: string): void {
    triggerHaptic();
    const question = step.question;
    if (!question) {
      return;
    }
    const achieved = selectedAnswer === question.answer;
    const result: CheckpointResult = {
      achieved,
      answer: {
        selected: selectedAnswer,
        expected: question.answer,
        checkpointId: question.checkpointId,
        stateCheckpoint: snapshot?.checkpointResults[question.checkpointId]?.answer ?? null,
      },
      explanation: achieved ? '选择正确,已记录本次设问结果。' : '当前选择不正确,请结合仿真状态重新判断。',
    };
    setQuestionAnswers((current) => ({ ...current, [question.checkpointId]: selectedAnswer }));
    onCheckpoint?.(question.checkpointId, result);
  }

  if (!descriptor || !snapshot || stepDuration === undefined) {
    return (
      <main className="sim-workbench" aria-label="仿真工作台">
        <header className="sim-workbench__bar">
          {onExit && (
            <Button variant="on-dark" size="sm" icon={<ArrowLeft size={16} />} onClick={onExit}>
              返回仿真实验室
            </Button>
          )}
          <div>
            <p className="sim-workbench__kicker">仿真可视化引擎</p>
            <h1>仿真正在准备</h1>
          </div>
          <div className="sim-workbench__status" />
        </header>
        <section className="sim-workbench__empty">
          {runtimeMessage ? <p>{runtimeMessage}</p> : <p>正在加载仿真环境,请稍候</p>}
        </section>
      </main>
    );
  }

  return (
    <main className={`sim-workbench ${inspectorCollapsed ? 'is-inspector-collapsed' : ''}`} aria-label={`${descriptor.meta.name}仿真工作台`}>
      <header className="sim-workbench__bar">
        {onExit && (
          <Button variant="on-dark" size="sm" icon={<ArrowLeft size={16} />} onClick={onExit}>
            返回仿真实验室
          </Button>
        )}
        <div className="sim-workbench__title">
          <p className="sim-workbench__kicker">仿真可视化引擎</p>
          <h1>{descriptor.meta.name}</h1>
        </div>
        <div className="sim-workbench__status">
          <span>步进 {snapshot.tick}</span>
          <span>{snapshot.state.phase}</span>
          {reducedMotion && <span>减少动态</span>}
        </div>
      </header>

      <section className="sim-workbench__layout">
        <section className="sim-workbench__stage" aria-label="仿真画面">
          <header className="sim-workbench__stage-head">
            <div>
              <p className="sim-workbench__summary">{snapshot.view.summary}</p>
            </div>
            {activeElementId && (
              <p className="sim-workbench__selection">
                已选择 {selectedElementType ?? '对象'} <code>{activeElementId}</code>
              </p>
            )}
          </header>
          <div className="sim-pattern-grid sim-pattern-grid--primary">
            {arrangedPatterns.main.length > 0 ? (
              arrangedPatterns.main.map((pattern) => (
                <PatternRenderer
                  key={pattern.id}
                  pattern={pattern}
                  selectedElementId={activeElementId}
                  reducedMotion={reducedMotion}
                  onSelectElement={(elementId, elementType) => {
                    setSelectedElementId(elementId);
                    setSelectedElementType(elementType);
                  }}
                />
              ))
            ) : (
              <ProtocolIssuePanel />
            )}
          </div>
        </section>

        <button
          aria-expanded={!inspectorCollapsed}
          className="sim-workbench__drawer-toggle"
          onClick={() => setInspectorCollapsed((c) => !c)}
          aria-label={inspectorCollapsed ? '展开说明' : '收起说明'}
          type="button"
        >
          {inspectorCollapsed ? <ChevronLeft size={20} /> : <ChevronRight size={20} />}
        </button>

        {!inspectorCollapsed && (
          <aside className="sim-workbench__panel sim-workbench__panel--rail" aria-label="当前阶段和操作">
            <SandboxStatus
              status={sandboxStatusFromSnapshot(snapshot, runtimeMessage)}
              detail={`当前阶段：${snapshot.state.phase}`}
            />
            <article className="sim-explain sim-explain--focus">
              <p className="sim-workbench__stage-kicker">当前阶段</p>
              <h2>{snapshot.state.explanation.title}</h2>
              <p>{snapshot.state.explanation.effect}</p>
              <strong>为什么重要</strong>
              <p>{snapshot.state.explanation.reason}</p>
            </article>
            <InteractionPanel
              interactions={descriptor.interactions}
              availability={snapshot.interactionAvailability}
              selectedElementId={activeElementId}
              selectedElementType={selectedElementType}
              eventSeq={snapshot.events.length}
              onEmit={(type, payload, target) => {
                void clientRef.current?.inject(type, payload, target).catch(handleRuntimeError);
              }}
            />
            {arrangedPatterns.support.length > 0 && (
              <InspectorSection title="过程记录" summary={`${arrangedPatterns.support.length} 组记录`}>
                <div className="sim-pattern-grid sim-pattern-grid--support" aria-label="过程记录">
                  {arrangedPatterns.support.map((pattern) => (
                    <PatternRenderer
                      key={pattern.id}
                      pattern={pattern}
                      selectedElementId={activeElementId}
                      reducedMotion={reducedMotion}
                      onSelectElement={(elementId, elementType) => {
                        setSelectedElementId(elementId);
                        setSelectedElementType(elementType);
                      }}
                    />
                  ))}
                </div>
              </InspectorSection>
            )}
            {currentStep?.question && (
              <InspectorSection title="设问检查点" summary="预测当前结果">
                <article className="sim-question">
                  <h2>{currentStep.question.prompt}</h2>
                  <div className="sim-question__options">
                    {currentStep.question.options.map((option) => {
                      const question = currentStep.question;
                      if (!question) {
                        return null;
                      }
                      const selected = questionAnswers[question.checkpointId] === option;
                      return (
                        <button aria-pressed={selected} className={selected ? 'is-selected' : undefined} key={option} type="button" onClick={() => submitCheckpoint(currentStep, option)}>
                          {option}
                        </button>
                      );
                    })}
                  </div>
                </article>
              </InspectorSection>
            )}
            {runtimeMessage && <p className="sim-runtime-message" role="alert">{runtimeMessage}</p>}
            {Object.keys(snapshot.state.metrics).length > 0 && (
              <InspectorSection title="状态指标" summary={`${Object.keys(snapshot.state.metrics).length} 项`}>
                <MetricPanel metrics={snapshot.state.metrics} />
              </InspectorSection>
            )}
            {descriptor.codeTrace && (
              <InspectorSection title="代码追踪" summary="代码行与变量">
                <CodeTracePanel codeTrace={descriptor.codeTrace} snapshot={snapshot} />
              </InspectorSection>
            )}
            {descriptor.checkpoints.length ? (
              <InspectorSection title="检查点结果" summary={`${descriptor.checkpoints.length} 个检查点`}>
                <CheckpointPanel descriptor={descriptor} snapshot={snapshot} />
              </InspectorSection>
            ) : null}
            <InspectorSection title="学习目标" summary={`${descriptor.meta.learningObjectives.length} 项目标`}>
              <LearningGoalPanel descriptor={descriptor} />
            </InspectorSection>
            <InspectorSection title="教学步骤" summary={`${descriptor.narrative.length} 个阶段`}>
              <StepList steps={descriptor.narrative} currentStep={currentStep} />
            </InspectorSection>
          </aside>
        )}
      </section>

      <footer className="sim-workbench__controls">
        <TimelinePanel descriptor={descriptor} snapshot={snapshot} stepDuration={stepDuration} speed={speed} />
        <div className="sim-workbench__control-group" aria-label="播放控制">
          <Button variant="on-dark" size="sm" icon={<SkipBack size={16} />} onClick={stepBack} disabled={snapshot.tick === 0 || playing}>
            回退一步
          </Button>
          <Button variant="primary" size="sm" icon={playing ? <Pause size={16} /> : <Play size={16} />} onClick={togglePlay} disabled={reducedMotion} title={reducedMotion ? '已开启减少动态，请使用单步推进' : undefined}>
            {playing ? '暂停' : '播放'}
          </Button>
          <Button variant="on-dark" size="sm" icon={<StepForward size={16} />} onClick={stepForward} disabled={playing}>
            单步推进
          </Button>
          <Button variant="on-dark" size="sm" icon={<RotateCcw size={16} />} onClick={reset}>
            重置
          </Button>
        </div>

        <div className="sim-workbench__control-group" aria-label="节奏控制">
          <label className="sim-control-field">
            <span>步骤时长</span>
            <input
              type="range"
              min={500}
              max={5000}
              step={100}
              value={stepDuration}
              onChange={(event) => setStepDuration(Number(event.currentTarget.value))}
            />
            <strong>{stepDuration}ms</strong>
          </label>

          <label className="sim-control-field">
            <span>播放速度</span>
            <select value={speed} onChange={(event) => setSpeed(Number(event.currentTarget.value))}>
              {speedOptions.map((option) => (
                <option key={option.label} value={option.multiplier}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>
        </div>
      </footer>
    </main>
  );
}

/**
 * sandboxStatusFromSnapshot 把仿真阶段映射到统一沙箱状态机组件,避免各工作台自建状态外观。
 */
function sandboxStatusFromSnapshot(snapshot: RuntimeSnapshot, runtimeMessage?: string): SandboxStatusKind {
  const phase = snapshot.state.phase.toLowerCase();
  if (runtimeMessage) return 'failed';
  if (/fail|error|异常|失败/.test(phase)) return 'failed';
  if (/compile|build|编译|构建/.test(phase)) return 'compiling';
  if (/mine|mining|pow|挖矿|出块/.test(phase)) return 'mining';
  if (/seal|commit|final|封块|提交|最终/.test(phase)) return 'sealing';
  return 'ready';
}

/**
 * splitViewPatterns 严格按仿真包声明的 region 组织主画面和补充视图,不替仿真包猜测主视图。
 */
function splitViewPatterns(patterns: PatternBinding[]): { main: PatternBinding[]; support: PatternBinding[] } {
  return {
    main: patterns.filter((pattern) => pattern.region === 'main'),
    support: patterns.filter((pattern) => pattern.region !== 'main'),
  };
}

/**
 * ProtocolIssuePanel 在仿真包未声明主画面时给出可恢复的用户向提示,避免静默错画。
 */
function ProtocolIssuePanel(): React.ReactElement {
  return (
    <section className="sim-protocol-issue" role="alert">
      <AlertTriangle size={22} />
      <div>
        <h2>暂时无法展示仿真画面</h2>
        <p>这个仿真包缺少主要画面配置。请联系管理员检查仿真包配置后再试。</p>
      </div>
    </section>
  );
}

/**
 * InspectorSection 提供右侧说明区的原生折叠分组,让补充信息按需展开且键盘可达。
 */
function InspectorSection({
  title,
  summary,
  defaultOpen = false,
  children,
}: {
  title: string;
  summary: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}): React.ReactElement {
  return (
    <details className="sim-inspector-section" open={defaultOpen}>
      <summary>
        <span>{title}</span>
        <small>{summary}</small>
      </summary>
      <div className="sim-inspector-section__body">{children}</div>
    </details>
  );
}

/**
 * LearningGoalPanel 展示当前仿真包自己的教学目标,避免所有工作台只呈现统一壳层。
 */
function LearningGoalPanel({ descriptor }: { descriptor: SimPackageDescriptor }): React.ReactElement {
  return (
    <section className="sim-side-section sim-learning">
      <h2>学习目标</h2>
      <p>{descriptor.meta.summary}</p>
      <ul>
        {descriptor.meta.learningObjectives.map((objective) => (
          <li key={objective}>{objective}</li>
        ))}
      </ul>
    </section>
  );
}

/**
 * MetricPanel 把仿真状态中的关键指标显式展示,让不同机制的运行差异可扫描。
 */
function MetricPanel({ metrics }: { metrics: RuntimeSnapshot['state']['metrics'] }): React.ReactElement | null {
  const entries = Object.entries(metrics).slice(0, 12);
  if (!entries.length) {
    return null;
  }
  return (
    <section className="sim-side-section sim-metrics">
      <h2>状态指标</h2>
      <dl>
        {entries.map(([key, value]) => (
          <div key={key}>
            <dt>{metricLabel(key)}</dt>
            <dd>{String(value)}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

/**
 * metricLabel 将常见内部指标名转成用户向文案,未知指标只做安全格式化。
 */
function metricLabel(key: string): string {
  const labels: Record<string, string> = {
    accountNonce: '账户 Nonce',
    activeStake: '活跃权益',
    attackerProfit: '攻击收益',
    attestedStake: '见证权益',
    challenge: '挑战值',
    commitIndex: '提交位置',
    coverage: '覆盖率',
    difficulty: '难度',
    dirty: '变更账户',
    entries: '条目数',
    failedCases: '失败用例',
    finalizedEpoch: '最终周期',
    progress: '进度',
    height: '高度',
    hops: '跳数',
    invalidCount: '异常数量',
    leaves: '叶子数量',
    latency: '延迟',
    nonce: 'Nonce',
    pathLength: '路径长度',
    quorum: '法定人数',
    result: '结果',
    risk: '风险',
    round: '轮次',
    shortlistSize: '候选列表',
    term: '任期',
    throughput: '吞吐',
    ts: '时间',
    validShares: '有效份额',
    validSignatures: '有效签名',
    vaultBalance: '金库余额',
    versionGap: '版本差距',
    view: '视图',
    votes: '投票数',
    work: '工作量',
    finalized: '已最终确认',
    committed: '已提交',
    failed: '失败次数',
    gasLeft: '剩余 Gas',
    gasUsed: '已用 Gas',
    balance: '余额',
    confirmations: '确认数',
  };
  if (labels[key]) {
    return labels[key];
  }
  return '指标';
}

/**
 * TimelinePanel 展示所有仿真共享的逻辑时间、事件进度和播放参数。
 */
function TimelinePanel({
  descriptor,
  snapshot,
  stepDuration,
  speed,
}: {
  descriptor: SimPackageDescriptor;
  snapshot: RuntimeSnapshot;
  stepDuration: number;
  speed: number;
}): React.ReactElement {
  const maxTick = Math.max(1, descriptor.meta.scaleLimit.maxTick);
  const progress = Math.min(100, Math.round((snapshot.tick / maxTick) * 100));
  const latestEvent = snapshot.events[snapshot.events.length - 1];
  return (
    <section className="sim-timeline" aria-label="仿真时间轴">
      <header>
        <Clock3 size={16} />
        <strong>时间轴</strong>
        <span>{progress}%</span>
      </header>
      <div className="sim-timeline__track" aria-hidden="true">
        <span style={{ transform: `scaleX(${progress / 100})` }} />
      </div>
      <dl>
        <div>
          <dt>当前步进</dt>
          <dd>
            {snapshot.tick}/{maxTick}
          </dd>
        </div>
        <div>
          <dt>当前阶段</dt>
          <dd>{snapshot.state.phase}</dd>
        </div>
        <div>
          <dt>事件数量</dt>
          <dd>
            {snapshot.events.length}/{descriptor.meta.scaleLimit.maxEvents}
          </dd>
        </div>
        <div>
          <dt>最近事件</dt>
          <dd>{latestEvent ? latestEvent.type : '等待开始'}</dd>
        </div>
        <div>
          <dt>播放参数</dt>
          <dd>
            {stepDuration}ms / {speed}x
          </dd>
        </div>
      </dl>
    </section>
  );
}

/**
 * 展示叙事步骤列表,帮助学生知道当前处于哪一个教学阶段。
 */
function StepList({ steps, currentStep }: { steps: NarrativeStepDescriptor[]; currentStep?: NarrativeStepDescriptor }): React.ReactElement {
  return (
    <ol className="sim-step-list" aria-label="教学步骤">
      {steps.map((step) => (
        <li className={step.id === currentStep?.id ? 'is-current' : undefined} key={step.id}>
          <span>{step.title}</span>
          <small>{step.defaultDurationMs}ms</small>
        </li>
      ))}
    </ol>
  );
}

/**
 * 根据仿真包声明的通用交互协议渲染可操作控件,不让仿真包自带 UI。
 */
function InteractionPanel({
  interactions,
  availability,
  selectedElementId,
  selectedElementType,
  eventSeq,
  onEmit,
}: {
  interactions: InteractionDescriptor[];
  availability: Record<string, boolean>;
  selectedElementId?: string;
  selectedElementType?: string;
  eventSeq: number;
  onEmit: (type: string, payload: JsonObject, target?: string) => void;
}): React.ReactElement {
  const [values, setValues] = useState<Record<string, JsonObject>>({});
  const [lastEmittedAt, setLastEmittedAt] = useState<Record<string, number>>({});
  const [pendingAttackId, setPendingAttackId] = useState<string | undefined>();

  /**
   * updateValue 保存交互字段当前值,作为下一次注入事件的载荷。
   */
  function updateValue(interaction: InteractionDescriptor, field: FieldDef, value: JsonValue): void {
    setValues((current) => ({
      ...current,
      [interaction.id]: {
        ...defaultPayload(interaction),
        ...(current[interaction.id] ?? {}),
        [field.name]: value,
      },
    }));
  }

  /**
   * handleFieldChange 处理字段变化,滑块类交互会立即注入参数变更。
   */
  function handleFieldChange(interaction: InteractionDescriptor, field: FieldDef, value: JsonValue): void {
    updateValue(interaction, field, value);
    if (interaction.kind === 'slider' && field.type === 'range') {
      emitInteraction(interaction, { [field.name]: value });
    }
  }

  /**
   * valueFor 读取字段当前值,没有本地值时回退到声明的默认值。
   */
  function valueFor(interaction: InteractionDescriptor, field: FieldDef): JsonValue {
    return values[interaction.id]?.[field.name] ?? field.default;
  }

  /**
   * emitInteraction 统一执行冷却、目标、确认和 payload 合成后再发给 Worker。
   */
  function emitInteraction(interaction: InteractionDescriptor, extra: JsonObject = {}): void {
    const payloadExtra: JsonObject = { ...extra };
    const confirmed = payloadExtra.confirmed === true;
    delete payloadExtra.confirmed;
    const bypassCooldown = payloadExtra.active === false || payloadExtra.phase === 'end';
    if (!canEmit(interaction, bypassCooldown)) {
      return;
    }
    const target = interaction.target === 'element' || interaction.kind === 'select-element' ? selectedElementId : undefined;
    const payload = { ...defaultPayload(interaction), ...(values[interaction.id] ?? {}), ...payloadExtra };
    if (interaction.labelTag === 'attack' && !confirmed) {
      triggerHaptic(50);
      setPendingAttackId(interaction.id);
      return;
    }
    triggerHaptic();
    onEmit(interaction.emits, payload, target);
    setPendingAttackId(undefined);
    setLastEmittedAt((current) => ({ ...current, [interaction.id]: eventSeq }));
  }

  /**
   * canEmit 判断交互在当前状态下是否可执行。
   */
  function canEmit(interaction: InteractionDescriptor, bypassCooldown = false): boolean {
    if (availability[interaction.id] === false) {
      return false;
    }
    if ((interaction.target === 'element' || interaction.kind === 'select-element') && !selectedElementId) {
      return false;
    }
    if (interaction.elementFilter && interaction.elementFilter !== selectedElementType) {
      return false;
    }
    if (hasMissingRequiredField(interaction, values[interaction.id])) {
      return false;
    }
    if (bypassCooldown) {
      return true;
    }
    const cooldownEvents = cooldownEventWindow(interaction.cooldownMs);
    const last = lastEmittedAt[interaction.id];
    return last === undefined || cooldownEvents === 0 || eventSeq - last >= cooldownEvents;
  }

  return (
    <section className="sim-side-section">
      <h2>可用操作</h2>
      <div className="sim-interactions">
        {interactions.map((interaction) => {
          const unavailable = availability[interaction.id] === false;
          const missingTarget = (interaction.target === 'element' || interaction.kind === 'select-element') && !selectedElementId;
          const targetTypeMismatch = Boolean(interaction.elementFilter && interaction.elementFilter !== selectedElementType);
          const disabled = !canEmit(interaction);
          return (
            <article className={`sim-interaction-card is-${interaction.labelTag ?? 'normal'}`} key={interaction.id}>
              <header>
                <strong>{interaction.label}</strong>
                <small>{interaction.description}</small>
              </header>
              {(interaction.target === 'element' || interaction.kind === 'select-element') && (
                <p className="sim-selected-target">
                  {selectedElementId ? `已选对象 ${selectedElementId}` : '请先选择一个对象'}
                  {targetTypeMismatch ? '，该对象不适用于此操作' : ''}
                </p>
              )}
              {unavailable && <p className="sim-interaction-hint">当前状态暂不可用</p>}
              {missingTarget && <p className="sim-interaction-hint">选择对象后即可操作</p>}
              {pendingAttackId === interaction.id && (
                <div className="sim-attack-confirm" role="group" aria-label="高影响操作确认">
                  <ShieldAlert size={16} />
                  <span>此操作会改变当前仿真走势。</span>
                  <button type="button" onClick={() => emitInteraction(interaction, { confirmed: true })}>
                    确认执行
                  </button>
                  <button type="button" onClick={() => setPendingAttackId(undefined)}>
                    取消
                  </button>
                </div>
              )}
              {renderInteractionFields(interaction, valueFor, handleFieldChange)}
              <InteractionCommand interaction={interaction} disabled={disabled} emitInteraction={emitInteraction} />
            </article>
          );
        })}
      </div>
    </section>
  );
}

/**
 * 将声明式 cooldown_ms 转为事件窗口,避免运行时依赖真实时间影响确定性。
 */
function cooldownEventWindow(cooldownMs?: number): number {
  if (!cooldownMs || cooldownMs <= 0) {
    return 0;
  }
  return Math.max(1, Math.ceil(cooldownMs / 1000));
}

/**
 * userRuntimeMessage 只展示用户可理解的仿真错误,避免泄漏 worker 或仿真包内部细节。
 */
function userRuntimeMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : typeof error === 'string' ? error : '';
  return message && /[\u4e00-\u9fa5]/.test(message) ? message : '仿真运行失败，请刷新后重试';
}

/**
 * 从交互字段声明中生成默认载荷,用于按钮类或未展开表单类交互的首个可执行动作。
 */
function defaultPayload(interaction: InteractionDescriptor): JsonObject {
  const payload: JsonObject = {};
  for (const field of interaction.params ?? []) {
    payload[field.name] = field.default;
  }
  return payload;
}

/**
 * 判断必填字段是否缺失,避免把不完整参数注入确定性 reducer。
 */
function hasMissingRequiredField(interaction: InteractionDescriptor, payload?: JsonObject): boolean {
  return Boolean(
    interaction.params?.some((field) => {
      if (!field.required) {
        return false;
      }
      const value = payload?.[field.name] ?? field.default;
      return value === null || value === undefined || value === '';
    })
  );
}

/**
 * 渲染交互字段,字段值只作为即将注入 reducer 的事件载荷。
 */
function renderInteractionFields(
  interaction: InteractionDescriptor,
  valueFor: (interaction: InteractionDescriptor, field: FieldDef) => JsonValue,
  updateValue: (interaction: InteractionDescriptor, field: FieldDef, value: JsonValue) => void
): React.ReactElement | null {
  if (!interaction.params?.length) {
    return null;
  }
  return (
    <div className="sim-interaction-fields">
      {interaction.params.map((field) => (
        <label className="sim-interaction-field" key={field.name}>
          <span>{field.label}</span>
          {renderFieldControl(field, valueFor(interaction, field), (value) => updateValue(interaction, field, value))}
        </label>
      ))}
    </div>
  );
}

/**
 * 按字段类型输出基础控件,保持所有仿真使用同一套输入规则。
 */
function renderFieldControl(field: FieldDef, value: JsonValue, onChange: (value: JsonValue) => void): React.ReactElement {
  if (field.type === 'boolean') {
    return <input checked={Boolean(value)} onChange={(event) => onChange(event.currentTarget.checked)} type="checkbox" />;
  }
  if (field.type === 'select') {
    return (
      <select value={selectedOptionIndex(field, value)} onChange={(event) => onChange(field.options?.[Number(event.currentTarget.value)]?.value ?? field.default)}>
        {(field.options ?? []).map((option, index) => (
          <option key={`${field.name}-${index}`} value={index}>
            {option.label}
          </option>
        ))}
      </select>
    );
  }
  if (field.type === 'range') {
    const numeric = typeof value === 'number' ? value : Number(field.default ?? 0);
    return (
      <span className="sim-range-control">
        <input max={field.max} min={field.min} onChange={(event) => onChange(Number(event.currentTarget.value))} step={field.step ?? 1} type="range" value={numeric} />
        <strong>{numeric}</strong>
      </span>
    );
  }
  if (field.type === 'number') {
    return (
      <input
        max={field.max}
        min={field.min}
        onChange={(event) => onChange(Number(event.currentTarget.value))}
        step={field.step ?? 1}
        type="number"
        value={typeof value === 'number' ? value : Number(field.default ?? 0)}
      />
    );
  }
  return <input onChange={(event) => onChange(event.currentTarget.value)} type="text" value={String(value ?? '')} />;
}

/**
 * 渲染不同交互类型的提交命令,让所有仿真复用同一套通用交互行为。
 */
function InteractionCommand({
  interaction,
  disabled,
  emitInteraction,
}: {
  interaction: InteractionDescriptor;
  disabled: boolean;
  emitInteraction: (interaction: InteractionDescriptor, extra?: JsonObject) => void;
}): React.ReactElement {
  const dragStartRef = useRef<{ x: number; y: number } | null>(null);
  const holdIntervalRef = useRef<number | null>(null);

  /**
   * stopHold 停止按住类交互并补发结束载荷。
   */
  function stopHold(): void {
    if (holdIntervalRef.current !== null) {
      window.clearInterval(holdIntervalRef.current);
      holdIntervalRef.current = null;
    }
    emitInteraction(interaction, { active: false });
  }

  if (interaction.kind === 'hold') {
    return (
      <button
        className="sim-interaction-command"
        disabled={disabled}
        onPointerCancel={stopHold}
        onPointerDown={() => {
          emitInteraction(interaction, { active: true });
          holdIntervalRef.current = window.setInterval(() => emitInteraction(interaction, { active: true }), interaction.cooldownMs ?? 200);
        }}
        onPointerLeave={stopHold}
        onPointerUp={stopHold}
        type="button"
      >
        按住执行
      </button>
    );
  }
  if (interaction.kind === 'drag') {
    return (
      <button
        className="sim-interaction-command sim-interaction-command--drag"
        disabled={disabled}
        onPointerDown={(event) => {
          dragStartRef.current = { x: event.clientX, y: event.clientY };
          event.currentTarget.setPointerCapture(event.pointerId);
          emitInteraction(interaction, { phase: 'start', startX: event.clientX, startY: event.clientY });
        }}
        onPointerMove={(event) => {
          if (!dragStartRef.current || disabled) {
            return;
          }
          emitInteraction(interaction, {
            phase: 'move',
            startX: dragStartRef.current.x,
            startY: dragStartRef.current.y,
            currentX: event.clientX,
            currentY: event.clientY,
            deltaX: event.clientX - dragStartRef.current.x,
            deltaY: event.clientY - dragStartRef.current.y,
          });
        }}
        onPointerUp={(event) => {
          if (!dragStartRef.current) {
            return;
          }
          emitInteraction(interaction, {
            phase: 'end',
            startX: dragStartRef.current.x,
            startY: dragStartRef.current.y,
            currentX: event.clientX,
            currentY: event.clientY,
            deltaX: event.clientX - dragStartRef.current.x,
            deltaY: event.clientY - dragStartRef.current.y,
          });
          dragStartRef.current = null;
        }}
        type="button"
      >
        拖动执行
      </button>
    );
  }
  const label = interaction.kind === 'slider' ? '应用当前参数' : interaction.kind === 'select-element' ? '应用到对象' : '执行操作';
  return (
    <button className="sim-interaction-command" disabled={disabled} onClick={() => emitInteraction(interaction)} type="button">
      {label}
    </button>
  );
}

/**
 * 查找 select 字段当前值对应的选项下标。
 */
function selectedOptionIndex(field: FieldDef, value: JsonValue): number {
  const index = (field.options ?? []).findIndex((option) => Object.is(option.value, value));
  return Math.max(0, index);
}

/**
 * 展示仿真状态到源码行与变量的映射,用于建立“现象到代码”的教学联系。
 */
function CodeTracePanel({ codeTrace, snapshot }: { codeTrace: CodeTraceDef; snapshot: RuntimeSnapshot }): React.ReactElement {
  const lines = codeTrace.sourceCode.split('\n');
  const trace = snapshot.state._trace;
  const active = new Set(trace?.triggeredLines ?? []);
  return (
    <section className="sim-side-section">
      <h2>代码追踪</h2>
      <div className="sim-code">
        {lines.map((line, index) => {
          const lineNo = index + 1;
          const mapping = codeTrace.lineMapping.find((item) => item.line === lineNo);
          return (
            <div className={`sim-code__line ${active.has(lineNo) ? `is-${mapping?.highlightStyle ?? 'normal'}` : ''}`} key={lineNo}>
              <span>{lineNo}</span>
              <code>{line || ' '}</code>
              {active.has(lineNo) && mapping?.annotation && <em>{mapping.annotation}</em>}
            </div>
          );
        })}
      </div>
      <dl className="sim-watch">
        {(codeTrace.variableWatch ?? []).map((watch) => (
          <div key={watch.name}>
            <dt>{watch.name}</dt>
            <dd>{String(trace?.variables?.[watch.name] ?? '')}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

/**
 * 展示 Worker 计算出的检查点状态,主线程不执行检查点判定函数。
 */
function CheckpointPanel({ descriptor, snapshot }: { descriptor: SimPackageDescriptor; snapshot: RuntimeSnapshot }): React.ReactElement {
  return (
    <section className="sim-side-section">
      <h2>检查点</h2>
      <div className="sim-checkpoints">
        {descriptor.checkpoints.map((checkpoint) => {
          const result = snapshot.checkpointResults[checkpoint.id];
          return (
            <article className={result?.achieved ? 'is-achieved' : 'is-pending'} key={checkpoint.id}>
              {result?.achieved ? <CheckCircle2 size={16} /> : <AlertTriangle size={16} />}
              <div>
                <strong>{checkpoint.label}</strong>
                <p>{result?.explanation ?? '等待仿真达到检查条件'}</p>
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}
