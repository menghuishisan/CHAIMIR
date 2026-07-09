// orchestration.ts 定义教师实验编排表单的默认后端契约结构。

import type { ComponentConfig, GroupConfig } from '@chaimir/api-client'

/**
 * emptyExperimentComponents 提供新建实验时的空组件集合。
 */
export const emptyExperimentComponents: ComponentConfig = {
  envs: [],
  sims: [],
  checkpoints: [],
  stages: [],
}

/**
 * defaultExperimentGroup 提供个人实验的默认分组配置。
 */
export const defaultExperimentGroup: GroupConfig = {
  size: 1,
  roles: [],
}
