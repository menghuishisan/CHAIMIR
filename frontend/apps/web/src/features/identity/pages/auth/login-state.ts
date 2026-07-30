// login-state 是登录页的状态机:三种登录方式共用的输入、就近错误、提交与落点判定。
// 与呈现分开的理由是这些规则跨视图共享 —— 密码在手机号与校内账号两种方式间沿用,
// 学校代号在校内账号与统一认证间沿用,切换方式只清错误不清输入,
// 这些行为写在表单组件里就会随视图各长一套。表单长什么样见 login.tsx。

import { useCallback, useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import type { LoginResponse, TenantOption } from '@chaimir/api-client'
import { SmsScene } from '@chaimir/api-client'
import { api } from '../../../../app/api'
import { loginEntryPath, persistLoginTokens } from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import {
  passwordRequiredError,
  phoneError,
  requiredError,
  useFieldErrors,
  useSmsCooldown,
} from './auth-form'
import { clearPendingTenantLogin, setPendingTenantLogin } from './pendingLogin'

/** LoginView 登录方式:手机号、校内账号、学校统一认证 */
export type LoginView = 'phone' | 'account' | 'sso'

/** PhoneLoginMethod 手机号登录的凭据类型 */
export type PhoneLoginMethod = 'password' | 'sms'

export interface LoginController {
  view: LoginView
  /** switchView 换登录方式:清空错误但保留已填内容 */
  switchView: (next: LoginView) => void
  phoneMethod: PhoneLoginMethod
  setPhoneMethod: (method: PhoneLoginMethod) => void

  phone: string
  setPhone: (value: string) => void
  password: string
  setPassword: (value: string) => void
  smsCode: string
  setSmsCode: (value: string) => void
  tenantCode: string
  setTenantCode: (value: string) => void
  accountNo: string
  setAccountNo: (value: string) => void
  remember: boolean
  setRemember: (value: boolean) => void

  /** errors 就近字段错误;blurXxx 在离开字段时写入 */
  errors: Record<string, string | null>
  blurPhone: () => void
  blurPassword: () => void
  blurSmsCode: () => void
  blurTenantCode: () => void
  blurAccountNo: () => void

  /** formError 提交/发送失败的用户向说明(后端裁决结果) */
  formError: string | null
  submitting: boolean
  sendingSms: boolean
  /** smsCooldown 距离可重发验证码的剩余秒数 */
  smsCooldown: number
  sendSms: () => void

  submitPhone: (event: FormEvent<HTMLFormElement>) => void
  submitAccount: (event: FormEvent<HTMLFormElement>) => void
  submitSso: (event: FormEvent<HTMLFormElement>) => void
}

/**
 * useLoginController 持有登录页全部输入与提交流程。
 * 登录成功后的落点由服务端返回的角色与改密要求决定(loginEntryPath),前端不自行判定权限。
 */
export function useLoginController(): LoginController {
  const navigate = useNavigate()
  const location = useLocation()
  const returnPath = (location.state as { from?: string } | null)?.from

  const [view, setView] = useState<LoginView>('phone')
  const [phoneMethod, setPhoneMethod] = useState<PhoneLoginMethod>('password')
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [smsCode, setSmsCode] = useState('')
  const [tenantCode, setTenantCode] = useState('')
  const [accountNo, setAccountNo] = useState('')
  const [remember, setRemember] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [sendingSms, setSendingSms] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const { errors, setError, clearErrors } = useFieldErrors()
  const { seconds: smsCooldown, start: startSmsCooldown } = useSmsCooldown()

  // 进入登录页即清除上一轮多校选择暂存的凭证
  useEffect(() => {
    clearPendingTenantLogin()
  }, [])

  const blurPhone = useCallback(() => {
    setError('phone', phoneError(phone, '手机号'))
  }, [phone, setError])

  const blurPassword = useCallback(() => {
    setError('password', passwordRequiredError(password, '请输入密码'))
  }, [password, setError])

  const blurSmsCode = useCallback(() => {
    setError('smsCode', requiredError(smsCode, '请输入验证码'))
  }, [setError, smsCode])

  const blurTenantCode = useCallback(() => {
    setError('tenantCode', requiredError(tenantCode, '请输入学校代号'))
  }, [setError, tenantCode])

  const blurAccountNo = useCallback(() => {
    setError('accountNo', requiredError(accountNo, '请输入学号或工号'))
  }, [accountNo, setError])

  /** completeLogin 持久化会话并按服务端改密要求或角色决定落点 */
  const completeLogin = useCallback(
    (response: LoginResponse) => {
      clearPendingTenantLogin()
      persistLoginTokens(response, remember)
      navigate(loginEntryPath(response, returnPath), { replace: true })
    },
    [navigate, remember, returnPath],
  )

  /** enterTenantSelect 暂存凭证进入多学校选择(凭证仅存内存) */
  const enterTenantSelect = useCallback(
    (tenants: TenantOption[], method: Parameters<typeof setPendingTenantLogin>[0]['method']) => {
      setPendingTenantLogin({ tenants, remember, returnPath, method })
      navigate('/auth/tenant-select')
    },
    [navigate, remember, returnPath],
  )

  /** sendSms 发送登录验证码并启动重发倒计时 */
  const sendSms = useCallback(async () => {
    if (!setError('phone', phoneError(phone, '手机号'))) return
    setSendingSms(true)
    setFormError(null)
    try {
      await api.identity.sendSMS({ phone: phone.trim(), scene: SmsScene.LOGIN })
      startSmsCooldown()
    } catch (sendError) {
      setFormError(userFacingErrorMessage(sendError, '验证码发送失败,请稍后重试。'))
    } finally {
      setSendingSms(false)
    }
  }, [phone, setError, startSmsCooldown])

  /** submitPhone 手机号登录(密码或验证码两种凭据);一号多校时转入选校页 */
  const submitPhone = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const phoneOk = setError('phone', phoneError(phone, '手机号'))
      const credentialOk =
        phoneMethod === 'password'
          ? setError('password', passwordRequiredError(password, '请输入密码'))
          : setError('smsCode', requiredError(smsCode, '请输入验证码'))
      if (!phoneOk || !credentialOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        if (phoneMethod === 'password') {
          const response = await api.identity.loginPhone({ phone: phone.trim(), password })
          if (response.need_select_tenant && response.tenants && response.tenants.length > 0) {
            enterTenantSelect(response.tenants, { type: 'phone', phone: phone.trim(), password })
            return
          }
          completeLogin(response)
        } else {
          const response = await api.identity.loginSMS({ phone: phone.trim(), code: smsCode.trim() })
          if (response.need_select_tenant && response.tenants && response.tenants.length > 0) {
            enterTenantSelect(response.tenants, { type: 'sms', phone: phone.trim(), code: smsCode.trim() })
            return
          }
          completeLogin(response)
        }
      } catch (loginError) {
        setFormError(
          userFacingErrorMessage(
            loginError,
            phoneMethod === 'password' ? '登录失败,请检查手机号和密码后重试。' : '登录失败,请检查验证码后重试。',
          ),
        )
      } finally {
        setSubmitting(false)
      }
    },
    [completeLogin, enterTenantSelect, password, phone, phoneMethod, setError, smsCode],
  )

  /** submitAccount 学校代号 + 学号/工号登录(学校已定,不存在选校分支) */
  const submitAccount = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const tenantOk = setError('tenantCode', requiredError(tenantCode, '请输入学校代号'))
      const noOk = setError('accountNo', requiredError(accountNo, '请输入学号或工号'))
      const passwordOk = setError('password', passwordRequiredError(password, '请输入密码'))
      if (!tenantOk || !noOk || !passwordOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        const response = await api.identity.loginNo({
          tenant_code: tenantCode.trim(),
          no: accountNo.trim(),
          password,
        })
        completeLogin(response)
      } catch (loginError) {
        setFormError(userFacingErrorMessage(loginError, '登录失败,请检查学校代号、账号和密码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [accountNo, completeLogin, password, setError, tenantCode],
  )

  /** submitSso 进入该学校的统一认证中转页(认证由学校侧完成) */
  const submitSso = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!setError('tenantCode', requiredError(tenantCode, '请输入学校代号'))) return
      navigate(`/auth/sso/${encodeURIComponent(tenantCode.trim())}`)
    },
    [navigate, setError, tenantCode],
  )

  /** switchView 切换登录方式表单,清空上一种方式留下的错误 */
  const switchView = useCallback(
    (next: LoginView) => {
      setView(next)
      setFormError(null)
      clearErrors()
    },
    [clearErrors],
  )

  return {
    view,
    switchView,
    phoneMethod,
    setPhoneMethod,
    phone,
    setPhone,
    password,
    setPassword,
    smsCode,
    setSmsCode,
    tenantCode,
    setTenantCode,
    accountNo,
    setAccountNo,
    remember,
    setRemember,
    errors,
    blurPhone,
    blurPassword,
    blurSmsCode,
    blurTenantCode,
    blurAccountNo,
    formError,
    submitting,
    sendingSms,
    smsCooldown,
    sendSms,
    submitPhone,
    submitAccount,
    submitSso,
  }
}
