// admin options 文件维护 M9 管理表单使用的封闭选项顺序。

import { AlertLevel } from '@chaimir/api-client'

/** ALERT_LEVELS 按告警规则表单顺序排列级别。 */
export const ALERT_LEVELS = [
  AlertLevel.NOTICE,
  AlertLevel.WARNING,
  AlertLevel.SEVERE,
  AlertLevel.CRITICAL,
] as const satisfies readonly AlertLevel[]

/** ALERT_METRICS 是告警规则允许选择的指标键。 */
export const ALERT_METRICS = [
  'sandbox.active_count',
  'sandbox.failed_rate',
  'judge.queue_length',
  'judge.failed_rate',
  'account.login_failed_count',
  'transfer.failed_count',
  'grade.warning_count',
] as const

/** ALERT_CONDITION_OPERATORS 是告警规则允许选择的比较方式。 */
export const ALERT_CONDITION_OPERATORS = ['gt', 'gte', 'lt', 'lte'] as const
