// 校管端导航数据(仅学校管理员通过 RoleGuard 后随该区懒加载壳加载,见铁律 2)。

import {
  TriangleAlert,
  BellRing,
  CheckCircle,
  FileText,
  LayoutDashboard,
  Megaphone,
  Network,
  Scale,
  Settings,
  Settings2,
  Shield,
  Users,
} from 'lucide-react'
import { ROLE_ROUTES } from '../../utils/roleRouting'
import type { RoleNavigationConfig } from '../../layouts/main/navigation'
import { TENANT_MODULE } from '@chaimir/api-client'

export const schoolAdminNavigation: RoleNavigationConfig = {
  ...ROLE_ROUTES.schoolAdmin,
  hasNotificationInbox: true,
  groups: [
    {
      title: '用户与组织',
      items: [
        { name: '账号管理', path: '/school-admin/users', icon: Users },
        { name: '组织架构', path: '/school-admin/organization', icon: Network },
      ],
    },
    {
      title: '概览',
      items: [{ name: '学校看板', path: '/school-admin/dashboard', icon: LayoutDashboard }],
    },
    {
      title: '教务与成绩',
      items: [
        {
          name: '成绩审核',
          path: '/school-admin/approvals',
          icon: CheckCircle,
          module: TENANT_MODULE.GRADE,
        },
        {
          name: '申诉处理',
          path: '/school-admin/appeals',
          icon: Scale,
          module: TENANT_MODULE.GRADE,
        },
        {
          name: '学业预警',
          path: '/school-admin/alerts',
          icon: TriangleAlert,
          module: TENANT_MODULE.GRADE,
        },
        {
          name: '成绩配置',
          path: '/school-admin/grade-settings',
          icon: Settings2,
          module: TENANT_MODULE.GRADE,
        },
      ],
    },
    {
      title: '系统配置',
      items: [
        { name: '租户配置', path: '/school-admin/settings', icon: Settings },
        { name: '认证配置', path: '/school-admin/auth-config', icon: Shield },
        { name: '系统公告', path: '/school-admin/announcements', icon: Megaphone },
        { name: '审计日志', path: '/school-admin/audit', icon: FileText },
        { name: '学校告警', path: '/school-admin/system-alerts', icon: BellRing },
      ],
    },
  ],
}
