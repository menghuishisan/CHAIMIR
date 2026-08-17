// 平台管理区导航数据(仅 SaaS 形态、且仅平台管理员通过 RoleGuard 后才会加载本文件)。
// 铁律 2:本文件必须只经该区懒加载壳引用 —— 一旦被入口包静态引用,
// 未登录访客即可读到整份平台后台路由结构。

import {
  BellRing,
  Bug,
  Building,
  Cpu,
  FileText,
  Inbox,
  LayoutDashboard,
  Megaphone,
  Monitor,
  Package,
  Save,
  Server,
  Settings,
  Shield,
} from 'lucide-react'
import { ROLE_ROUTES } from '../../utils/roleRouting'
import type { RoleNavigationConfig } from '../../layouts/main/navigation'

export const platformAdminNavigation: RoleNavigationConfig = {
  ...ROLE_ROUTES.platformAdmin,
  // 平台管理员没有租户,M10 站内信在数据模型层不成立(notification.tenant_id NOT NULL),
  // 因此顶栏不出现通知铃铛;平台是公告发布方,公告能力落在侧栏「系统公告」。
  hasNotificationInbox: false,
  groups: [
    {
      title: '学校管理',
      items: [
        { name: '学校管理', path: '/platform-admin/schools', icon: Building },
        { name: '入驻申请', path: '/platform-admin/applications', icon: Inbox },
      ],
    },
    {
      title: '运营',
      items: [{ name: '平台看板', path: '/platform-admin/dashboard', icon: LayoutDashboard }],
    },
    {
      title: '底层资源',
      items: [
        { name: '链运行时', path: '/platform-admin/runtimes', icon: Server },
        { name: '沙箱工具', path: '/platform-admin/sandbox-tools', icon: Package },
        { name: '判题器', path: '/platform-admin/judges', icon: Cpu },
        { name: '仿真治理', path: '/platform-admin/simulations', icon: Shield },
        { name: '漏洞题源', path: '/platform-admin/vulnerabilities', icon: Bug },
        { name: '告警中心', path: '/platform-admin/alerts', icon: BellRing },
        { name: '系统公告', path: '/platform-admin/announcements', icon: Megaphone },
        { name: '系统配置', path: '/platform-admin/settings', icon: Settings },
        { name: '监控面板', path: '/platform-admin/monitoring', icon: Monitor },
        { name: '备份记录', path: '/platform-admin/backups', icon: Save },
        { name: '平台审计', path: '/platform-admin/audit', icon: FileText },
      ],
    },
  ],
}
