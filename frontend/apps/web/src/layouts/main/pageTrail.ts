// pageTrail.ts 计算顶栏「当前位置」与浏览器标题(规范 §6.2):侧栏入口提供第一层,页面声明其后的层级。
// 纯函数,供 MainLayout 与 node:test 使用;只用 import type,不引入任何运行时依赖。

import type { PageLocationItem } from '../../hooks/pageLocation'

/** 侧栏入口里与位置计算有关的字段 */
export interface TrailNavItem {
  name: string
  path: string
}

/** 浏览器标题里的平台名 */
const PRODUCT_NAME = 'Chaimir'

/** matchesSection 按完整路径段判断 pathname 是否属于某个侧栏入口(/a/b 不匹配 /a/bc) */
function matchesSection(pathname: string, sectionPath: string): boolean {
  return pathname === sectionPath || pathname.startsWith(`${sectionPath}/`)
}

/**
 * resolvePageTrail 给出顶栏要显示的层级。
 * - 侧栏一级页不声明:只显示自己的名称;
 * - 更深的页:侧栏入口作为可点的上级,后接页面声明的层级(对象名未读到时 label 为 undefined,显示占位);
 * - 声明的第一项就是侧栏入口,或页面不属于任何侧栏入口时,按声明原样显示。
 */
export function resolvePageTrail(
  navItems: readonly TrailNavItem[],
  pathname: string,
  declared: readonly PageLocationItem[] | null,
): PageLocationItem[] {
  // 最长前缀胜出:嵌套入口(如 /x/courses 与 /x/courses/settings)归属更具体的那一个
  const section = navItems
    .filter((item) => matchesSection(pathname, item.path))
    .sort((a, b) => b.path.length - a.path.length)[0]

  if (!section) return declared ? [...declared] : []
  if (pathname === section.path && !declared) return [{ label: section.name }]

  const sectionCrumb: PageLocationItem = { label: section.name, to: section.path }
  if (!declared || declared.length === 0) return [sectionCrumb]
  if (declared[0]?.to === section.path) return [...declared]
  return [sectionCrumb, ...declared]
}

/** pageTitle 给出浏览器标题:最后一个已知名字的层级 + 平台名;都未知时只显示平台名 */
export function pageTitle(trail: readonly PageLocationItem[]): string {
  for (let index = trail.length - 1; index >= 0; index -= 1) {
    const label = trail[index]?.label
    if (label) return `${label} · ${PRODUCT_NAME}`
  }
  return PRODUCT_NAME
}
