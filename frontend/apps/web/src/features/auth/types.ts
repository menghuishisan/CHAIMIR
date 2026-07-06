// 认证类型：集中声明登录前公共页面、表单状态和可选登录方式。

export type AuthPage = 'login' | 'forgot' | 'sso' | 'apply' | 'activate' | 'platform-login' | 'change-pwd'
export type LoginMode = 'phone' | 'no' | 'sms'
export type FormValues = Record<string, string>

export interface FormState {
  /** 当前表单字段值。 */
  values: FormValues
  /** 异步提交或验证码发送中的加载状态。 */
  loading: boolean
  /** 用户向成功提示。 */
  message?: string
  /** 用户向错误提示。 */
  error?: string
}
