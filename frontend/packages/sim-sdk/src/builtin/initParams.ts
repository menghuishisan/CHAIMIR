// 本文件提供内置仿真包共享的初始化参数读取规则,统一处理范围限制和阶段注入值解包。

import type { JsonObject, JsonValue, SimInitParams, StageInjectedParam } from '../types';

/**
 * paramValue 从初始化参数中读取原始值,并兼容 M7 阶段编排注入的 {_source,_value} 包装。
 */
export function paramValue(params: SimInitParams, name: string): JsonValue | undefined {
  const value = params[name];
  if (isStageInjectedParam(value)) {
    return value._value;
  }
  return value;
}

/**
 * numberParam 读取数值参数并限制到算法声明的安全范围内。
 */
export function numberParam(params: SimInitParams, name: string, fallback: number, min: number, max: number): number {
  const value = paramValue(params, name);
  const numeric = typeof value === 'number' && Number.isFinite(value) ? value : fallback;
  return Math.min(max, Math.max(min, numeric));
}

/**
 * integerParam 读取整数参数,用于节点数、序号、叶子下标等离散配置。
 */
export function integerParam(params: SimInitParams, name: string, fallback: number, min: number, max: number): number {
  return Math.round(numberParam(params, name, fallback, min, max));
}

/**
 * stringParam 读取面向教学场景的短文本参数,避免空字符串和超长内容进入状态机。
 */
export function stringParam(params: SimInitParams, name: string, fallback: string, maxLength = 96): string {
  const value = paramValue(params, name);
  if (typeof value !== 'string') {
    return fallback;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed.slice(0, maxLength) : fallback;
}

/**
 * stringArrayParam 读取字符串数组参数,去掉空项并限制数量和单项长度。
 */
export function stringArrayParam(params: SimInitParams, name: string, fallback: string[], minCount: number, maxCount: number, maxItemLength = 64): string[] {
  const value = paramValue(params, name);
  const source = Array.isArray(value) ? value : fallback;
  const normalized = source.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean).map((item) => item.slice(0, maxItemLength));
  const bounded = normalized.slice(0, maxCount);
  return bounded.length >= minCount ? bounded : fallback.slice(0, Math.max(minCount, Math.min(maxCount, fallback.length)));
}

/**
 * integerArrayParam 读取整数数组参数,用于金额、序号等不能归一化的离散值。
 */
export function integerArrayParam(params: SimInitParams, name: string, fallback: number[], minCount: number, maxCount: number, minValue: number, maxValue: number): number[] {
  const value = paramValue(params, name);
  const source = Array.isArray(value) ? value : fallback;
  const fallbackValue = fallback.find((item) => Number.isFinite(item)) ?? minValue;
  const normalized = source
    .filter((item): item is number => typeof item === 'number' && Number.isFinite(item))
    .map((item) => Math.round(Math.min(maxValue, Math.max(minValue, item))))
    .slice(0, maxCount);
  if (normalized.length >= minCount) {
    return normalized;
  }
  return Array.from({ length: Math.min(maxCount, minCount) }, (_, index) => Math.round(Math.min(maxValue, Math.max(minValue, fallback[index % Math.max(1, fallback.length)] ?? fallbackValue))));
}

/**
 * weightedShares 读取权重数组并归一到指定总量,用于算力和权益类仿真。
 */
export function weightedShares(params: SimInitParams, name: string, fallback: number[], count: number, total = 100): number[] {
  const value = paramValue(params, name);
  const source = Array.isArray(value) ? value : fallback;
  const raw = Array.from({ length: count }, (_, index) => {
    const item = source[index];
    return typeof item === 'number' && Number.isFinite(item) && item > 0 ? item : fallback[index % fallback.length] ?? 1;
  });
  const sum = raw.reduce((acc, item) => acc + item, 0) || 1;
  const rounded = raw.map((item) => Math.max(1, Math.round((item / sum) * total)));
  const drift = total - rounded.reduce((acc, item) => acc + item, 0);
  rounded[0] += drift;
  return rounded;
}

/**
 * indexFromSeed 把任意 seed 映射成稳定数组下标,避免各算法重复处理取模边界。
 */
export function indexFromSeed(seed: number, count: number): number {
  if (count <= 0) {
    return 0;
  }
  return Math.abs(Math.trunc(seed)) % count;
}

/**
 * isStageInjectedParam 判断值是否是阶段编排注入参数,避免算法内重复写结构判断。
 */
function isStageInjectedParam(value: JsonValue | undefined): value is StageInjectedParam & JsonObject {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value) && '_value' in value);
}
