// 平台管理区懒加载壳:本文件与 platformAdminNavigation 同属一个打包块,
// 只有 SaaS 形态下通过 RoleGuard 的平台管理员才会下载 —— 这是铁律 2 的关键落点,
// 未登录访客的入口包里不得出现任何 /platform-admin 路径或后台栏目名。
// 区内页面清单在阶段 4 加在下方 MainLayout 子级。

import { Route, Routes } from 'react-router-dom'
import { MainLayout } from '../../layouts/main/MainLayout'
import { RoleNotFoundPage } from '../../components/StatusPages'
import { platformAdminNavigation } from './platformAdminNavigation'

/**
 * PlatformAdminSection 装配平台管理区内部路由。
 */
export default function PlatformAdminSection() {
  return (
    <Routes>
      <Route element={<MainLayout config={platformAdminNavigation} />}>
        <Route
          path="*"
          element={<RoleNotFoundPage homePath={platformAdminNavigation.homePath} />}
        />
      </Route>
    </Routes>
  )
}
