// AppSidebar 角色导航(生长在墨色底层上,非独立面板,FE-6/规范 §6.2):
// 主标志 + 分组导航(图标+文字,激活态玉浅底+左指示条+aria-current)。
// 折叠态(仅桌面)只留图标,标签经 Tooltip 提供;窄屏由 MainLayout 放入抽屉,不折叠。

import { NavLink } from 'react-router'
import { PanelLeftClose, PanelLeftOpen } from 'lucide-react'
import { BrandMark, Icon, IconButton, Tooltip, cn } from '@chaimir/ui'
import type { RoleNavigationConfig, RoleNavigationItem } from './navigation'
import { useOnlineStatus } from '../../hooks'

export interface AppSidebarProps {
  config: RoleNavigationConfig
  /** 折叠为图标条(仅桌面常驻形态) */
  collapsed?: boolean
  /** 传入即渲染底部折叠开关(抽屉形态不传) */
  onToggleCollapsed?: () => void
}

/**
 * AppSidebar 渲染品牌与分组导航;当前位置高亮由 NavLink 判定。
 */
export function AppSidebar({ config, collapsed = false, onToggleCollapsed }: AppSidebarProps) {
  const online = useOnlineStatus()
  return (
    <div className="flex h-full min-h-0 flex-col px-3 py-4">
      {/* 品牌:锁定组合 = 主标志 + Chaimir,旁边不加角色名/端名(规范 §1.3);角色由分组导航与头像菜单表达 */}
      <div className={cn('flex items-center gap-2.5 px-2 pb-4', collapsed && 'justify-center px-0')}>
        <BrandMark size="md" className="text-accent" label={collapsed ? 'Chaimir' : undefined} />
        {!collapsed && <span className="truncate text-sm font-semibold text-on-dark">Chaimir</span>}
      </div>

      {/* 分组导航:独立滚动,不带动整页 */}
      <nav aria-label="主导航" className="min-h-0 flex-1 overflow-y-auto">
        {config.groups.map((group) => (
          <div key={group.title} className="mt-4 first:mt-0">
            {!collapsed && (
              <div className="px-2.5 pb-1.5 font-mono text-xs text-on-dark-sub">
                {group.title}
              </div>
            )}
            <ul className="flex flex-col gap-0.5">
              {group.items.map((item) => (
                <li key={item.path}>
                  <SidebarLink item={item} collapsed={collapsed} />
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>

      {/* 底部:连通状态一行(底层「账本活着」的常驻微表达,静态非动画)+ 桌面折叠开关 */}
      <div className="mt-3 border-t border-dark-line pt-3">
        <ConnectionRow collapsed={collapsed} online={online} />
        {onToggleCollapsed ? (
          <div className={cn('mt-2 flex', collapsed ? 'justify-center' : 'justify-end pr-1')}>
            <IconButton
              variant="on-dark"
              size="sm"
              icon={collapsed ? PanelLeftOpen : PanelLeftClose}
              aria-label={collapsed ? '展开导航' : '收起导航'}
              onClick={onToggleCollapsed}
            />
          </div>
        ) : null}
      </div>
    </div>
  )
}

/**
 * ConnectionRow 侧栏底部连通状态行:等宽字体一行,点色承载语义但文字才是信息载体。
 * 折叠态只留点,状态文字经 Tooltip 提供。
 */
function ConnectionRow({ collapsed, online }: { collapsed: boolean; online: boolean }) {
  const label = online ? '已同步' : '网络已断开'
  const dot = (
    <span
      aria-hidden="true"
      className={cn('size-1.5 shrink-0 rounded-full', online ? 'bg-accent' : 'bg-on-dark-faint')}
    />
  )

  if (collapsed) {
    return (
      <Tooltip content={label} side="right">
        <div className="flex justify-center py-1" role="status">
          {dot}
          <span className="sr-only">{label}</span>
        </div>
      </Tooltip>
    )
  }

  return (
    <div
      role="status"
      className="flex items-center gap-2 px-2.5 font-mono text-xs text-on-dark-sub"
    >
      {dot}
      {label}
    </div>
  )
}

/**
 * SidebarLink 单个导航项;折叠态用 Tooltip 补文字(可发现性)。
 */
function SidebarLink({ item, collapsed }: { item: RoleNavigationItem; collapsed: boolean }) {
  const link = (
    <NavLink
      to={item.path}
      className={({ isActive }) =>
        cn(
          'relative flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors duration-fast',
          collapsed && 'justify-center px-0 py-2.5',
          isActive
            ? 'bg-on-dark-accent-soft font-medium text-accent before:absolute before:left-0 before:top-1/2 before:h-4 before:w-0.5 before:-translate-y-1/2 before:rounded-full before:bg-accent'
            : 'text-on-dark-sub hover:bg-dark-elevated hover:text-on-dark',
        )
      }
    >
      <Icon icon={item.icon} size="sm" />
      {!collapsed && <span className="truncate">{item.name}</span>}
    </NavLink>
  )

  if (!collapsed) return link
  return (
    <Tooltip content={item.name} side="right">
      {link}
    </Tooltip>
  )
}
