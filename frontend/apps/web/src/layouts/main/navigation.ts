// navigation 定义 MainLayout 导航契约和模块过滤，不含任何角色的导航数据。
// 铁律 2(目录设计 §4):普通用户不得下载其他角色、尤其是管理后台的路由结构,
// 因此导航数据按角色分文件放在 routes/sections/,随该角色区的懒加载壳一起加载。
// 各区只把自己的配置传入公共过滤函数，不会把其它角色路径清单带进入口包。

import type { LucideIcon } from 'lucide-react'
import type { TenantModule } from '@chaimir/api-client'
import type { RoleRouteConfig } from '../../utils/roleRouting'

export interface RoleNavigationItem {
  /**
   * 侧栏可见的栏目名(面向用户的自然语言,不用开发术语)。
   * 名称与顺序的唯一真相源是 `docs/前端后端功能对齐清单.md` §1.4 四端侧栏表,必须逐字一致。
   */
  name: string
  path: string
  icon: LucideIcon
  /** 对应租户业务开关；未声明的身份和共享资源入口不受五类开关控制。 */
  module?: TenantModule
}

interface RoleNavigationGroup {
  /** 分组标题:纯中文短词,不加英文副标 */
  title: string
  items: RoleNavigationItem[]
}

export interface RoleNavigationConfig extends RoleRouteConfig {
  /**
   * 该端是否有站内信收件箱(顶栏通知铃铛)。
   * M10 收件箱、未读数、通知偏好、公告已读和业务 WS 都要求租户身份,
   * 平台管理员没有租户(notification 表 tenant_id NOT NULL),因此平台端声明 false:
   * 平台是公告发布方,公告能力在侧栏「系统公告」页,不做收件箱。
   * 由各区导航配置显式声明,壳层不在运行时判角色枚举(见目录设计铁律 2)。
   */
  hasNotificationInbox: boolean
  groups: RoleNavigationGroup[]
  /** 没有独立侧栏项的深页，仍声明模块归属；路径只随本角色块加载。 */
  modulePaths?: { path: string; module: TenantModule }[]
}

/** requiredNavigationModule 按完整路径段匹配列表、详情和沉浸工作台的同一归属。 */
export function requiredNavigationModule(
  config: RoleNavigationConfig,
  pathname: string
): TenantModule | undefined {
  const paths = [...config.groups.flatMap((group) => group.items), ...(config.modulePaths ?? [])]
  return paths.find((item) => pathname === item.path || pathname.startsWith(`${item.path}/`))
    ?.module
}

/** availableRoleNavigation 只使用 /me 的授权结果，隐藏关闭入口及空组并选择可用首页。 */
export function availableRoleNavigation(
  config: RoleNavigationConfig,
  enabled: TenantModule[]
): RoleNavigationConfig {
  const groups = config.groups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => !item.module || enabled.includes(item.module)),
    }))
    .filter((group) => group.items.length > 0)
  return {
    ...config,
    groups,
    homePath: groups[0]?.items[0]?.path ?? `${config.pathPrefix}/profile`,
  }
}
