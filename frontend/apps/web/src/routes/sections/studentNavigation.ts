// 学台导航数据(仅学生通过 RoleGuard 后随该区懒加载壳加载,见铁律 2)。

import {
  TriangleAlert,
  BookOpen,
  FlaskConical,
  GraduationCap,
  Network,
  Swords,
  Trophy,
} from 'lucide-react'
import { ROLE_ROUTES } from '../../utils/roleRouting'
import type { RoleNavigationConfig } from '../../layouts/main/navigation'
import { TENANT_MODULE } from '@chaimir/api-client'

export const studentNavigation: RoleNavigationConfig = {
  ...ROLE_ROUTES.student,
  hasNotificationInbox: true,
  groups: [
    {
      title: '学习区',
      items: [
        {
          name: '我的课程',
          path: '/student/courses',
          icon: BookOpen,
          module: TENANT_MODULE.TEACHING,
        },
        {
          name: '实验实训',
          path: '/student/experiments',
          icon: FlaskConical,
          module: TENANT_MODULE.EXPERIMENT,
        },
        {
          name: '仿真实验室',
          path: '/student/simulations',
          icon: Network,
          module: TENANT_MODULE.SIM,
        },
        {
          name: '竞赛参赛',
          path: '/student/contests',
          icon: Swords,
          module: TENANT_MODULE.CONTEST,
        },
        { name: '竞赛战绩', path: '/student/records', icon: Trophy, module: TENANT_MODULE.CONTEST },
      ],
    },
    {
      title: '学业区',
      items: [
        {
          name: '成绩中心',
          path: '/student/grades',
          icon: GraduationCap,
          module: TENANT_MODULE.GRADE,
        },
        {
          name: '学业预警',
          path: '/student/alerts',
          icon: TriangleAlert,
          module: TENANT_MODULE.GRADE,
        },
      ],
    },
  ],
}
