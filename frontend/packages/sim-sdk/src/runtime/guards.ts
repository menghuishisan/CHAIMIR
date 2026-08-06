// 本文件是两个执行宿主共用的能力封锁原语:浏览器 Worker 与后端隔离容器同一口径。
//
// 为什么共用:两个宿主都必须在装配仿真包**之前**冻结随机与时间来源,否则同一个 seed
// 在两处跑出两条过程,回放、分享与判分随之失效(见 docs/04-仿真可视化引擎/02-架构设计.md §3)。
// 封锁逻辑各写一份的直接后果就是口径漂移 —— 例如只封 `Date.now` 而漏掉 `new Date()`,
// 那一处仍然读得到真实时钟。故属性封锁与时间/随机策略收敛到这里,
// 各宿主只保留自己那份"该封哪些宿主入口"的清单(浏览器有 importScripts,容器有 process)。

/** blockedCapability 是被封锁能力被调用时抛出的统一原因。 */
export function blockedCapability(): never {
  throw new Error('仿真包能力不被允许');
}

/**
 * sealGlobal 以不可写方式覆盖全局属性;不可重定义的属性直接失败,不静默跳过。
 * 静默跳过会让"已封锁"变成假象,而每一项都关系到确定性或隔离。
 */
export function sealGlobal(scope: object, key: string, value: unknown): void {
  const descriptor = findPropertyDescriptor(scope, key);
  if (descriptor && descriptor.configurable === false) {
    throw new Error(`仿真运行环境无法封锁必要能力: ${key}`);
  }
  Object.defineProperty(scope, key, { value, configurable: true, writable: false });
}

/**
 * installDeterminismGuards 冻结两个宿主共有的非确定性来源:随机数与真实时间。
 *
 * 只封 `Date.now` 不够:`new Date()` 无参构造同样读系统时钟,而带参构造(解析固定字符串、
 * 由 tick 计算时间戳)是仿真包表达"链上时间"的正当手段,必须保留。
 */
export function installDeterminismGuards(scope: object): void {
  Math.random = blockedCapability;
  const RealDate = Date;
  const guarded = new Proxy(RealDate, {
    construct(target, args: unknown[]) {
      if (args.length === 0) {
        blockedCapability();
      }
      return Reflect.construct(target, args);
    },
    apply() {
      return blockedCapability();
    },
  });
  Object.defineProperty(guarded, 'now', {
    value: blockedCapability,
    configurable: true,
    writable: false,
  });
  sealGlobal(scope, 'Date', guarded);
}

/**
 * findPropertyDescriptor 沿原型链查找全局属性描述符。
 */
function findPropertyDescriptor(scope: object, key: string): PropertyDescriptor | undefined {
  let cursor: object | null = scope;
  while (cursor) {
    const descriptor = Object.getOwnPropertyDescriptor(cursor, key);
    if (descriptor) {
      return descriptor;
    }
    cursor = Object.getPrototypeOf(cursor) as object | null;
  }
  return undefined;
}
