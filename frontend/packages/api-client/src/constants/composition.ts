/** 由服务端校验的组合访问范围。 */
export const SANDBOX_ACCESS_PROFILE = {
  /** 实验环境:学生可进 IDE/终端,按实验定义挂工具 */
  EXPERIMENT: 'experiment',
  CONTEST_SOLVE: 'contest-solve',
  CONTEST_BATTLE: 'contest-battle',
  VULNERABILITY_PREVALIDATE: 'vulnerability-prevalidate',
  /** 判题私有环境:学生不可见、不可进入 */
  JUDGE_PRIVATE: 'judge-private',
} as const

export type SandboxAccessProfile =
  (typeof SANDBOX_ACCESS_PROFILE)[keyof typeof SANDBOX_ACCESS_PROFILE]

/** 教师仅提交显式组件。自动依赖由编译器生成，只读展示。 */
export const COMPOSITION_SELECTION = {
  EXPLICIT: 'explicit',
  AUTO: 'auto',
} as const

export type CompositionSelection =
  (typeof COMPOSITION_SELECTION)[keyof typeof COMPOSITION_SELECTION]
