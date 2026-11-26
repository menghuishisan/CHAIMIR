// AuthLayout 公共认证壳:统一承载品牌介绍、认证内容和页脚。

import { Link, Outlet, useLocation } from 'react-router'
import { useReducedMotion } from '@chaimir/ui'
import { platformLayerEnabled } from '../../app/config'
import { AuthIntro } from './AuthIntro'
import { authFooterContent } from './authFooter'

/** 认证域的统一布局;路由内容用地址作为 key,切换时由 CSS 负责入场动画。 */
export default function AuthLayout() {
  const location = useLocation()
  const reducedMotion = useReducedMotion()
  const footer = authFooterContent(location.pathname, platformLayerEnabled)

  return (
    <div className="relative flex min-h-dvh flex-col overflow-hidden bg-substrate text-on-dark">
      <main className="relative z-base flex flex-1 flex-col lg:flex-row">
        <AuthIntro />
        <section className="relative flex basis-2/5 items-center justify-center px-6 py-10 lg:justify-start lg:px-16">
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0 hidden bg-gradient-to-r from-substrate/50 via-dark-bg/60 to-dark-bg lg:block"
          />
          <div className="relative w-full max-w-sm">
            <div className={reducedMotion ? undefined : 'animate-fade-in'} key={location.pathname}>
              <Outlet />
            </div>
          </div>
        </section>
      </main>

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
