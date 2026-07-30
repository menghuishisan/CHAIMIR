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

export const studentNavigation: RoleNavigationConfig = {
  ...ROLE_ROUTES.student,
  brandName: 'Chaimir 学台',
  hasNotificationInbox: true,
  groups: [
    {
      title: '学习区',
      items: [
        { name: '我的课程', path: '/student/courses', icon: BookOpen },
        { name: '实验实训', path: '/student/experiments', icon: FlaskConical },
        { name: '仿真实验室', path: '/student/simulations', icon: Network },
        { name: '竞赛参赛', path: '/student/contests', icon: Swords },
        { name: '竞赛战绩', path: '/student/records', icon: Trophy },
      ],
    },
    {
      title: '学业区',
      items: [
        { name: '成绩中心', path: '/student/grades', icon: GraduationCap },
        { name: '学业预警', path: '/student/alerts', icon: TriangleAlert },
      ],
    },
  ],
}
