// 校管端区懒加载壳:本文件与 schoolAdminNavigation 同属一个打包块,
// 只有通过 RoleGuard 的学校管理员才会下载(铁律 2)。区内页面清单在阶段 4 加在下方 MainLayout 子级。

import { Route, Routes } from 'react-router-dom'
import { MainLayout } from '../../layouts/main/MainLayout'
import { RoleNotFoundPage } from '../../components/StatusPages'
import { schoolAdminNavigation } from './schoolAdminNavigation'

/**
 * SchoolAdminSection 装配校管端区内部路由。
 */
export default function SchoolAdminSection() {
  return (
    <Routes>
      <Route element={<MainLayout config={schoolAdminNavigation} />}>
        <Route path="*" element={<RoleNotFoundPage homePath={schoolAdminNavigation.homePath} />} />
      </Route>
    </Routes>
  )
}
