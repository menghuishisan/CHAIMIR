// ChangePasswordGate：首登改密的登录前提示页。

import React from 'react'
import { AuthBlock } from '../components/AuthBlock'

/**
 * ChangePasswordGate 说明首登强制改密应在已登录态内完成，避免重复实现鉴权流程。
 */
export function ChangePasswordGate(): React.ReactElement {
  return (
    <AuthBlock title="需要更新密码" description="为保护账号安全，请进入个人中心修改密码后继续使用。" state={{ values: {}, loading: false }}>
      <a className="public-card-link" href="#login">重新登录后前往个人中心</a>
    </AuthBlock>
  )
}
