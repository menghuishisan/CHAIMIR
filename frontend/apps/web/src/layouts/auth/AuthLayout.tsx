// AuthLayout 公共认证壳(底层全裸露形态,FE-6):
// 墨色底层 + 左侧固定介绍区 + 右侧路由内容区 + 横贯全宽的页脚。
// 左侧介绍区对所有认证页可见(登录/忘记密码/入驻申请/激活/SSO 共享);
// 右侧内容区经 Outlet 切换,路由过渡时左侧不动,只有右侧内容淡入淡出(丝滑切换)。
// 子页面(各认证页)不自建左右分栏,只渲染表单内容 —— 分栏由本壳层统一控制。
// 页脚是平台管理员登录的界面入口(仅 SaaS),显示什么由 authFooter.ts 决定。

import { useEffect, useRef, useState } from 'react'
import { Link, Outlet, useLocation } from 'react-router'
import { useReducedMotion } from '@chaimir/ui'
import { platformLayerEnabled } from '../../app/config'
import { AuthIntro } from './AuthIntro'
import { authFooterContent } from './authFooter'

/**
 * AuthLayout 提供认证域的底层氛围、固定左侧介绍区、右侧内容区与页脚。
 * 路由切换时左侧介绍区不动,只有右侧 Outlet 内容切换,带淡入淡出过渡。
 */
export default function AuthLayout() {
  const location = useLocation()
  const [displayLocation, setDisplayLocation] = useState(location)
  const [isTransitioning, setIsTransitioning] = useState(false)
  const timeoutRef = useRef<NodeJS.Timeout | undefined>(undefined)
  const reducedMotion = useReducedMotion()
  const footer = authFooterContent(location.pathname, platformLayerEnabled)

  // 路由变化时触发淡出 → 切换内容 → 淡入
  useEffect(() => {
    if (location !== displayLocation) {
      if (reducedMotion) {
        setDisplayLocation(location)
        setIsTransitioning(false)
        return
      }
      setIsTransitioning(true)
      // 淡出完成后切换内容(200ms 与 animate-fade-in 时长一致)
      timeoutRef.current = setTimeout(() => {
        setDisplayLocation(location)
        setIsTransitioning(false)
      }, 200)
    }
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
    }
  }, [displayLocation, location, reducedMotion])

  return (
    <div className="relative flex min-h-dvh flex-col overflow-hidden bg-substrate text-on-dark">
      <main className="relative z-base flex flex-1 flex-col lg:flex-row">
        {/* ---------- 左区:品牌与介绍(桌面),窄屏折叠为顶部紧凑版 ---------- */}
        <AuthIntro />

        {/* ---------- 右区:认证页内容(Outlet),同一底层,仅暗化渐变过渡,无边线 ---------- */}
        <section className="relative flex basis-2/5 items-center justify-center px-6 py-10 lg:justify-start lg:px-16">
          {/* 右区底层压暗渐变(规范 §1.2:表单侧只是底层被压暗一档) */}
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0 hidden bg-gradient-to-r from-substrate/50 via-dark-bg/60 to-dark-bg lg:block"
          />
          <div className="relative w-full max-w-sm">
            {/* 路由内容切换点,带淡入淡出过渡(丝滑切换) */}
            <div
              className={
                reducedMotion ? undefined : isTransitioning ? 'animate-fade-out' : 'animate-fade-in'
              }
              key={displayLocation.pathname}
            >
              <Outlet />
            </div>
          </div>
        </section>
      </main>

      {/* 横贯全宽的页脚把两区收进同一平面;左侧联系本校管理员(只在面向师生的页面),
          右侧是平台管理员登录入口(仅 SaaS;在平台管理员登录页上换成回学校登录)与版权 */}
      <footer className="relative z-base border-t border-dark-line/70">
        <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 px-6 py-4 text-sm text-on-dark-sub lg:px-14">
          {footer.showSchoolHelp ? <span>遇到问题?请联系本校管理员</span> : <span />}
          <div className="flex items-center gap-5">
            {footer.link ? (
              <Link
                to={footer.link.to}
                className="inline-flex min-h-11 items-center rounded-sm transition-colors duration-fast hover:text-on-dark focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 lg:min-h-0"
              >
                {footer.link.label}
              </Link>
            ) : null}
            <span className="font-mono text-xs">Chaimir © {new Date().getFullYear()}</span>
          </div>
        </div>
      </footer>
    </div>
  )
}
