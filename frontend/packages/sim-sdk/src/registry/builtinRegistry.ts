// 本文件集中注册 M4 内置仿真包,仅供平台内部装配和教学场景目录使用。

import type { SimCategory } from '../types';
import type { SimPackage } from '../types';
import { builtinSimulations } from '../builtin/catalog';

export const SIM_CATEGORY_ORDER: SimCategory[] = [
  'consensus',
  'cryptography',
  'network',
  'data-structure',
  'contract-security',
  'transaction-runtime',
  'cross-chain-system',
];

export const SIM_CATEGORY_LABELS: Record<SimCategory, string> = {
  consensus: '共识算法',
  cryptography: '密码学',
  network: '网络传播',
  'data-structure': '链上数据结构',
  'contract-security': '合约安全',
  'transaction-runtime': '交易与执行',
  'cross-chain-system': '跨链与系统机制',
};

export interface BuiltinSimulationEntry {
  code: string;
  name: string;
  category: SimCategory;
  version: string;
  summary: string;
}

/**
 * 返回全部内置仿真元数据,列表层不暴露 reducer 等可执行内核。
 */
export function listBuiltinSimulations(): BuiltinSimulationEntry[] {
  return builtinSimulations.map(toEntry);
}

/**
 * 按教学主题分类返回内置仿真,用于仿真实验室筛选和教师选包。
 */
export function listBuiltinSimulationsByCategory(category: SimCategory): BuiltinSimulationEntry[] {
  return listBuiltinSimulations().filter((entry) => entry.category === category);
}

/**
 * 按包 code 查找内置仿真,供平台内部把目录条目装配为 Worker 可运行包。
 */
export function getBuiltinSimulation(code: string): SimPackage | undefined {
  return builtinSimulations.find((simPackage) => simPackage.meta.code === code);
}

/**
 * 将 SimPackage 转为轻量注册条目,避免调用方读取 reducer 等可执行函数作为列表数据。
 */
function toEntry(simPackage: SimPackage): BuiltinSimulationEntry {
  return {
    code: simPackage.meta.code,
    name: simPackage.meta.name,
    category: simPackage.meta.category,
    version: simPackage.meta.version,
    summary: simPackage.meta.summary,
  };
}
