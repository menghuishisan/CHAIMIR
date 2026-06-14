// 本文件实现仿真可视化沉浸式工作台,统一承载教学步骤、模式渲染、交互控制、时间调节、代码追踪与检查点状态。

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, CheckCircle2, Pause, Play, RotateCcw, SkipBack, StepForward } from 'lucide-react';
import { Button } from '@chaimir/ui';
import type { FieldDef, InteractionDef, JsonObject, JsonValue, NarrativeStep, SimEvent, SimInitParams, SimPackage, SimState } from '../types';
import { NarrativeController } from '../runtime/NarrativeController';
import { SimEngine } from '../runtime/SimEngine';
import { PatternRenderer } from '../renderers/PatternRenderer';
import './SimulationWorkbench.css';

export interface SimulationWorkbenchProps<TState extends SimState = SimState> {
  simPackage: SimPackage<TState>;
  initParams: SimInitParams;
  seed: number;
  onActionLog?: (event: SimEvent) => void;
  onCheckpoint?: (checkpointId: string, result: unknown) => void;
}

const speedOptions = [
  { label: '0.5x', multiplier: 0.5 },
  { label: '1x', multiplier: 1 },
  { label: '1.5x', multiplier: 1.5 },
  { label: '2x', multiplier: 2 },
];

/**
 * 渲染完整仿真工作台,并把确定性引擎的状态变化同步到教学叙事、可视化舞台和右侧状态面板。
 */
export function SimulationWorkbench<TState extends SimState = SimState>({
  simPackage,
  initParams,
  seed,
  onActionLog,
  onCheckpoint,
}: SimulationWorkbenchProps<TState>): React.ReactElement {
  const engineRef = useRef<SimEngine<TState> | null>(null);
  const narrative = useMemo(() => new NarrativeController(simPackage), [simPackage]);
  const [state, setState] = useState<TState>(() => simPackage.initState(initParams, seed));
  const [tick, setTick] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [stepDuration, setStepDuration] = useState(state.explanation.defaultDurationMs);
  const [selectedElementId, setSelectedElementId] = useState<string | undefined>(state.selectedElementId);
  const [selectedElementType, setSelectedElementType] = useState<string | undefined>();
  const currentStep = narrative.currentStep(state);
  const view = simPackage.render(state);
  const activeElementId = state.selectedElementId ?? selectedElementId;

  useEffect(() => {
    const engine = new SimEngine<TState>({
      simPackage,
      initParams,
      seed,
      stepDurationMs: stepDuration / speed,
      onStateChange: (nextState, nextTick) => {
        setState(nextState);
        setTick(nextTick);
      },
      onEvent: (event) => onActionLog?.(event),
    });
    engineRef.current = engine;
    setState(engine.snapshot().state);
    setTick(0);
    setPlaying(false);
    setSelectedElementId(undefined);
    setSelectedElementType(undefined);
    return () => engine.destroy();
  }, [simPackage, initParams, seed, onActionLog]);

  useEffect(() => {
    if (state.selectedElementId) {
      setSelectedElementId(state.selectedElementId);
    }
  }, [state.selectedElementId]);

  useEffect(() => {
    engineRef.current?.setStepDuration(stepDuration / speed);
  }, [stepDuration, speed]);

  function togglePlay(): void {
    if (!engineRef.current) return;
    if (playing) {
      engineRef.current.pause();
      setPlaying(false);
      return;
    }
    engineRef.current.start();
    setPlaying(true);
  }

  function stepForward(): void {
    engineRef.current?.step();
  }

  function stepBack(): void {
    engineRef.current?.back();
  }

  function reset(): void {
    engineRef.current?.reset();
    setPlaying(false);
  }

  function submitCheckpoint(step: NarrativeStep): void {
    if (!step.question) return;
    const checkpoint = simPackage.checkpoints?.find((item) => item.id === step.question?.checkpointId);
    if (!checkpoint) return;
    const result = checkpoint.evaluate(state);
    onCheckpoint?.(checkpoint.id, result);
  }

  return (
    <main className="sim-workbench" aria-label={`${simPackage.meta.name}仿真工作台`}>
      <header className="sim-workbench__bar">
        <div>
          <p className="sim-workbench__kicker">仿真可视化引擎</p>
          <h1>{simPackage.meta.name}</h1>
        </div>
        <div className="sim-workbench__status">
          <span>步进 {tick}</span>
          <span>{state.phase}</span>
        </div>
      </header>

      <section className="sim-workbench__layout">
        <aside className="sim-workbench__panel sim-workbench__panel--left">
          <StepList steps={narrative.allSteps()} currentStep={currentStep} />
          <article className="sim-explain">
            <h2>{state.explanation.title}</h2>
            <p>{state.explanation.effect}</p>
            <strong>为什么重要</strong>
            <p>{state.explanation.reason}</p>
          </article>
          {currentStep?.question && (
            <article className="sim-question">
              <h2>{currentStep.question.prompt}</h2>
              <div className="sim-question__options">
                {currentStep.question.options.map((option) => (
                  <button type="button" key={option} onClick={() => submitCheckpoint(currentStep)}>
                    {option}
                  </button>
                ))}
              </div>
            </article>
          )}
        </aside>

        <section className="sim-workbench__stage" aria-label="仿真可视化舞台">
          <p className="sim-workbench__summary">{view.summary}</p>
          <div className="sim-pattern-grid">
            {view.patterns
              .filter((pattern) => pattern.region === 'main')
              .map((pattern) => (
                <PatternRenderer
                  key={pattern.id}
                  pattern={pattern}
                  selectedElementId={activeElementId}
                  onSelectElement={(elementId, elementType) => {
                    setSelectedElementId(elementId);
                    setSelectedElementType(elementType);
                  }}
                />
              ))}
          </div>
          <div className="sim-pattern-grid sim-pattern-grid--compact">
            {view.patterns
              .filter((pattern) => pattern.region !== 'main')
              .map((pattern) => (
                <PatternRenderer
                  key={pattern.id}
                  pattern={pattern}
                  selectedElementId={activeElementId}
                  onSelectElement={(elementId, elementType) => {
                    setSelectedElementId(elementId);
                    setSelectedElementType(elementType);
                  }}
                />
              ))}
          </div>
        </section>

        <aside className="sim-workbench__panel sim-workbench__panel--right">
          <InteractionPanel
            interactions={simPackage.interactions}
            selectedElementId={activeElementId}
            selectedElementType={selectedElementType}
            state={state}
            onEmit={(type, payload, target) => engineRef.current?.inject(type, payload, target)}
          />
          {simPackage.codeTrace && <CodeTracePanel simPackage={simPackage} state={state} />}
          {simPackage.checkpoints?.length ? <CheckpointPanel simPackage={simPackage} state={state} /> : null}
        </aside>
      </section>

      <footer className="sim-workbench__controls">
        <Button variant="on-dark" size="sm" icon={<SkipBack size={16} />} onClick={stepBack} disabled={tick === 0 || playing}>
          回退一步
        </Button>
        <Button variant="primary" size="sm" icon={playing ? <Pause size={16} /> : <Play size={16} />} onClick={togglePlay}>
          {playing ? '暂停' : '播放'}
        </Button>
        <Button variant="on-dark" size="sm" icon={<StepForward size={16} />} onClick={stepForward} disabled={playing}>
          单步推进
        </Button>
        <Button variant="on-dark" size="sm" icon={<RotateCcw size={16} />} onClick={reset}>
          重置
        </Button>

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
      </footer>
    </main>
  );
}

/**
 * 展示叙事步骤列表,帮助学生知道当前处于哪一个教学阶段。
 */
function StepList({ steps, currentStep }: { steps: NarrativeStep[]; currentStep?: NarrativeStep }): React.ReactElement {
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
  selectedElementId,
  selectedElementType,
  state,
  onEmit,
}: {
  interactions: InteractionDef[];
  selectedElementId?: string;
  selectedElementType?: string;
  state: SimState;
  onEmit: (type: string, payload: JsonObject, target?: string) => void;
}): React.ReactElement {
  const [values, setValues] = useState<Record<string, JsonObject>>({});
  const [lastEmittedAt, setLastEmittedAt] = useState<Record<string, number>>({});
  const [, setCooldownVersion] = useState(0);

  function updateValue(interaction: InteractionDef, field: FieldDef, value: JsonValue): void {
    setValues((current) => ({
      ...current,
      [interaction.id]: {
        ...defaultPayload(interaction),
        ...(current[interaction.id] ?? {}),
        [field.name]: value,
      },
    }));
  }

  function handleFieldChange(interaction: InteractionDef, field: FieldDef, value: JsonValue): void {
    updateValue(interaction, field, value);
    if (interaction.kind === 'slider' && field.type === 'range') {
      emitInteraction(interaction, { [field.name]: value });
    }
  }

  function valueFor(interaction: InteractionDef, field: FieldDef): JsonValue {
    return values[interaction.id]?.[field.name] ?? field.default;
  }

  function emitInteraction(interaction: InteractionDef, extra: JsonObject = {}): void {
    const bypassCooldown = extra.active === false || extra.phase === 'end';
    if (!canEmit(interaction, bypassCooldown)) {
      return;
    }
    const target = interaction.target === 'element' || interaction.kind === 'select-element' ? selectedElementId : undefined;
    const payload = { ...defaultPayload(interaction), ...(values[interaction.id] ?? {}), ...extra };
    if (target) {
      payload.target_id = target;
    }
    if (interaction.labelTag === 'attack' && extra.confirmed !== true) {
      const confirmed = window.confirm('该操作会改变当前仿真走势,确认继续吗?');
      if (!confirmed) {
        return;
      }
      payload.confirmed = true;
    }
    onEmit(interaction.emits, payload, target);
    setLastEmittedAt((current) => ({ ...current, [interaction.id]: Date.now() }));
    if (interaction.cooldownMs && interaction.cooldownMs > 0) {
      window.setTimeout(() => setCooldownVersion((version) => version + 1), interaction.cooldownMs);
    }
  }

  function canEmit(interaction: InteractionDef, bypassCooldown = false): boolean {
    if (interaction.availableWhen && !interaction.availableWhen(state)) {
      return false;
    }
    if ((interaction.target === 'element' || interaction.kind === 'select-element') && !selectedElementId) {
      return false;
    }
    if (interaction.elementFilter && selectedElementType && interaction.elementFilter !== selectedElementType) {
      return false;
    }
    if (hasMissingRequiredField(interaction, values[interaction.id])) {
      return false;
    }
    if (bypassCooldown) {
      return true;
    }
    const last = lastEmittedAt[interaction.id];
    return !last || !interaction.cooldownMs || Date.now() - last >= interaction.cooldownMs;
  }

  return (
    <section className="sim-side-section">
      <h2>交互组件</h2>
      <div className="sim-interactions">
        {interactions.map((interaction) => {
          const unavailable = interaction.availableWhen ? !interaction.availableWhen(state) : false;
          const missingTarget = (interaction.target === 'element' || interaction.kind === 'select-element') && !selectedElementId;
          const targetTypeMismatch = Boolean(interaction.elementFilter && selectedElementType && interaction.elementFilter !== selectedElementType);
          const disabled = !canEmit(interaction);
          return (
            <article className={`sim-interaction-card is-${interaction.labelTag ?? 'normal'}`} key={interaction.id}>
              <header>
                <strong>{interaction.label}</strong>
                <small>{interaction.description}</small>
              </header>
              {(interaction.target === 'element' || interaction.kind === 'select-element') && (
                <p className="sim-selected-target">
                  {selectedElementId ? `已选对象 ${selectedElementId}` : '请先在舞台中选择对象'}
                  {targetTypeMismatch ? '，该对象不适用于此操作' : ''}
                </p>
              )}
              {unavailable && <p className="sim-interaction-hint">当前状态暂不可用</p>}
              {missingTarget && <p className="sim-interaction-hint">选择对象后即可操作</p>}
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
 * 从交互字段声明中生成默认载荷,用于按钮类或未展开表单类交互的首个可执行动作。
 */
function defaultPayload(interaction: InteractionDef): JsonObject {
  const payload: JsonObject = {};
  for (const field of interaction.params ?? []) {
    payload[field.name] = field.default;
  }
  return payload;
}

/**
 * 判断必填字段是否缺失,避免把不完整参数注入确定性 reducer。
 */
function hasMissingRequiredField(interaction: InteractionDef, payload?: JsonObject): boolean {
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
  interaction: InteractionDef,
  valueFor: (interaction: InteractionDef, field: FieldDef) => JsonValue,
  updateValue: (interaction: InteractionDef, field: FieldDef, value: JsonValue) => void
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
    return (
      <input
        checked={Boolean(value)}
        onChange={(event) => onChange(event.currentTarget.checked)}
        type="checkbox"
      />
    );
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
        <input
          max={field.max}
          min={field.min}
          onChange={(event) => onChange(Number(event.currentTarget.value))}
          step={field.step ?? 1}
          type="range"
          value={numeric}
        />
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
  interaction: InteractionDef,
  disabled: boolean,
  emitInteraction: (interaction: InteractionDef, extra?: JsonObject) => void,
}): React.ReactElement {
  const dragStartRef = useRef<{ x: number; y: number } | null>(null);
  const holdIntervalRef = useRef<number | null>(null);

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
          emitInteraction(interaction, { phase: 'start', start_x: event.clientX, start_y: event.clientY });
        }}
        onPointerMove={(event) => {
          if (!dragStartRef.current || disabled) {
            return;
          }
          emitInteraction(interaction, {
            phase: 'move',
            start_x: dragStartRef.current.x,
            start_y: dragStartRef.current.y,
            current_x: event.clientX,
            current_y: event.clientY,
            delta_x: event.clientX - dragStartRef.current.x,
            delta_y: event.clientY - dragStartRef.current.y,
          });
        }}
        onPointerUp={(event) => {
          if (!dragStartRef.current) {
            return;
          }
          emitInteraction(interaction, {
            phase: 'end',
            start_x: dragStartRef.current.x,
            start_y: dragStartRef.current.y,
            current_x: event.clientX,
            current_y: event.clientY,
            delta_x: event.clientX - dragStartRef.current.x,
            delta_y: event.clientY - dragStartRef.current.y,
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
function CodeTracePanel<TState extends SimState>({
  simPackage,
  state,
}: {
  simPackage: SimPackage<TState>;
  state: TState;
}): React.ReactElement {
  if (!simPackage.codeTrace) {
    return <></>;
  }
  const codeTrace = simPackage.codeTrace;
  const lines = codeTrace.sourceCode.split('\n');
  const trace = state._trace;
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
 * 计算并展示当前状态下各检查点是否达成,供学生理解目标状态。
 */
function CheckpointPanel<TState extends SimState>({
  simPackage,
  state,
}: {
  simPackage: SimPackage<TState>;
  state: TState;
}): React.ReactElement {
  const checkpoints = simPackage.checkpoints ?? [];
  return (
    <section className="sim-side-section">
      <h2>检查点</h2>
      <div className="sim-checkpoints">
        {checkpoints.map((checkpoint) => {
          const result = checkpoint.evaluate(state);
          return (
            <article className={result.achieved ? 'is-achieved' : 'is-pending'} key={checkpoint.id}>
              {result.achieved ? <CheckCircle2 size={16} /> : <AlertTriangle size={16} />}
              <div>
                <strong>{checkpoint.label}</strong>
                <p>{result.explanation}</p>
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}
