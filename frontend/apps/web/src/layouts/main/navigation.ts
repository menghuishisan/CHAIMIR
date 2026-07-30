// navigation 定义 MainLayout 壳层的导航契约(只有类型,不含任何角色的导航数据)。
// 铁律 2(目录设计 §4):普通用户不得下载其他角色、尤其是管理后台的路由结构,
// 因此导航数据按角色分文件放在 routes/sections/,随该角色区的懒加载壳一起加载。
// 类型在编译期擦除、不产生运行时代码,所以壳层与各区共享本文件不会把路径清单带进入口包。

import type { LucideIcon } from 'lucide-react'
import type { RoleRouteConfig } from '../../utils/roleRouting'

export interface RoleNavigationItem {
  /**
   * 侧栏可见的栏目名(面向用户的自然语言,不用开发术语)。
   * 名称与顺序的唯一真相源是 `docs/前端后端功能对齐清单.md` §1.4 四端侧栏表,必须逐字一致。
   */
  name: string
  path: string
  icon: LucideIcon
}

export interface RoleNavigationGroup {
  /** 分组标题:纯中文短词,不加英文副标 */
  title: string
  items: RoleNavigationItem[]
}

export interface RoleNavigationConfig extends RoleRouteConfig {
  /** 侧栏品牌区与窄屏顶栏显示的端名 */
  brandName: string
  /**
   * 该端是否有站内信收件箱(顶栏通知铃铛)。
   * M10 收件箱、未读数、通知偏好、公告已读和业务 WS 都要求租户身份,
   * 平台管理员没有租户(notification 表 tenant_id NOT NULL),因此平台端声明 false:
   * 平台是公告发布方,公告能力在侧栏「系统公告」页,不做收件箱。
   * 由各区导航配置显式声明,壳层不在运行时判角色枚举(见目录设计铁律 2)。
   */
  hasNotificationInbox: boolean
  groups: RoleNavigationGroup[]
}
