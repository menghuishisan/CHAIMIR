// 实验编排向导的服务端草稿状态机(教师深页共用)。
//
// FE-7:wizard_step 落服务端,刷新或换设备继续编排都不丢。
// 每一步「下一步」即整体保存(PATCH 实验)并把 wizard_step 推进一格 ——
// 后端 ExperimentRequest 是整体更新,没有分步接口,故每步提交完整草稿。
//
// 步进用 Steps 组件表达,禁止把 wizard_step 做成手填数字输入(规范 §6.6)。

import { useCallback, useState } from 'react'
import type {
  ComponentConfig,
  Experiment,
  ExperimentCollabMode,
  ExperimentRequest,
  GroupConfig,
} from '@chaimir/api-client'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 向导四步:与 wizard_step 的取值一一对应(1 起,便于服务端读出即定位)。 */
export const WIZARD_STEPS = [
  { key: 'basic', label: '基础信息', description: '名称、说明与完成方式' },
  { key: 'components', label: '环境与仿真', description: '代码环境与仿真场景' },
  { key: 'stages', label: '实验阶段', description: '分阶段与解锁条件' },
  { key: 'checkpoints', label: '检查点', description: '判分点与分值' },
] as const

export type WizardStepKey = (typeof WIZARD_STEPS)[number]['key']

/** WIZARD_STEP_COUNT 供进度表达与越界夹取共用同一来源。 */
export const WIZARD_STEP_COUNT = WIZARD_STEPS.length

/** ExperimentDraft 是向导内的完整草稿形状,与后端 ExperimentRequest 对齐。 */
export interface ExperimentDraft {
  course_id?: string
  template_ref: string
  template_version: string
  name: string
  description: string
  components: ComponentConfig
  collab_mode: ExperimentCollabMode
  group_config: GroupConfig
  require_report: boolean
  wizard_step: number
}

/** emptyComponents 给出空组件配置,四类组件都以空数组起步(后端要求字段存在)。 */
export function emptyComponents(): ComponentConfig {
  return { envs: [], sims: [], checkpoints: [], stages: [] }
}

/**
 * draftFromExperiment 把服务端实验定义转成向导草稿。
 * 这是「进入向导先拉服务端草稿」的落点:服务端草稿为权威,本地不预填任何内容。
 */
export function draftFromExperiment(experiment: Experiment): ExperimentDraft {
  return {
    course_id: experiment.course_id,
    template_ref: experiment.template_ref ?? '',
    template_version: experiment.template_version ?? '',
    name: experiment.name,
    description: experiment.description,
    components: experiment.components,
    collab_mode: experiment.collab_mode,
    group_config: experiment.group_config,
    require_report: experiment.require_report,
    wizard_step: experiment.wizard_step,
  }
}

/** toRequest 把草稿转成后端更新请求;course_id 可缺省(不挂课程的独立实验)。 */
function toRequest(draft: ExperimentDraft): ExperimentRequest {
  return {
    course_id: draft.course_id,
    template_ref: draft.template_ref,
    template_version: draft.template_version,
    name: draft.name,
    description: draft.description,
    components: draft.components,
    collab_mode: draft.collab_mode,
    group_config: draft.group_config,
    require_report: draft.require_report,
    wizard_step: draft.wizard_step,
  }
}

export interface WizardPersistence {
  /** 当前所在步骤下标(0 起) */
  stepIndex: number
  /** 保存中:按钮 loading 与阻止重复提交 */
  saving: boolean
  /** 上一次保存失败的用户向文案 */
  saveError: string | undefined
  /** 最近一次成功保存的时刻,供 Autosave 展示 */
  savedAt: Date | undefined
  /** saveStep 保存当前草稿并把步骤推进到目标下标;返回是否成功 */
  saveStep: (draft: ExperimentDraft, nextIndex: number) => Promise<boolean>
  /** goBack 回到上一步:后退不需要保存(已保存过的内容不因后退丢失) */
  goBack: () => void
}

/**
 * useWizardPersistence 管理向导的服务端持久化与步骤推进。
 * 步骤下标以服务端 wizard_step 为初值:刷新后回到上次所在的步骤。
 */
export function useWizardPersistence(
  experimentId: string,
  initialStep: number,
  onSaved: () => void,
): WizardPersistence {
  // 服务端 wizard_step 从 1 起;越界时夹到有效范围,避免刷新后落到不存在的步骤
  const [stepIndex, setStepIndex] = useState(() =>
    Math.min(Math.max(initialStep - 1, 0), WIZARD_STEP_COUNT - 1),
  )
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string>()
  const [savedAt, setSavedAt] = useState<Date>()

  const saveStep = useCallback(
    async (draft: ExperimentDraft, nextIndex: number): Promise<boolean> => {
      setSaving(true)
      setSaveError(undefined)
      try {
        // wizard_step 与目标步骤同步落库:服务端记录的就是用户下次进来该看到的那一步
        await api.experiment.updateExperiment(experimentId, {
          ...toRequest(draft),
          wizard_step: nextIndex + 1,
        })
        setSavedAt(new Date())
        setStepIndex(nextIndex)
        onSaved()
        return true
      } catch (error) {
        setSaveError(userFacingErrorMessage(error, '这一步没有保存成功,请稍后重试。'))
        return false
      } finally {
        setSaving(false)
      }
    },
    [experimentId, onSaved],
  )

  const goBack = useCallback(() => {
    setStepIndex((current) => Math.max(0, current - 1))
  }, [])

  return { stepIndex, saving, saveError, savedAt, saveStep, goBack }
}
