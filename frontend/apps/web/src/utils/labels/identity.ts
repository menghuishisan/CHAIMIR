// identity labels 文件维护 identity 模块枚举值的用户向文案。

/** 机构类型取值与后端 school_type 一致(1 本科 / 2 高职高专 / 3 其他),取值与文案在此单一登记。 */
const TENANT_APPLICATION_SCHOOL_TYPE_LABELS = {
  1: '本科院校',
  2: '高职高专',
  3: '其他教育机构',
} as const

export type TenantApplicationSchoolType = keyof typeof TENANT_APPLICATION_SCHOOL_TYPE_LABELS

/** TENANT_APPLICATION_SCHOOL_TYPES 供表单按登记顺序渲染选项,避免页面再抄一份取值清单。 */
export const TENANT_APPLICATION_SCHOOL_TYPES = [1, 2, 3] as const satisfies readonly TenantApplicationSchoolType[]

/** tenantApplicationSchoolTypeLabel 返回入驻机构类型文案。 */
export function tenantApplicationSchoolTypeLabel(type: TenantApplicationSchoolType): string {
  return TENANT_APPLICATION_SCHOOL_TYPE_LABELS[type]
}
