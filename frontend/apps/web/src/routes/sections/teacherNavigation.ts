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
import { TENANT_MODULE } from '@chaimir/api-client'

export const teacherNavigation: RoleNavigationConfig = {
  ...ROLE_ROUTES.teacher,
  hasNotificationInbox: true,
  modulePaths: [{ path: '/teacher/vuln-workshop', module: TENANT_MODULE.CONTEST }],
  groups: [
    {
      title: '教学',
      items: [
        { name: '课程管理', path: '/teacher/courses', icon: Book, module: TENANT_MODULE.TEACHING },
        {
          name: '批改中心',
          path: '/teacher/grading',
          icon: CheckSquare,
          module: TENANT_MODULE.TEACHING,
        },
      ],
    },
    {
      title: '实践',
      items: [
        {
          name: '实验编排',
          path: '/teacher/experiments',
          icon: LayoutTemplate,
          module: TENANT_MODULE.EXPERIMENT,
        },
        {
          name: '赛事组织',
          path: '/teacher/contests',
          icon: Trophy,
          module: TENANT_MODULE.CONTEST,
        },
        { name: '实时监控', path: '/teacher/monitoring', icon: Activity },
      ],
    },
    {
      title: '资源',
      items: [
        { name: '题库内容', path: '/teacher/questions', icon: Database },
        { name: '试卷组卷', path: '/teacher/exams', icon: FileText },
        {
          name: '仿真场景',
          path: '/teacher/simulations',
          icon: Network,
          module: TENANT_MODULE.SIM,
        },
        { name: '共享资源库', path: '/teacher/shared', icon: Share2 },
      ],
    },
    {
      title: '组织与成绩',
      items: [
        { name: '成绩报送', path: '/teacher/grades', icon: Send, module: TENANT_MODULE.GRADE },
        { name: '组织查看', path: '/teacher/organization', icon: Users },
      ],
    },
  ],
}
