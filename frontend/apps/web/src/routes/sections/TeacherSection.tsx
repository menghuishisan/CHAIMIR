// 教学端区懒加载壳:本文件与 teacherNavigation 同属一个打包块,
// 只有通过 RoleGuard 的教师才会下载(铁律 2)。区内页面清单在阶段 4 加在下方 MainLayout 子级。

import { Route, Routes } from 'react-router-dom'
import { MainLayout } from '../../layouts/main/MainLayout'
import { RoleNotFoundPage } from '../../components/StatusPages'
import { teacherNavigation } from './teacherNavigation'

/**
 * TeacherSection 装配教学端区内部路由。
 */
export default function TeacherSection() {
  return (
    <Routes>
      <Route element={<MainLayout config={teacherNavigation} />}>
        <Route path="*" element={<RoleNotFoundPage homePath={teacherNavigation.homePath} />} />
      </Route>
    </Routes>
  )
}
