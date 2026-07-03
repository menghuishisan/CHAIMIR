// TenantPicker：展示手机号匹配到的学校候选并写回租户选择。

import React from 'react'
import { Button } from '@chaimir/ui'
import { safeParseTenants } from '../form-state'

/**
 * TenantPicker 展示手机号匹配到的学校列表，让用户选择目标租户后重试登录。
 */
export function TenantPicker({ tenants, onSelect }: { tenants?: string; onSelect: (tenantId: string) => void }): React.ReactElement | null {
  if (!tenants) {
    return null
  }
  const items = safeParseTenants(tenants)
  return (
    <div className="public-tenant-picker" role="group" aria-label="选择学校">
      {items.map((item) => (
        <Button className="public-tenant-choice" type="button" variant="outline" key={item.tenant_id} onClick={() => onSelect(item.tenant_id)}>
          {item.name}（{item.code}）
        </Button>
      ))}
    </div>
  )
}
