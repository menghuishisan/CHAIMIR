// LoginPage 公共登录页(墨玉体系,底层全裸露形态):
// 左区为面向用户的平台介绍(非对称 ~58%,见 login-intro),右区登录表单(无卡片盒,底线输入),
// 两区之间无分隔线,靠暗化渐变过渡(FE-6/规范 §1.2)。
// 本文件只做两区布局与「当前是哪种登录方式」的取舍:
//   输入与提交流程在 login-state(三种方式共用一套状态),各方式的表单在 login-forms。
// 多学校账号经内存暂存凭证进入 /auth/tenant-select(凭证不进路由 state,防历史记录持久化)。

import { AuthBrandMark } from './auth-ui'
import { AccountLoginForm, PhoneLoginForm, SsoLoginForm } from './login-forms'
import { LoginIntro } from './login-intro'
import { useLoginController } from './login-state'

/**
 * LoginPage 组合介绍区与表单区,并按当前登录方式渲染对应表单。
 */
export default function LoginPage() {
  const login = useLoginController()

  return (
    <div className="flex w-full flex-1 flex-col lg:flex-row">
      {/* ---------- 左区:品牌与介绍(桌面),窄屏折叠为顶部紧凑版 ---------- */}
      <LoginIntro />

      {/* ---------- 右区:登录表单(同一底层,仅暗化渐变过渡,无边线) ---------- */}
      <section className="relative flex flex-1 items-center justify-center px-6 py-10 lg:justify-start lg:px-16">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 hidden bg-gradient-to-r from-transparent via-substrate/40 to-substrate/70 lg:block"
        />
        <div className="relative w-full max-w-sm">
          {/* 窄屏品牌行(左区隐藏时的紧凑替代) */}
          <div className="mb-10 lg:hidden">
            <AuthBrandMark />
          </div>

          {login.view === 'phone' ? (
            <PhoneLoginForm login={login} />
          ) : login.view === 'account' ? (
            <AccountLoginForm login={login} />
          ) : (
            <SsoLoginForm login={login} />
          )}
        </div>
      </section>
    </div>
  )
}
