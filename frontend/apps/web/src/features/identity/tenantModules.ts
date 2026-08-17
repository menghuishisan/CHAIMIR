/** identity tenantModules 文件统一解释租户 feature_flags.modules 封闭契约。 */

import { TENANT_MODULE_VALUES, type TenantModule } from '@chaimir/api-client'

/** TENANT_MODULES_KEY 是租户功能开关中业务模块集合的固定键。 */
export const TENANT_MODULES_KEY = 'modules'

/** readTenantModules 区分字段缺失的默认全启用与显式数组,并拒绝未知值或重复项。 */
export function readTenantModules(featureFlags: Record<string, unknown>): TenantModule[] {
  const raw = featureFlags[TENANT_MODULES_KEY]
  if (raw === undefined) return [...TENANT_MODULE_VALUES]
  if (!Array.isArray(raw)) return []

  const seen = new Set<TenantModule>()
  const modules: TenantModule[] = []
  for (const item of raw) {
    if (typeof item !== 'string' || !TENANT_MODULE_VALUES.includes(item as TenantModule)) return []
    const module = item as TenantModule
    if (seen.has(module)) return []
    seen.add(module)
    modules.push(module)
  }
  return modules
}
