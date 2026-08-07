// authRoutes 认证域路由清单(公共页,底层全裸露):
// 目录设计 §4 铁律 2 —— 路由层只维护清单、懒加载入口与权限装配,不写业务逻辑。
// 页面按后端 identity 的免认证接口一一对应:
//   /auth/login          手机号密码 · 短码学号 · 短信验证码(POST login/phone|no|sms、sms/send)
//   /auth/platform-login 平台管理员特权通道(POST login/platform),仅 SaaS 形态注册
//   /auth/tenant-select  一号多校的租户选择(登录响应返回候选租户时进入)
//   /auth/change-pwd     强制改密(首登/过期)
//   /auth/forgot         找回密码(POST password/reset)
//   /auth/activate       账号激活(POST activate)
//   /auth/apply          学校入驻申请(POST platform/applications,免认证),仅 SaaS 形态
//   /auth/sso/:tenantCode CAS 回跳落地(GET sso/:tenant_code/callback)

import { lazy } from 'react'
import { Navigate, Route } from 'react-router'

/* 懒加载:未登录用户只下载认证域代码,不触碰任何角色端代码碎片 */
const LoginPage = lazy(() => import('../features/identity/pages/auth/login'))
const PlatformLoginPage = lazy(() => import('../features/identity/pages/auth/platform-login'))
const TenantSelectPage = lazy(() => import('../features/identity/pages/auth/tenant-select'))
const ChangePasswordPage = lazy(() => import('../features/identity/pages/auth/change-password'))
const ForgotPasswordPage = lazy(() => import('../features/identity/pages/auth/forgot'))
const ActivatePage = lazy(() => import('../features/identity/pages/auth/activate'))
const ApplyPage = lazy(() => import('../features/identity/pages/auth/apply'))
const SsoCallbackPage = lazy(() => import('../features/identity/pages/auth/sso'))

/**
 * authRoutes 返回认证域子路由。
 * platformEnabled 决定平台特权通道与入驻申请是否存在 —— 私有化部署下这两条路径
 * 根本不注册(后端同样不注册 /platform 路由),而非靠前端隐藏入口。
 */
export function authRoutes(platformEnabled: boolean) {
  return (
    <>
      <Route index element={<Navigate to="login" replace />} />
      <Route path="login" element={<LoginPage />} />
      <Route path="tenant-select" element={<TenantSelectPage />} />
      <Route path="change-pwd" element={<ChangePasswordPage />} />
      <Route path="forgot" element={<ForgotPasswordPage />} />
      <Route path="activate" element={<ActivatePage />} />
      <Route path="sso/:tenantCode" element={<SsoCallbackPage />} />
      {platformEnabled ? <Route path="platform-login" element={<PlatformLoginPage />} /> : null}
      {platformEnabled ? <Route path="apply" element={<ApplyPage />} /> : null}
    </>
  )
}
