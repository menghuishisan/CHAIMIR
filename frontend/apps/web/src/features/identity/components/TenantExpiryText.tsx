// TenantExpiryText 显示学校服务到期:长期有效 / 到期日期;30 天内到期用警示色并写剩余天数,已过期用危险色(设计 §7.6)。
// 学校管理表格与学校详情页头共用。

import { formatDate } from '../../../utils/formatters'
import { tenantExpiry } from '../tenantExpiry'

/** TenantExpiryText 到期日期 + 提醒;同时用文字表达紧迫程度,不只靠颜色 */
export function TenantExpiryText({ expireAt }: { expireAt?: string }) {
  const expiry = tenantExpiry(expireAt)
  if (expiry.kind === 'none') return <span className="text-ink-sub">长期有效</span>
  if (expiry.kind === 'unknown') return <span className="text-ink-sub">到期日读不出</span>
  const day = formatDate(expireAt ?? '')
  if (expiry.kind === 'expired') {
    return <span className="whitespace-nowrap text-danger">{day} · 已到期</span>
  }
  if (expiry.kind === 'soon') {
    return (
      <span className="whitespace-nowrap text-warning">
        {day} · {expiry.days === 0 ? '今天到期' : `还剩 ${expiry.days} 天`}
      </span>
    )
  }
  return <span className="whitespace-nowrap text-ink">{day}</span>
}
