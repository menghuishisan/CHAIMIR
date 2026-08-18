// judge 领域维护判题器表单使用的枚举选项顺序。

import { JudgerStatus, JudgerType } from '@chaimir/api-client'

/** JUDGER_TYPES 供表单按登记顺序渲染判题器类型选项。 */
export const JUDGER_TYPES = [
  JudgerType.TESTCASE,
  JudgerType.ONCHAIN_ASSERT,
  JudgerType.FLAG,
  JudgerType.STATIC_SCAN,
  JudgerType.SIM_CHECKPOINT,
  JudgerType.MANUAL,
] as const

/** JUDGER_STATUSES 供表单渲染状态选项。 */
export const JUDGER_STATUSES = [JudgerStatus.AVAILABLE, JudgerStatus.DISABLED] as const
