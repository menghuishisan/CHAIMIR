// MainLayout 日常页壳层(FE-6 底层曝光系统的「光面态」):
// 侧栏与顶栏直接生长在墨色底层上,内容区是浮起的宣纸光面。
// 光面的缝只留在**导航侧**(顶与左,由 pane-frame 取 --pane-inset),右下出血到视口边缘;
// 圆角只在左上角(radius-lg)、投影只从顶左透出 —— 四周等宽的缝会把内容读作「被装进一个容器」(规范 §1.2)。
// 断点行为(§6.4):≥lg 侧栏常驻可折叠 220↔64;<lg 侧栏抽屉化(汉堡触发,遮罩层级高于侧栏,
// Esc 与焦点陷阱由 Radix Dialog 保证);<md 缝与圆角归零走全宽。
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
  useMediaQuery,
} from '@chaimir/ui'
import { RouteErrorBoundary } from '../../components/RouteErrorBoundary'
import type { RoleNavigationConfig } from './navigation'
import { AppSidebar } from './AppSidebar'
import { NotificationBell } from './NotificationBell'
import { TaskCenterButton } from './TaskCenterButton'
import { UserMenu } from './UserMenu'
import { errorDiagnostics } from '../../utils/userFacingError'

/** 侧栏折叠选择:仅界面偏好(非业务状态),全站一个键 —— 同一个人换端不需要重设一次 */
const SIDEBAR_COLLAPSED_KEY = 'chaimir.sidebar_collapsed'

/** 常驻侧栏断点:与 Tailwind lg(1024px)一致,禁止另造魔法断点 */
const DESKTOP_QUERY = '(min-width: 64rem)'

/** readSidebarCollapsed 读取可选布局偏好,存储被策略禁用时回到默认展开状态。 */
function readSidebarCollapsed(): boolean {
  try {
    return window.localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === 'true'
  } catch (error) {
    console.warn('sidebar_preference_read_failed', {
      operation: 'read_sidebar_collapsed',
      error: errorDiagnostics(error),
    })
    return false
  }
}

/** writeSidebarCollapsed 持久化可选布局偏好,失败不影响当前交互。 */
function writeSidebarCollapsed(collapsed: boolean): void {
  try {
    window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(collapsed))
  } catch (error) {
    console.warn('sidebar_preference_write_failed', {
      operation: 'write_sidebar_collapsed',
      error: errorDiagnostics(error),
    })
  }
}

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

  const [collapsed, setCollapsed] = useState(readSidebarCollapsed)
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
      writeSidebarCollapsed(next)
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

          {/* 窄屏顶栏承载品牌(侧栏收进抽屉后锁定组合需有落点),同样不加端名(§1.3) */}
          {!isDesktop ? (
            <span className="flex min-w-0 flex-1 items-center gap-2">
              <BrandMark size="sm" className="text-accent" />
              <span className="truncate text-sm font-semibold text-on-dark">Chaimir</span>
            </span>
          ) : (
            /*
              宽屏顶栏左侧刻意留空,这是决定而不是遗漏:
              页面标题与面包屑归光面里的 PageHeader,顶栏再放一遍就是把同一件事说两遍
              (§6.5.0 通则 1);全站检索没有对应的后端能力,放一个搜不到东西的框更糟。
              这段裸露的墨色底层正是 FE-6 的「三种状态共用同一世界」——
              它是层次信号,不是待填的空位,不得为填满而加装饰或重复导航(§1.2)。
            */
            <span className="flex-1" />
          )}

          <div className="flex shrink-0 items-center gap-1">
            <TaskCenterButton tasksPath={tasksPath} />
            {/* 铃铛只在有站内信收件箱的端出现:归属由该区导航配置声明(见 navigation.ts) */}
            {config.hasNotificationInbox ? <NotificationBell allPath={notificationsPath} /> : null}
            <UserMenu profilePath={profilePath} loginPath={config.loginPath} />
          </div>
        </header>

        {/* 光面外框:缝只在导航侧(顶与左),右下出血到视口边;<md 归零全宽 */}
        <div className="pane-frame flex min-h-0 flex-1 flex-col">
          <main
            id="main-content"
            /* key 绑定路径:切页时重放入场动画,只动 opacity/translateY(§4.4) */
            key={location.pathname}
            className={cn(
              'flex min-h-0 flex-1 flex-col overflow-hidden bg-canvas',
              'rounded-none md:rounded-tl-lg md:shadow-pane',
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
