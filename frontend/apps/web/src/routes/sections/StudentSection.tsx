// 学台区懒加载壳:本文件与 studentNavigation 同属一个打包块,
// 只有通过 RoleGuard 的学生才会下载(铁律 2)。区内页面清单在阶段 4 加在下方 MainLayout 子级,
// 沉浸态(实验/仿真/答题)在阶段 5 与 MainLayout 并列注册 —— 并列而非嵌套,
// 退出沉浸时才能回到本角色区而非站点根(审查 S1)。

import { Route, Routes } from 'react-router-dom'
import { MainLayout } from '../../layouts/main/MainLayout'
import { RoleNotFoundPage } from '../../components/StatusPages'
import { studentNavigation } from './studentNavigation'

/**
 * StudentSection 装配学台区内部路由。
 * 区内路径写在后代 Routes 里(父级以 /student/* 挂载),
 * 因此这些路径只存在于本块,不会进入入口包。
 */
export default function StudentSection() {
  return (
    <Routes>
      <Route element={<MainLayout config={studentNavigation} />}>
        <Route path="*" element={<RoleNotFoundPage homePath={studentNavigation.homePath} />} />
      </Route>
    </Routes>
  )
}
