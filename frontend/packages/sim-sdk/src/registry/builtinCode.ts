// 本文件只负责内置仿真包 code 的识别规则,不引用任何可运行包。
// 拆成独立模块的原因:主线程(如公开回放页)需要判定一个 package_code 能否本地运行,
// 而内置包内核只允许在 Worker 内解析 —— 从主入口导出 registry 会把整个 builtin/ 拉进主线程包。

/** 平台内置前端计算包的 code 前缀,与后端 sim 模块的内置包判定规则同源。 */
export const BUILTIN_SIM_CODE_PREFIX = 'builtin__';

/**
 * isBuiltinSimulationCode 判定 package_code 是否为平台内置前端计算包。
 * 内置包在装配期已强制使用该前缀(见 builtinRegistry 的唯一索引),因此前缀判定等价于内置包成员判定。
 */
export function isBuiltinSimulationCode(code: string): boolean {
  return code.startsWith(BUILTIN_SIM_CODE_PREFIX);
}
