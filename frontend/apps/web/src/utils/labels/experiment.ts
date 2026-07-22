// experiment labels 文件维护实验发布和协作模式文案。

import { ExperimentCollabMode, ExperimentStatus } from '@chaimir/api-client'
import { labelFromMap } from './map'

/** experimentStatusLabel 返回实验编排发布状态文案。 */
export function experimentStatusLabel(status: ExperimentStatus): string {
  return labelFromMap(status, { [ExperimentStatus.PUBLISHED]: '已发布', [ExperimentStatus.UNPUBLISHED]: '已下架' }, '草稿')
}

/** experimentCollabModeLabel 返回实验协作模式文案。 */
export function experimentCollabModeLabel(mode: ExperimentCollabMode): string {
  return labelFromMap(mode, { [ExperimentCollabMode.SOLO]: '个人实验', [ExperimentCollabMode.GROUP]: '小组实验' }, '未识别的协作模式')
}
