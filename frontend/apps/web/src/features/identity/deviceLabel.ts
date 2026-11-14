// deviceLabel.ts 把登录会话的浏览器信息串写成用户看得懂的设备名(设计 §6 DeviceLabel:「Windows · Chrome 152」)。
// 解析交给成熟库 bowser(MIT),不自己写正则;完整信息串由调用方放在悬停提示里。
// 只取浏览器主版本:小版本号对「这是不是我」的判断没有帮助,反而让一行变长。

import Bowser from 'bowser'

/** 读不出任何信息时的显示名 */
const UNKNOWN_DEVICE = '未知设备'

/** deviceLabel 把浏览器信息串写成「系统 · 浏览器 主版本」,读不出的部分省略 */
export function deviceLabel(userAgent: string | undefined): string {
  const ua = userAgent?.trim()
  if (!ua) return UNKNOWN_DEVICE
  const parsed = Bowser.parse(ua)
  const os = parsed.os.name?.trim()
  const browser = parsed.browser.name?.trim()
  const major = parsed.browser.version?.split('.')[0]
  const browserText = browser ? (major ? `${browser} ${major}` : browser) : undefined
  const parts = [os, browserText].filter((part): part is string => Boolean(part))
  return parts.length > 0 ? parts.join(' · ') : UNKNOWN_DEVICE
}
