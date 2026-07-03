// 本文件提供前端仿真使用的确定性随机、稳定序列化与轻量哈希工具。
// 所有仿真包必须通过这些纯函数获得可复现行为,禁止依赖真实时间或 Math.random。

import type { DeterministicRandom, JsonValue } from '../types';

const UINT32_MAX = 0xffffffff;

export class XorShiftRandom implements DeterministicRandom {
  private state: number;

  /**
   * constructor 使用给定种子初始化确定性随机状态。
   */
  constructor(seed: number) {
    this.state = seed >>> 0 || 0x9e3779b9;
  }

  /**
   * 生成 0 到 1 之间的确定性伪随机数。
   */
  next(): number {
    let x = this.state;
    x ^= x << 13;
    x ^= x >>> 17;
    x ^= x << 5;
    this.state = x >>> 0;
    return this.state / UINT32_MAX;
  }

  /**
   * 生成闭区间内的确定性整数。
   */
  int(min: number, max: number): number {
    const lower = Math.ceil(min);
    const upper = Math.floor(max);
    return lower + Math.floor(this.next() * (upper - lower + 1));
  }

  /**
   * 从非空列表中确定性选择一个元素。
   */
  pick<T>(items: readonly T[]): T {
    if (items.length === 0) {
      throw new Error('无法从空列表中选择确定性随机项');
    }
    return items[this.int(0, items.length - 1)];
  }
}

/**
 * 将基础种子和命名空间组合成新的确定性种子,用于隔离不同事件上下文。
 */
export function hashSeed(seed: number, namespace: string): number {
  let hash = 2166136261 ^ (seed >>> 0);
  for (let index = 0; index < namespace.length; index += 1) {
    hash ^= namespace.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

/**
 * 对对象进行稳定序列化,保证字段顺序不影响哈希结果。
 */
export function stableStringify(value: JsonValue | unknown): string {
  if (value === null || typeof value !== 'object') {
    return JSON.stringify(value);
  }

  if (Array.isArray(value)) {
    return `[${value.map((item) => stableStringify(item)).join(',')}]`;
  }

  const record = value as Record<string, unknown>;
  return `{${Object.keys(record)
    .sort()
    .map((key) => `${JSON.stringify(key)}:${stableStringify(record[key])}`)
    .join(',')}}`;
}

/**
 * 计算轻量 FNV-1a 十六进制摘要,用于教学仿真的确定性标识和哈希展示。
 */
export function fnv1aHex(input: string, length = 16): string {
  let hash = 2166136261;
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }

  const parts: string[] = [];
  let value = hash >>> 0;
  while (parts.join('').length < length) {
    value = Math.imul(value ^ 0x85ebca6b, 0xc2b2ae35) >>> 0;
    parts.push(value.toString(16).padStart(8, '0'));
  }
  return parts.join('').slice(0, length);
}

/**
 * 基于前缀和语义值生成稳定 ID。
 */
export function deterministicId(prefix: string, value: JsonValue | unknown): string {
  return `${prefix}-${fnv1aHex(stableStringify(value), 12)}`;
}
