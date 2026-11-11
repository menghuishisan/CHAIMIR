// passwordRules.ts 是密码强度规则的唯一定义:与后端 identity ValidatePassword 同一口径 ——
// 至少 8 位,并且同时含英文字母(A-Z/a-z)与数字(0-9)。
// 逐条结果供设置密码时的实时勾选清单使用,整体结果供提交前校验使用(authForm.isPasswordStrong)。
// 纯函数,供 node:test 直接运行。

/** 三条规则各自是否满足 */
export interface PasswordRuleStates {
  minLength: boolean
  letter: boolean
  digit: boolean
}

/** 密码最小长度:与后端一致 */
export const PASSWORD_MIN_LENGTH = 8

/** passwordRuleStates 逐条判断密码规则 */
export function passwordRuleStates(value: string): PasswordRuleStates {
  return {
    minLength: value.length >= PASSWORD_MIN_LENGTH,
    letter: /[A-Za-z]/.test(value),
    digit: /\d/.test(value),
  }
}

/** 清单里每条规则的用户向说法,顺序即显示顺序 */
export const PASSWORD_RULE_TEXT: { key: keyof PasswordRuleStates; text: string }[] = [
  { key: 'minLength', text: `至少 ${PASSWORD_MIN_LENGTH} 位` },
  { key: 'letter', text: '包含英文字母' },
  { key: 'digit', text: '包含数字' },
]
