// AuthLayout 公共认证壳(底层全裸露形态,FE-6):
// 墨色底层 + 横贯全宽的页脚。
// 子页面(登录/多校选择/强制改密)直接生长在底层上,无卡片壳。

import { Outlet } from 'react-router'

/**
 * AuthLayout 提供认证域的底层氛围与页脚;内容经 Outlet 渲染。
 */
export default function AuthLayout() {
  return (
    <div className="relative flex min-h-dvh flex-col overflow-hidden bg-substrate text-on-dark">
      <main className="relative z-base flex flex-1">
        <Outlet />
      </main>

      {/* 横贯全宽的页脚把两区收进同一平面 */}
      <footer className="relative z-base border-t border-dark-line/70">
        <div className="flex items-center justify-between gap-6 px-8 py-4 text-xs text-on-dark-sub lg:px-14">
          <span>遇到问题?请联系本校管理员</span>
          <span className="font-mono">Chaimir © {new Date().getFullYear()}</span>
        </div>
      </footer>
    </div>
  )
}
