// 教学端导航数据(仅教师通过 RoleGuard 后随该区懒加载壳加载,见铁律 2)。

import {
  Activity,
  Book,
  CheckSquare,
  Database,
  FileText,
  LayoutTemplate,
  Network,
  Send,
  Share2,
  Trophy,
  Users,
} from 'lucide-react'
import { ROLE_ROUTES } from '../../utils/roleRouting'
import type { RoleNavigationConfig } from '../../layouts/main/navigation'

export const teacherNavigation: RoleNavigationConfig = {
  ...ROLE_ROUTES.teacher,
  hasNotificationInbox: true,
  groups: [
    {
      title: '教学',
      items: [
        { name: '课程管理', path: '/teacher/courses', icon: Book },
        { name: '批改中心', path: '/teacher/grading', icon: CheckSquare },
      ],
    },
    {
      title: '实践',
      items: [
        { name: '实验编排', path: '/teacher/experiments', icon: LayoutTemplate },
        { name: '赛事组织', path: '/teacher/contests', icon: Trophy },
        { name: '实时监控', path: '/teacher/monitoring', icon: Activity },
      ],
    },
    {
      title: '资源',
      items: [
        { name: '题库内容', path: '/teacher/questions', icon: Database },
        { name: '试卷组卷', path: '/teacher/exams', icon: FileText },
        { name: '仿真场景', path: '/teacher/simulations', icon: Network },
        { name: '共享资源库', path: '/teacher/shared', icon: Share2 },
      ],
    },
    {
      title: '组织与成绩',
      items: [
        { name: '成绩报送', path: '/teacher/grades', icon: Send },
        { name: '组织查看', path: '/teacher/organization', icon: Users },
      ],
    },
  ],
}
