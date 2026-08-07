// auth-form 是认证域表单的校验规则与就近错误状态原语。
// 同一条规则只有一处实现:手机号格式、密码强度、两次输入一致等在八个认证页里口径完全相同,
// 各页各写一遍的后果是规则调整时漏改某几页,提示语也会长出好几种说法。
// 这里只做「填没填、格式对不对」的就近校验(规范 §5.2 校验时机),
// 账号是否存在、验证码是否有效、激活码是否在有效期内一律由服务端裁决,前端不复刻。

import { useCallback, useEffect, useState } from 'react'

/** 手机号格式:与后端 identity 的手机号口径一致(11 位、1 开头) */
const PHONE_PATTERN = /^1\d{10}$/

/** 邮箱格式:只判「像不像邮箱」,能否收到由平台运营人员实际联系时确认 */
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const MAX_EMAIL_LENGTH = 254

/** 短信验证码的重发冷却秒数:与后端 identity 的发送频控口径一致 */
const SMS_RESEND_SECONDS = 60

/** 密码类字段在文案里的称呼:首次设置叫「密码」,改密与重置叫「新密码」 */
type PasswordNoun = '密码' | '新密码'

/**
 * requiredError 校验标识类必填项(手机号、学校代号、学号、验证码、激活码等)。
 * 去空白后判空:这类值提交时同样会去空白,只输空格等于没填。
 */
export function requiredError(value: string, message: string): string | null {
  return value.trim() ? null : message
}

/**
 * passwordRequiredError 校验密码类必填项。
 * 密码不去空白:首尾空格是密码本身的字符,按空白判空会把合法密码判成没填,
 * 提交时也原样发送(见各页 api.identity.* 调用)。
 */
export function passwordRequiredError(value: string, message: string): string | null {
  return value ? null : message
}

/**
 * phoneError 校验手机号格式;label 决定文案里的字段名(登录用「手机号」,入驻申请用「联系电话」)。
 */
export function phoneError(value: string, label: string): string | null {
  const phone = value.trim()
  if (!phone) return `请输入${label}`
  if (!PHONE_PATTERN.test(phone)) return `${label}格式不正确,请检查后重试`
  return null
}

/**
 * emailError 校验邮箱格式(入驻申请的联系邮箱)。
 */
export function emailError(value: string): string | null {
  const email = value.trim()
  if (!email) return '请输入联系邮箱'
  if (email.length > MAX_EMAIL_LENGTH) return '邮箱格式不正确,请检查后重试'
  if (!EMAIL_PATTERN.test(email)) return '邮箱格式不正确,请检查后重试'
  return null
}

/**
 * passwordRuleError 校验新设密码的强度;规则与后端 ValidatePassword 一致:至少 8 位且同时含字母和数字。
 */
export function passwordRuleError(value: string, noun: PasswordNoun): string | null {
  if (!value) return `请输入${noun}`
  if (value.length < 8 || !/[A-Za-z]/.test(value) || !/\d/.test(value)) {
    return `${noun}至少 8 位,并同时包含字母和数字`
  }
  return null
}

/**
 * confirmPasswordError 校验两次输入是否一致。
 */
export function confirmPasswordError(
  confirm: string,
  password: string,
  noun: PasswordNoun,
): string | null {
  if (!confirm) return `请再次输入${noun}`
  if (confirm !== password) return `两次输入的${noun}不一致,请重新确认`
  return null
}

export interface FieldErrorState {
  /** errors 各字段当前的就近错误;值为 null 表示该字段已通过 */
  errors: Record<string, string | null>
  /** setError 记录或清除一个字段的错误,并返回该字段是否通过 —— 提交时可直接汇总各字段结果 */
  setError: (key: string, message: string | null) => boolean
  /** clearErrors 清空全部就近错误(切换登录方式等换表单场景) */
  clearErrors: () => void
}

/**
 * useFieldErrors 管理就近字段错误:blur 时校验单个字段,提交时校验全部并把错误留在原位。
 */
export function useFieldErrors(): FieldErrorState {
  const [errors, setErrors] = useState<Record<string, string | null>>({})

  const setError = useCallback((key: string, message: string | null): boolean => {
    setErrors((previous) => ({ ...previous, [key]: message }))
    return message === null
  }, [])

  const clearErrors = useCallback(() => setErrors({}), [])

  return { errors, setError, clearErrors }
}

export interface SmsCooldown {
  /** seconds 距离可重发的剩余秒数;为 0 表示此刻可以发送 */
  seconds: number
  /** start 发送成功后启动重发冷却 */
  start: () => void
}

/**
 * useSmsCooldown 管理验证码的重发冷却读秒。
 * 冷却只是把「刚发过、稍等再发」讲给用户,真正的发送频控在服务端;
 * 秒数只在大于 0 时排下一次读秒,故不会读到负数。
 */
export function useSmsCooldown(): SmsCooldown {
  const [seconds, setSeconds] = useState(0)

  useEffect(() => {
    if (seconds <= 0) return
    const timer = window.setTimeout(() => setSeconds((current) => current - 1), 1000)
    return () => window.clearTimeout(timer)
  }, [seconds])

  const start = useCallback(() => setSeconds(SMS_RESEND_SECONDS), [])

  return { seconds, start }
}
