// roleNavigation 定义应用级四角色日常导航与顶栏品牌配置。

import type { LucideIcon } from 'lucide-react'
import type { RoleRouteConfig } from '../utils/roleRouting'
import { ROLE_ROUTES } from '../utils/roleRouting'
import {
  Activity,
  AlertTriangle,
  BellRing,
  Book,
  BookOpen,
  Bug,
  Building,
  CheckCircle,
  CheckSquare,
  Cpu,
  Database,
  FileText,
  FlaskConical,
  GraduationCap,
  Inbox,
  LayoutDashboard,
  LayoutTemplate,
  Monitor,
  Network,
  Package,
  Save,
  Scale,
  Send,
  Server,
  Settings,
  Settings2,
  Share2,
  Shield,
  Swords,
  Trophy,
  Users,
} from 'lucide-react'

export interface RoleNavigationItem {
  name: string
  path: string
  icon: LucideIcon
}

export interface RoleNavigationGroup {
  title: string
  items: RoleNavigationItem[]
}

export interface RoleNavigationConfig extends RoleRouteConfig {
  brandName: string
  groups: RoleNavigationGroup[]
}

const ROLE_NAVIGATION: RoleNavigationConfig[] = [
  {
    ...ROLE_ROUTES.platformAdmin,
    brandName: 'Chaimir 平台管理',
    groups: [
      { title: '租户', items: [{ name: '学校管理', path: '/platform-admin/schools', icon: Building }, { name: '入驻申请', path: '/platform-admin/applications', icon: Inbox }] },
      { title: '运营', items: [{ name: '平台看板', path: '/platform-admin/dashboard', icon: LayoutDashboard }] },
      {
        title: '底层资源',
        items: [
          { name: '链运行时', path: '/platform-admin/runtimes', icon: Server },
          { name: '沙箱工具', path: '/platform-admin/sandbox-tools', icon: Package },
          { name: '判题器', path: '/platform-admin/judges', icon: Cpu },
          { name: '仿真治理', path: '/platform-admin/simulations', icon: Shield },
          { name: '漏洞题源', path: '/platform-admin/vulnerabilities', icon: Bug },
          { name: '告警中心', path: '/platform-admin/alerts', icon: BellRing },
          { name: '系统配置', path: '/platform-admin/settings', icon: Settings },
          { name: '监控面板', path: '/platform-admin/monitoring', icon: Monitor },
          { name: '备份记录', path: '/platform-admin/backups', icon: Save },
          { name: '平台审计', path: '/platform-admin/audit', icon: FileText },
        ],
      },
    ],
  },
  {
    ...ROLE_ROUTES.schoolAdmin,
    brandName: 'Chaimir 校管端',
    groups: [
      { title: '用户与组织', items: [{ name: '账号管理', path: '/school-admin/users', icon: Users }, { name: '组织架构', path: '/school-admin/organization', icon: Network }] },
      { title: '概览', items: [{ name: '学校看板', path: '/school-admin/dashboard', icon: LayoutDashboard }] },
      { title: '教务与成绩', items: [{ name: '成绩审核', path: '/school-admin/approvals', icon: CheckCircle }, { name: '申诉处理', path: '/school-admin/appeals', icon: Scale }, { name: '学业预警', path: '/school-admin/alerts', icon: AlertTriangle }, { name: '成绩配置', path: '/school-admin/grade-settings', icon: Settings2 }] },
      { title: '系统配置', items: [{ name: '租户配置', path: '/school-admin/settings', icon: Settings }, { name: '认证配置', path: '/school-admin/auth-config', icon: Shield }, { name: '审计日志', path: '/school-admin/audit', icon: FileText }, { name: '学校告警', path: '/school-admin/system-alerts', icon: BellRing }] },
    ],
  },
  {
    ...ROLE_ROUTES.teacher,
    brandName: 'Chaimir 教学端',
    groups: [
      { title: '教学 TEACHING', items: [{ name: '课程管理', path: '/teacher/courses', icon: Book }, { name: '批改中心', path: '/teacher/grading', icon: CheckSquare }] },
      { title: '实践 PRACTICE', items: [{ name: '实验编排', path: '/teacher/experiments', icon: LayoutTemplate }, { name: '赛事组织', path: '/teacher/contests', icon: Trophy }, { name: '实时监控', path: '/teacher/monitoring', icon: Activity }] },
      { title: '资源 RESOURCES', items: [{ name: '题库内容', path: '/teacher/questions', icon: Database }, { name: '试卷组卷', path: '/teacher/exams', icon: FileText }, { name: '漏洞题源转化', path: '/teacher/vulnerabilities', icon: AlertTriangle }, { name: '仿真场景', path: '/teacher/simulations', icon: Network }, { name: '共享资源库', path: '/teacher/shared', icon: Share2 }] },
      { title: '组织与成绩 GRADES', items: [{ name: '成绩报送', path: '/teacher/grades', icon: Send }, { name: '组织查看', path: '/teacher/organization', icon: Users }] },
    ],
  },
  {
    ...ROLE_ROUTES.student,
    brandName: 'Chaimir 学台',
    groups: [
      { title: '学习区 LEARNING', items: [{ name: '课程', path: '/student/courses', icon: BookOpen }, { name: '实验', path: '/student/experiments', icon: FlaskConical }, { name: '仿真', path: '/student/simulations', icon: Network }, { name: '参赛', path: '/student/contests', icon: Swords }, { name: '战绩', path: '/student/records', icon: Trophy }] },
      { title: '学业区 PERFORMANCE', items: [{ name: '成绩', path: '/student/grades', icon: GraduationCap }, { name: '预警', path: '/student/alerts', icon: AlertTriangle }] },
    ],
  },
]

/** roleNavigationForPath 根据已鉴权角色路径返回唯一导航配置。 */
export function roleNavigationForPath(pathname: string): RoleNavigationConfig {
  const navigation = ROLE_NAVIGATION.find((config) => pathname === config.pathPrefix || pathname.startsWith(`${config.pathPrefix}/`))
  if (!navigation) {
    throw new Error(`未找到角色导航配置: ${pathname}`)
  }
  return navigation
}
