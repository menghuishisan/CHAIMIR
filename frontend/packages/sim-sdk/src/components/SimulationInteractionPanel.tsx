// 本文件实现仿真声明式交互、代码追踪与检查点面板。

import React, { useRef, useState } from 'react';
import { ShieldAlert } from 'lucide-react';
import { FormField, triggerHaptic } from '@chaimir/ui';
import type { FieldDef, InteractionDescriptor, JsonObject, JsonValue } from '../types';
import './SimulationInteractionPanel.css';

/**
 * 根据仿真包声明的通用交互协议渲染可操作控件,不让仿真包自带 UI。
 */
export function InteractionPanel({
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
export function userRuntimeMessage(error: unknown): string {
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
        <FormField className="sim-interaction-field" key={field.name} label={field.label}>{renderFieldControl(field, valueFor(interaction, field), (value) => updateValue(interaction, field, value))}</FormField>
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
