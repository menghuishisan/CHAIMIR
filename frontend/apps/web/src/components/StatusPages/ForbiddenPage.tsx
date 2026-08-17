// ForbiddenPage(403):已登录但角色与当前区域不符时展示。
// 保留原 URL(便于返回/报障),提供回到本人首页与退出登录两条出路;
// 绝不把角色不匹配误判为未登录(审查 S6 的根治点)。

import { useNavigate } from 'react-router'
import { ShieldX } from 'lucide-react'
import type { UserRole } from '@chaimir/api-client'
import { Button } from '@chaimir/ui'
import { AppStatusScreen } from '../AppStatusScreen'
import { clearLoginTokens } from '../../utils/authSession'
import { roleRouteForRoles } from '../../utils/roleRouting'

export interface ForbiddenPageProps {
  /** 当前账号的角色(用于计算「回到我的首页」落点) */
  roles: UserRole[]
}

/**
 * ForbiddenPage 说明无权原因并引导用户回到自己的功能区。
 */
export function ForbiddenPage({ roles }: ForbiddenPageProps) {
  const navigate = useNavigate()
  const homePath = roleRouteForRoles(roles)?.homePath

  return (
    <AppStatusScreen
      icon={ShieldX}
      tone="danger"
      title="没有访问这个页面的权限"
      description="你当前的账号角色无法查看该区域。如果你认为这是误判,请联系本校管理员。"
      actions={
        <>
          {homePath ? (
            <Button variant="primary" onClick={() => navigate(homePath, { replace: true })}>
              回到我的首页
            </Button>
          ) : null}
          <Button
            variant="on-dark"
            onClick={() => {
              clearLoginTokens()
              navigate('/auth/login', { replace: true })
            }}
          >
            退出登录
          </Button>
        </>
      }
    />
  )
}
