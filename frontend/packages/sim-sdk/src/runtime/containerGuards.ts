// 本文件在装配扩展仿真包之前封锁容器内的宿主访问能力。
//
// 与浏览器 Worker 的 installWorkerGuards 同职责、同时序要求:必须在 import 包代码之前完成封锁,
// 因为包的顶层代码在 import 那一刻就会执行,晚一步等于没封。
// 随机与时间这两项两个宿主口径必须完全一致,故由共享的 installDeterminismGuards 承担(见 guards.ts);
// 本文件只声明"容器这一侧还要封哪些宿主入口"。
//
// 容器本身已是隔离边界(deny-all 网络、只读根、无凭据、无 ServiceAccount Token),
// 这一层封锁的主要目的是确定性,其次是让越界调用早失败并给出原因,而不是在容器边界静默出错。

import { installDeterminismGuards, sealGlobal } from './guards';

/** blockedGlobals 是仿真包不得触碰的容器宿主入口;容器已无网络,封锁同时保证确定性与早失败。 */
const blockedGlobals = [
  'fetch',
  'XMLHttpRequest',
  'WebSocket',
  'EventSource',
  'Worker',
  'SharedWorker',
  'BroadcastChannel',
  'indexedDB',
  'caches',
  'localStorage',
  'sessionStorage',
  'process',
  'require',
] as const;

let installed = false;

/**
 * installContainerGuards 冻结随机与时间来源并抹掉宿主访问入口;重复调用是幂等的。
 */
export function installContainerGuards(): void {
  if (installed) {
    return;
  }
  installed = true;

  installDeterminismGuards(globalThis);
  for (const name of blockedGlobals) {
    sealGlobal(globalThis, name, undefined);
  }
}
