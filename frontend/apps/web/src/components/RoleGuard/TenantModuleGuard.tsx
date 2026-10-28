// TenantModuleGuard 在角色懒加载块内统一阻止关闭模块的日常页与沉浸深链。
import type { ReactNode } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router'
import { ShieldAlert } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { AppStatusScreen } from '../AppStatusScreen'
import { useSession } from './session'
import {
  availableRoleNavigation,
  requiredNavigationModule,
  type RoleNavigationConfig,
} from '../../layouts/main/navigation'

/** TenantModuleGuard 不挂载被拒绝的子页，避免其读取或提交业务数据。 */
export function TenantModuleGuard({
  config,
  children,
}: {
  config: RoleNavigationConfig
  children: ReactNode
}) {
  const { me } = useSession()
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const required = requiredNavigationModule(config, pathname)
  if (!required || me.enabled_modules.includes(required)) return children

  const available = availableRoleNavigation(config, me.enabled_modules)
  // 登录使用规范角色首页，模块关闭时统一选择首个可用入口，避免首页重定向循环。
  if (pathname === config.homePath) return <Navigate to={available.homePath} replace />
  return (
    <AppStatusScreen
      icon={ShieldAlert}
      title="学校暂未开放此功能"
      description="请联系学校管理员了解开放安排，你仍可以使用其它已开放功能。"
      actions={
        <Button variant="on-dark" onClick={() => navigate(available.homePath, { replace: true })}>
          回到我的首页
        </Button>
      }
    />
  )
}
