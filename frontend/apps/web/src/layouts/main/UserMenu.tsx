// UserMenu 顶栏身份区(规范 §6.3):显示当前账号姓名与角色,下拉提供个人中心与退出登录。
// 身份数据全部来自服务端会话(RoleGuard 的 /me),不接受任何客户端传入的角色。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import { ChevronDown, LogOut, UserCog } from 'lucide-react'
import {
  Menu,
  MenuContent,
  MenuItem,
  MenuLabel,
  MenuSeparator,
  MenuTrigger,
  cn,
} from '@chaimir/ui'
import { api } from '../../app/api'
import { useSession } from '../../components/RoleGuard'
import { clearLoginTokens } from '../../utils/authSession'
import { userRolesLabel } from '../../utils/labels/identity'

export interface UserMenuProps {
  /** 个人中心页路径(角色区内) */
  profilePath: string
  /** 退出后回到的登录入口(平台管理与其余角色入口不同) */
  loginPath: string
}

/**
 * UserMenu 渲染身份触发器与下拉菜单;退出登录先请求服务端注销,
 * 再清除本地令牌并回到登录页 —— 注销失败也必须完成本地清除,否则用户会卡在已登录假象里。
 */
export function UserMenu({ profilePath, loginPath }: UserMenuProps) {
  const navigate = useNavigate()
  const { me } = useSession()
  const [signingOut, setSigningOut] = useState(false)
  const account = me.account

  /** 身份副标题:优先职称,其次学号/工号,最后角色名 */
  const roleName = userRolesLabel(account.roles)
  const subtitle = account.title || account.no || roleName

  const signOut = useCallback(async () => {
    if (signingOut) return
    setSigningOut(true)
    try {
      await api.identity.logout()
    } catch {
      // 服务端注销失败不阻断退出:本地令牌必须清除,否则界面停留在已登录状态
    } finally {
      clearLoginTokens()
      navigate(loginPath, { replace: true })
    }
  }, [loginPath, navigate, signingOut])

  return (
    <Menu>
      <MenuTrigger
        className={cn(
          'pressable flex max-w-56 items-center gap-2 rounded-md px-2 py-1.5 text-left',
          'text-on-dark hover:bg-dark-elevated',
          'outline-hidden focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2',
        )}
        aria-label={`当前账号 ${account.name},展开账号菜单`}
      >
        <span
          aria-hidden="true"
          className="grid size-8 shrink-0 place-items-center rounded-full bg-on-dark-accent-soft text-sm font-medium text-accent"
        >
          {account.name.slice(0, 1)}
        </span>
        <span className="hidden min-w-0 flex-col sm:flex">
          <span className="truncate text-sm font-medium leading-tight">{account.name}</span>
          {subtitle ? (
            <span className="truncate text-xs leading-tight text-on-dark-sub">{subtitle}</span>
          ) : null}
        </span>
        <ChevronDown aria-hidden="true" className="size-4 shrink-0 text-on-dark-sub" />
      </MenuTrigger>

      <MenuContent align="end" className="min-w-52">
        <MenuLabel>
          {account.name}
          {roleName ? ` · ${roleName}` : ''}
        </MenuLabel>
        <MenuSeparator />
        <MenuItem icon={UserCog} onSelect={() => navigate(profilePath)}>
          个人中心
        </MenuItem>
        <MenuSeparator />
        <MenuItem icon={LogOut} danger disabled={signingOut} onSelect={() => void signOut()}>
          {signingOut ? '正在退出' : '退出登录'}
        </MenuItem>
      </MenuContent>
    </Menu>
  )
}
