// organizationView 提供 identity 组织页面共享的数据装配与班级统计。
// 三张组织表一次并行读取,避免教师与校管页面分别维护同一套 N+1 防护和关联统计。

import type { Class, Department, Major } from '@chaimir/api-client'
import { api } from '../../app/api'

/** OrganizationView 是组织页面一次读取的院系、专业与班级数据。 */
export interface OrganizationView {
  departments: Department[]
  majors: Major[]
  classes: Class[]
}

/** loadOrganizationView 并行读取院系、专业和班级并组装为统一视图。 */
export async function loadOrganizationView(): Promise<OrganizationView> {
  const [departments, majors, classes] = await Promise.all([
    api.identity.listDepartments(),
    api.identity.listMajors(),
    api.identity.listClasses(),
  ])
  return { departments, majors, classes }
}

/** countClassesByMajor 统计每个专业当前关联的班级数。 */
export function countClassesByMajor(classes: Class[]): Map<string, number> {
  const counts = new Map<string, number>()
  for (const entity of classes) {
    counts.set(entity.major_id, (counts.get(entity.major_id) ?? 0) + 1)
  }
  return counts
}
