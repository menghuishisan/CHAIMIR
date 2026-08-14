// MainLayout 日常页壳层(FE-6 底层曝光系统的「光面态」):
// 侧栏与顶栏直接生长在墨色底层上,内容区是浮起的宣纸光面(四周可见底层)。
// 断点行为(§6.4):≥lg 侧栏常驻可折叠 220↔64;<lg 侧栏抽屉化(汉堡触发,遮罩层级高于侧栏,
// Esc 与焦点陷阱由 Radix Dialog 保证);<md 光面内距与圆角归零走全宽。
// 路由切换只动光面内容的 opacity/translateY,底层与导航保持静止(§4.4)。

import { useCallback, useEffect, useState } from 'react'
import { Outlet, useLocation } from 'react-router'
import { Menu as MenuIcon } from 'lucide-react'
import {
  BrandMark,
  Drawer,
  DrawerContent,
  DrawerTitle,
  IconButton,
  cn,
} from '@chaimir/ui'
import { RouteErrorBoundary } from '../../components/RouteErrorBoundary'
import { useMediaQuery } from '../../hooks'
import type { RoleNavigationConfig } from './navigation'
import { AppSidebar } from './AppSidebar'
import { NotificationBell } from './NotificationBell'
import { TaskCenterButton } from './TaskCenterButton'
import { UserMenu } from './UserMenu'

/** 侧栏折叠选择:仅界面偏好(非业务状态),全站一个键 —— 同一个人换端不需要重设一次 */
const SIDEBAR_COLLAPSED_KEY = 'chaimir.sidebar_collapsed'

/** 常驻侧栏断点:与 Tailwind lg(1024px)一致,禁止另造魔法断点 */
const DESKTOP_QUERY = '(min-width: 64rem)'

export interface MainLayoutProps {
  /** 本角色区的导航配置,由 routes/ 层在该区懒加载壳内注入(铁律 2:壳层不持有全角色清单) */
  config: RoleNavigationConfig
}

/**
 * MainLayout 组装底层上的导航壳与光面内容区。
 * 导航配置由所属角色区注入,壳层本身不认识其他角色 —— 这样打包时
 * 各角色的路径清单只进各自的懒加载块,不会汇聚到入口包。
 */
export function MainLayout({ config }: MainLayoutProps) {
  const location = useLocation()
  const isDesktop = useMediaQuery(DESKTOP_QUERY)

  const [collapsed, setCollapsed] = useState(
    () => window.localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === 'true',
  )
  const [drawerOpen, setDrawerOpen] = useState(false)

  /** 回到桌面宽度时关闭抽屉:否则抽屉遮罩会盖在已常驻的侧栏上 */
  useEffect(() => {
    if (isDesktop) setDrawerOpen(false)
  }, [isDesktop])

  /** 路由切换后关闭抽屉:窄屏点导航即离开当前页,抽屉必须让位 */
  useEffect(() => {
    setDrawerOpen(false)
  }, [location.pathname])

  const toggleCollapsed = useCallback(() => {
    setCollapsed((previous) => {
      const next = !previous
      window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(next))
      return next
    })
  }, [])

  const notificationsPath = `${config.pathPrefix}/notifications`
  const tasksPath = `${config.pathPrefix}/tasks`
  const profilePath = `${config.pathPrefix}/profile`

  return (
    <div className="flex min-h-dvh bg-substrate">
      <a href="#main-content" className="skip-link">
        跳到主要内容
      </a>

      {/* 常驻侧栏(≥lg):固定宽度走令牌,折叠只改宽度不改结构 */}
      {isDesktop ? (
        <div
          className="sticky top-0 h-dvh shrink-0 transition-sidebar duration-base ease-out"
          style={{ width: collapsed ? 'var(--sidebar-w-collapsed)' : 'var(--sidebar-w)' }}
        >
          <AppSidebar config={config} collapsed={collapsed} onToggleCollapsed={toggleCollapsed} />
        </div>
      ) : (
        // 抽屉侧栏(<lg):Radix Dialog 提供遮罩(z-drawer)、Esc 与焦点陷阱
        <Drawer open={drawerOpen} onOpenChange={setDrawerOpen}>
          <DrawerContent side="left" tone="dark" className="max-w-64 p-0">
            <DrawerTitle className="sr-only">主导航</DrawerTitle>
            <AppSidebar config={config} />
          </DrawerContent>
        </Drawer>
      )}

      {/* 右侧:顶栏 + 光面内容区;min-w-0 防子元素撑破布局产生非预期横向滚动 */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header
          className="sticky top-0 z-sticky flex shrink-0 items-center gap-2 bg-substrate px-3 sm:px-4"
          style={{ height: 'var(--topnav-h)' }}
        >
          {!isDesktop ? (
            <IconButton
              variant="on-dark"
              icon={MenuIcon}
              aria-label="打开主导航"
              onClick={() => setDrawerOpen(true)}
            />
          ) : null}

          {/* 窄屏顶栏承载品牌与端名(侧栏收进抽屉后品牌需有落点) */}
          {!isDesktop ? (
            <span className="flex min-w-0 flex-1 items-center gap-2">
              <BrandMark size="sm" className="text-accent" />
              <span className="truncate text-sm font-semibold text-on-dark">
                {config.brandName}
              </span>
            </span>
          ) : (
            <span className="flex-1" />
          )}

          <div className="flex shrink-0 items-center gap-1">
            <TaskCenterButton tasksPath={tasksPath} />
            {/* 铃铛只在有站内信收件箱的端出现:归属由该区导航配置声明(见 navigation.ts) */}
            {config.hasNotificationInbox ? <NotificationBell allPath={notificationsPath} /> : null}
            <UserMenu profilePath={profilePath} loginPath={config.loginPath} />
          </div>
        </header>

        {/* 光面外框:四周底层曝光(上右下 14px,左侧 8px 适度分离);<md 归零全宽 */}
        <div className="flex min-h-0 flex-1 flex-col p-0 md:pb-3.5 md:pl-2 md:pr-3.5 md:pt-3.5">
          <main
            id="main-content"
            /* key 绑定路径:切页时重放入场动画,只动 opacity/translateY(§4.4) */
            key={location.pathname}
            className={cn(
              'flex min-h-0 flex-1 flex-col overflow-hidden bg-canvas',
              'rounded-none md:rounded-pane md:shadow-pane',
              'animate-pane-in',
            )}
          >
            <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
              <RouteErrorBoundary>
                <Outlet />
              </RouteErrorBoundary>
            </div>
          </main>
        </div>
      </div>
    </div>
  )
}
