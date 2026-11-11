// MainLayout 日常页外框(规范 FE-6 / §1.2):墨色侧栏与顶栏 + 浅色内容纸面。
// 纸面直接贴住侧栏和顶栏,不留缝、不加投影;侧栏常驻(≥lg)时左上角收成圆角(paper-surface)。
// 顶栏左侧是「当前位置」(页面经 usePageLocation 声明,规范 §6.2),右侧是任务与下载、通知铃铛、头像菜单;
// 浏览器标题随当前位置变化。内容区不再放面包屑。
// 断点(§6.4):≥lg 侧栏常驻可折叠 224↔64;<lg 侧栏收进抽屉(品牌在抽屉顶部),
// 顶栏依次是菜单按钮、返回上级、当前页名。路由切换只动纸面内容的淡入(§4.4)。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Outlet, useLocation } from 'react-router'
import { Menu as MenuIcon } from 'lucide-react'
import { Drawer, DrawerContent, DrawerTitle, IconButton, cn, useMediaQuery } from '@chaimir/ui'
import { RouteErrorBoundary } from '../../components/RouteErrorBoundary'
import { PageLocationContext, type PageLocationItem } from '../../hooks/pageLocation'
import type { RoleNavigationConfig } from './navigation'
import { AppSidebar } from './AppSidebar'
import { NotificationBell } from './NotificationBell'
import { TaskCenterButton } from './TaskCenterButton'
import { TopBarTrail } from './TopBarTrail'
import { UserMenu } from './UserMenu'
import { pageTitle, resolvePageTrail } from './pageTrail'
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
 * MainLayout 组装导航、顶栏与内容纸面。
 * 导航配置由所属角色区注入,壳层本身不认识其他角色 —— 这样打包时
 * 各角色的路径清单只进各自的懒加载块,不会汇聚到入口包。
 */
export function MainLayout({ config }: MainLayoutProps) {
  const location = useLocation()
  const isDesktop = useMediaQuery(DESKTOP_QUERY)

  const [collapsed, setCollapsed] = useState(readSidebarCollapsed)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [declared, setDeclared] = useState<PageLocationItem[] | null>(null)

  /** 回到桌面宽度时关闭抽屉:否则抽屉遮罩会盖在已常驻的侧栏上 */
  useEffect(() => {
    if (isDesktop) setDrawerOpen(false)
  }, [isDesktop])

  /** 路由切换后关闭抽屉:窄屏点导航即离开当前页,抽屉必须让位 */
  useEffect(() => {
    setDrawerOpen(false)
  }, [location.pathname])

  // 记住每个路径最近一次的查询串(筛选、搜索、页码):从详情页经顶栏点回列表时,视图还在(规范 §6.2)。
  // 只在内存里、只在本角色区内有效,刷新后由地址本身承担。
  const recentSearch = useRef(new Map<string, string>())
  useEffect(() => {
    if (location.search) recentSearch.current.set(location.pathname, location.search)
    else recentSearch.current.delete(location.pathname)
  }, [location.pathname, location.search])
  const hrefOf = useCallback((to: string) => {
    if (to.includes('?')) return to
    return `${to}${recentSearch.current.get(to) ?? ''}`
  }, [])

  const navItems = useMemo(() => config.groups.flatMap((group) => group.items), [config.groups])
  const trail = useMemo(
    () => resolvePageTrail(navItems, location.pathname, declared),
    [declared, location.pathname, navItems],
  )

  /** 浏览器标题随当前位置变化;离开日常页外框时还原 */
  useEffect(() => {
    document.title = pageTitle(trail)
  }, [trail])
  useEffect(() => {
    const original = document.title
    return () => {
      document.title = original
    }
  }, [])

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
    <PageLocationContext.Provider value={setDeclared}>
      {/* 外框恰好一屏高、自身不滚动:只有纸面内层(paper-scrollport)滚动,
          页面里的吸顶、吸底元素(保存条、答题卡、阅读目录)以它为准 */}
      <div className="flex h-dvh overflow-hidden bg-substrate">
        <a href="#main-content" className="skip-link">
          跳到主要内容
        </a>

        {/* 常驻侧栏(≥lg):固定宽度走令牌,折叠只改宽度不改结构 */}
        {isDesktop ? (
          <div
            className="h-full shrink-0 transition-sidebar duration-base ease-out"
            style={{ width: collapsed ? 'var(--sidebar-w-collapsed)' : 'var(--sidebar-w)' }}
          >
            <AppSidebar config={config} collapsed={collapsed} onToggleCollapsed={toggleCollapsed} />
          </div>
        ) : (
          // 抽屉侧栏(<lg):Radix Dialog 提供遮罩(z-drawer)、Esc 与焦点陷阱;品牌在抽屉顶部
          <Drawer open={drawerOpen} onOpenChange={setDrawerOpen}>
            <DrawerContent side="left" tone="dark" className="max-w-64 p-0">
              <DrawerTitle className="sr-only">主导航</DrawerTitle>
              <AppSidebar config={config} />
            </DrawerContent>
          </Drawer>
        )}

        {/* 右侧:顶栏 + 内容纸面;min-w-0 防子元素撑破布局产生非预期横向滚动 */}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <header
            className={cn(
              'z-sticky flex shrink-0 items-center gap-2 bg-substrate',
              isDesktop ? 'pl-9 pr-4' : 'px-3',
            )}
            style={{ height: 'var(--topnav-h)' }}
          >
            {!isDesktop ? (
              <IconButton
                variant="on-dark"
                icon={MenuIcon}
                aria-label="打开主导航"
                onClick={() => setDrawerOpen(true)}
                className="border-transparent"
              />
            ) : null}

            <TopBarTrail trail={trail} compact={!isDesktop} hrefOf={hrefOf} />

            <div className="flex shrink-0 items-center gap-1">
              <TaskCenterButton tasksPath={tasksPath} />
              {/* 铃铛只在有站内信收件箱的端出现:归属由该区导航配置声明(见 navigation.ts) */}
              {config.hasNotificationInbox ? <NotificationBell allPath={notificationsPath} /> : null}
              <UserMenu profilePath={profilePath} loginPath={config.loginPath} />
            </div>
          </header>

          {/* 内容纸面:贴住侧栏与顶栏,外层固定形状,内层独立滚动 */}
          <main
            id="main-content"
            /* key 绑定路径:切页时重放入场动画,只动 opacity/translateY(§4.4) */
            key={location.pathname}
            className="paper-surface animate-pane-in"
          >
            <div className="paper-scrollport">
              <RouteErrorBoundary>
                <Outlet />
              </RouteErrorBoundary>
            </div>
          </main>
        </div>
      </div>
    </PageLocationContext.Provider>
  )
}
