// identity options 文件维护身份与租户表单使用的封闭选项顺序。

import { TenantStatus, TENANT_MODULE } from '@chaimir/api-client'
import {
  TENANT_MODULE_DESCRIPTIONS,
  TENANT_MODULE_LABELS,
} from '../../utils/labels/identity'

/** TENANT_MODULE_OPTIONS 是校管端模块开关的唯一选项清单。 */
export const TENANT_MODULE_OPTIONS = Object.values(TENANT_MODULE).map((value) => ({
  value,
  label: TENANT_MODULE_LABELS[value],
  description: TENANT_MODULE_DESCRIPTIONS[value],
}))

/** TENANT_APPLICATION_SCHOOL_TYPES 按入驻表单顺序排列机构类型。 */
export const TENANT_APPLICATION_SCHOOL_TYPES = [1, 2, 3] as const

/** TenantApplicationSchoolType 是入驻表单允许选择的机构类型。 */
export type TenantApplicationSchoolType = (typeof TENANT_APPLICATION_SCHOOL_TYPES)[number]

/** TENANT_STATUSES 按状态调整表单顺序排列后端允许值。 */
export const TENANT_STATUSES = [TenantStatus.ACTIVE, TenantStatus.DISABLED, TenantStatus.EXPIRED] as const
