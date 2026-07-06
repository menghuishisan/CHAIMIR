// 输入校验和脱敏展示：公共认证页和表单页共用。

export function maskPhone(phone: string): string {
  if (phone.length !== 11) return phone
  return `${phone.slice(0, 3)}****${phone.slice(7)}`
}

export function isValidPhone(phone: string): boolean {
  return /^1[3-9]\d{9}$/.test(phone)
}

export function isValidEmail(email: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)
}
