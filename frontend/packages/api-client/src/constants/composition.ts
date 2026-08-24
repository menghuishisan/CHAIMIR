// 沙箱组合契约常量:实验、竞赛与判题共用的访问边界与组件来源取值。
//
// 字符串取值统一用 `as const` 对象 + 派生联合类型(与 SIM_COMPUTE、SANDBOX_CHAIN_OPERATION 一致),
// 不用 string enum —— 全平台同类问题只用同一种表达。

/**
 * SANDBOX_ACCESS_PROFILE 是组合运行期的访问边界,取值与后端闭集一致。
 * 它决定沙箱能连什么、使用者能否进入,故不是自由文本 —— 越界值后端直接拒绝。
 */
export const SANDBOX_ACCESS_PROFILE = {
  /** 实验环境:学生可进 IDE/终端,按实验定义挂工具 */
  EXPERIMENT: 'experiment',
  /** 解题赛实操环境 */
  CONTEST_SOLVE: 'contest-solve',
  /** 对抗赛官方对局环境 */
  CONTEST_BATTLE: 'contest-battle',
  /** 漏洞题预验证环境(教师侧正反向验证) */
  VULNERABILITY_PREVALIDATE: 'vulnerability-prevalidate',
  /** 判题私有环境:学生不可见、不可进入 */
  JUDGE_PRIVATE: 'judge-private',
} as const

export type SandboxAccessProfile =
  (typeof SANDBOX_ACCESS_PROFILE)[keyof typeof SANDBOX_ACCESS_PROFILE]

/**
 * COMPOSITION_SELECTION 是组件引用的来源标记。
 * 教师页面只提交 EXPLICIT;AUTO 由服务端编译器按能力依赖展开后写入,前端只读展示。
 */
export const COMPOSITION_SELECTION = {
  EXPLICIT: 'explicit',
  AUTO: 'auto',
} as const

export type CompositionSelection =
  (typeof COMPOSITION_SELECTION)[keyof typeof COMPOSITION_SELECTION]
