// LoginPage 公共登录页(墨玉体系,底层全裸露形态):
// 左区介绍已移至 AuthLayout 作为固定左栏(对所有认证页可见);本页只渲染右区登录表单。
// 窄屏时左区折叠,需要在表单顶部补充紧凑品牌行(lg:hidden)。
// 本文件只做「当前是哪种登录方式」的取舍:
//   输入与提交流程在 login-state(三种方式共用一套状态),各方式的表单在 login-forms。
// 多学校账号经内存暂存凭证进入 /auth/tenant-select(凭证不进路由 state,防历史记录持久化)。

import { useTenantBrand } from '../../useTenantBrand'
import { AuthBrandMark, SchoolBrandLine } from './auth-ui'
import { AccountLoginForm, PhoneLoginForm, SsoLoginForm } from './login-forms'
import { useLoginController } from './login-state'

/**
 * LoginPage 按当前登录方式渲染对应表单。
 * 左侧介绍区已提升至 AuthLayout,本页不再自建分栏。
 */
export default function LoginPage() {
  const login = useLoginController()
  const brand = useTenantBrand()

  return (
    <>
      {/* 窄屏品牌行(左区隐藏时的紧凑替代) */}
      <div className="mb-10 flex flex-col gap-3 lg:hidden">
        <AuthBrandMark />
        {brand ? <SchoolBrandLine name={brand.display_name} logoSrc={brand.logo_image} /> : null}
      </div>

      {login.view === 'phone' ? (
        <PhoneLoginForm login={login} />
      ) : login.view === 'account' ? (
        <AccountLoginForm login={login} />
      ) : (
        <SsoLoginForm login={login} />
      )}
    </>
  )
}
