// 本文件实现仿真可视化沉浸式工作台,主线程只渲染 Worker 返回的纯数据快照。

import React, { useEffect, useRef, useState } from 'react';
import { ArrowLeft, ChevronLeft, ChevronRight, Pause, Play, RotateCcw, SkipBack, StepForward } from 'lucide-react';
import { breakpoints, Button, SandboxStatus, triggerHaptic, useReducedMotion } from '@chaimir/ui';
import type { SandboxStatusKind } from '@chaimir/ui';
import type {
  CheckpointResult,
  JsonObject,
  NarrativeStepDescriptor,
  PatternBinding,
  PlaybackSpeed,
  RuntimeSnapshot,
  SimEvent,
  SimInitParams,
  SimPackageDescriptor,
  SimState,
} from '../types';
import { SimWorkerClient } from '../runtime/SimWorkerClient';
import { PatternRenderer } from '../renderers/PatternRenderer';
import './SimulationWorkbench.css';
import { InteractionPanel, userRuntimeMessage } from './SimulationInteractionPanel';
import { CheckpointPanel, CodeTracePanel } from './SimulationTracePanels';
import { InspectorSection, LearningGoalPanel, MetricPanel, ProtocolIssuePanel, StepList, TimelinePanel } from './SimulationWorkbenchPanels';

export interface SimulationWorkbenchProps {
  moduleUrl?: string;
  builtinCode?: string;
  initParams: SimInitParams;
  seed: number;
  workerCommandTimeoutMs: number;
  initialActions?: SimulationInitialAction[];
  actions?: React.ReactNode;
  computeMode?: 'frontend' | 'backend';
  backendState?: SimulationBackendState;
  onBackendInteraction?: (eventType: string, payload: JsonObject, target?: string) => void;
  onActionLog?: (event: SimEvent) => void;
  onCheckpoint?: (checkpointId: string, result: RuntimeSnapshot['checkpointResults'][string]) => void;
  onExit?: () => void;
  exitLabel?: string;
}

export interface SimulationInitialAction {
  eventType: string;
  payload: JsonObject;
  target?: string;
  atTick?: number;
}

export interface SimulationBackendState {
  tick: number;
  state: SimState;
}

const emptyInitialActions: SimulationInitialAction[] = [];

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
  initialActions = emptyInitialActions,
  actions,
  computeMode = 'frontend',
  backendState,
  onBackendInteraction,
  onActionLog,
  onCheckpoint,
  onExit,
  exitLabel = '返回仿真实验室',
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
  const [inspectorCollapsed, setInspectorCollapsed] = useState(initialInspectorCollapsed);
  const reducedMotion = useReducedMotion();

  useEffect(() => {
    let hydrating = true;
    setDescriptor(undefined);
    setSnapshot(undefined);
    setRuntimeMessage(undefined);
    setPlaying(false);
    setStepDuration(undefined);
    setSelectedElementId(undefined);
    setSelectedElementType(undefined);
    setQuestionAnswers({});
    setInspectorCollapsed(initialInspectorCollapsed());

    const client = new SimWorkerClient({
      moduleUrl,
      builtinCode,
      initParams,
      seed,
      commandTimeoutMs: workerCommandTimeoutMs,
      onReady: (nextDescriptor, nextSnapshot) => {
        setRuntimeMessage(undefined);
        setDescriptor(nextDescriptor);
        setSnapshot(nextSnapshot);
        setStepDuration(nextSnapshot.state.explanation.defaultDurationMs);
      },
      onSnapshot: (nextSnapshot, event) => {
        setRuntimeMessage(undefined);
        setSnapshot(nextSnapshot);
        if (nextSnapshot.state.selectedElementId) {
          setSelectedElementId(nextSnapshot.state.selectedElementId);
        }
        if (!hydrating && event?.source === 'user') {
          onActionLog?.(event);
        }
      },
      onError: (message) => {
        setRuntimeMessage(userRuntimeMessage(message));
        setPlaying(false);
      },
    });
    clientRef.current = client;
    void client.init()
      .then(async () => {
        let replayTick = 0;
        for (const action of initialActions) {
          const actionTick = action.atTick ?? replayTick;
          if (!Number.isSafeInteger(actionTick) || actionTick < replayTick) {
            throw new Error('回放操作顺序不完整，请重新打开仿真。');
          }
          while (replayTick < actionTick) {
            await client.step();
            replayTick += 1;
          }
          await client.inject(action.eventType, action.payload, action.target);
        }
        hydrating = false;
      })
      .catch((error: Error) => {
        setRuntimeMessage(userRuntimeMessage(error));
        setPlaying(false);
      });
    return () => {
      client.destroy();
      clientRef.current = null;
    };
  }, [moduleUrl, builtinCode, initParams, seed, workerCommandTimeoutMs, initialActions, onActionLog]);

  useEffect(() => {
    if (computeMode !== 'backend' || !backendState || !descriptor) {
      return;
    }
    void clientRef.current?.syncState(backendState.tick, backendState.state).catch((error: unknown) => {
      setRuntimeMessage(userRuntimeMessage(error));
      setPlaying(false);
    });
  }, [backendState, computeMode, descriptor]);

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
  const arrangedPatterns = snapshot ? arrangeTeachingFrame(snapshot.view) : { primary: undefined, support: [] };

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
    setRuntimeMessage(undefined);
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
              {exitLabel}
            </Button>
          )}
          <div>
            <p className="sim-workbench__kicker">仿真可视化引擎</p>
            <h1>仿真正在准备</h1>
          </div>
          <div className="sim-workbench__status" />
          {actions && <div className="sim-workbench__actions">{actions}</div>}
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
            {exitLabel}
          </Button>
        )}
        <div className="sim-workbench__title">
          <p className="sim-workbench__kicker">仿真可视化引擎</p>
          <h1>{descriptor.meta.name}</h1>
        </div>
        <div className="sim-workbench__status">
          <span>步进 {snapshot.tick}</span>
          <span>{snapshot.view.phase.title}</span>
          {reducedMotion && <span>减少动态</span>}
        </div>
        {actions && <div className="sim-workbench__actions">{actions}</div>}
      </header>

      <section className="sim-workbench__layout">
        <section className="sim-workbench__stage" aria-label="仿真画面">
          <header className="sim-workbench__stage-head">
            <div>
              <p className="sim-workbench__watch">{snapshot.view.phase.explanation.watch}</p>
            </div>
            {activeElementId && (
              <p className="sim-workbench__selection">
                已选择 {selectedElementType ?? '对象'} <code>{activeElementId}</code>
              </p>
            )}
          </header>
          {runtimeMessage && <p className="sim-runtime-message" role="alert">{runtimeMessage}</p>}
          <div className="sim-pattern-grid sim-pattern-grid--primary">
            {arrangedPatterns.primary ? (
              <PatternRenderer
                key={arrangedPatterns.primary.id}
                pattern={arrangedPatterns.primary}
                focus={snapshot.view.focus}
                selectedElementId={activeElementId}
                reducedMotion={reducedMotion}
                onSelectElement={(elementId, elementType) => {
                  setSelectedElementId(elementId);
                  setSelectedElementType(elementType);
                }}
              />
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
              detail={`当前阶段：${snapshot.view.phase.title}`}
            />
            <article className="sim-explain sim-explain--focus">
              <p className="sim-workbench__stage-kicker">当前阶段</p>
              <h2>{snapshot.view.phase.title}</h2>
              <p>{snapshot.view.phase.explanation.what}</p>
              <strong>为什么重要</strong>
              <p>{snapshot.view.phase.explanation.why}</p>
            </article>
            <InteractionPanel
              interactions={descriptor.interactions}
              availability={snapshot.interactionAvailability}
              selectedElementId={activeElementId}
              selectedElementType={selectedElementType}
              eventSeq={snapshot.events.length}
              onEmit={(type, payload, target) => {
                if (computeMode === 'backend') {
                  onBackendInteraction?.(type, payload, target);
                  return;
                }
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
                      focus={snapshot.view.focus}
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
          <Button variant="on-dark" size="sm" icon={<SkipBack size={16} />} onClick={stepBack} disabled={computeMode === 'backend' || snapshot.tick === 0 || playing}>
            回退一步
          </Button>
          <Button variant="primary" size="sm" icon={playing ? <Pause size={16} /> : <Play size={16} />} onClick={togglePlay} disabled={computeMode === 'backend' || reducedMotion} title={computeMode === 'backend' ? '后端仿真由计算服务推进' : reducedMotion ? '已开启减少动态，请使用单步推进' : undefined}>
            {playing ? '暂停' : '播放'}
          </Button>
          <Button variant="on-dark" size="sm" icon={<StepForward size={16} />} onClick={stepForward} disabled={computeMode === 'backend' || playing}>
            单步推进
          </Button>
          <Button variant="on-dark" size="sm" icon={<RotateCcw size={16} />} onClick={reset} disabled={computeMode === 'backend'}>
            重置
          </Button>
        </div>

        <div className="sim-workbench__control-group" aria-label="节奏控制">
          <label className="sim-control-field" htmlFor="simulation-step-duration">
            <span>步骤时长</span>
            <input
              id="simulation-step-duration"
              type="range"
              min={500}
              max={5000}
              step={100}
              value={stepDuration}
              onChange={(event) => setStepDuration(Number(event.currentTarget.value))}
            />
            <strong>{stepDuration}ms</strong>
          </label>

          <label className="sim-control-field" htmlFor="simulation-playback-speed">
            <span>播放速度</span>
            <select id="simulation-playback-speed" value={speed} onChange={(event) => setSpeed(Number(event.currentTarget.value))}>
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

/** initialInspectorCollapsed 让窄屏先保留完整舞台，桌面则直接展示教学说明。 */
function initialInspectorCollapsed(): boolean {
  return typeof window !== 'undefined' && window.matchMedia(`(max-width: ${breakpoints.lg - 1}px)`).matches;
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
 * arrangeTeachingFrame 严格按教学画面声明组织主舞台和辅助证据,不从模式顺序推断布局。
 */
function arrangeTeachingFrame(view: RuntimeSnapshot['view']): { primary?: PatternBinding; support: PatternBinding[] } {
  const byId = new Map(view.patterns.map((pattern) => [pattern.id, pattern]));
  const primary = byId.get(view.layout.primary);
  const supportIds = [
    ...(view.layout.evidence ?? []),
    ...(view.layout.timeline ? [view.layout.timeline] : []),
    ...(view.layout.metrics ?? []),
    ...(view.layout.trace ? [view.layout.trace] : []),
    ...(view.layout.checkpoints ?? []),
  ];
  const seen = new Set<string>(primary ? [primary.id] : []);
  const support = supportIds
    .map((id) => byId.get(id))
    .filter((pattern): pattern is PatternBinding => Boolean(pattern))
    .filter((pattern) => {
      if (seen.has(pattern.id)) {
        return false;
      }
      seen.add(pattern.id);
      return true;
    });
  return {
    primary,
    support,
  };
}
