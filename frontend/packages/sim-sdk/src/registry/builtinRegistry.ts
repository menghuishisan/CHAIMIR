// 本文件集中注册 M4 内置仿真包,把目录条目装配为 Worker 可运行包。
// 只允许在 Worker 内被引入:它 import 了全部内置包内核,主线程若引用会把整个 builtin/ 拉进主包。
// 主线程只需判定 code 是否内置时用 ./builtinCode。

import type { SimPackage } from '../types';
import { builtinSimulations } from '../builtin/catalog';
import { BUILTIN_SIM_CODE_PREFIX } from './builtinCode';

const builtinSimulationByCode = indexBuiltinSimulations();

/**
 * 按包 code 查找内置仿真,供 Worker 把会话/分享中的 package_code 装配为可运行包。
 */
export function getBuiltinSimulation(code: string): SimPackage | undefined {
  return builtinSimulationByCode.get(code);
}

/**
 * indexBuiltinSimulations 建立内置包唯一索引。
 * code 必须带内置前缀:主线程按前缀判定能否本地运行(见 ./builtinCode),此处强制两侧口径一致;
 * 缺前缀或重复 code 在装配阶段立即失败,不留到运行期。
 */
function indexBuiltinSimulations(): ReadonlyMap<string, SimPackage> {
  const index = new Map<string, SimPackage>();
  for (const simPackage of builtinSimulations) {
    const code = simPackage.meta.code.trim();
    if (!code.startsWith(BUILTIN_SIM_CODE_PREFIX) || index.has(code)) {
      throw new Error('内置仿真包 code 缺少平台内置前缀或重复');
    }
    index.set(code, simPackage);
  }
  return index;
}
