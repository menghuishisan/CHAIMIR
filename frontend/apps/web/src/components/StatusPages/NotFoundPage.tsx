// NotFoundPage(全局 404):角色区之外的未知路径落点(角色区内另有保留导航壳的 404 页)。

import { useNavigate } from 'react-router-dom'
import { Compass } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { AppStatusScreen } from '../AppStatusScreen'
import { getStoredAccessToken } from '../../utils/authSession'

/**
 * NotFoundPage 引导用户回到登录页或继续使用平台。
 */
export function NotFoundPage() {
  const navigate = useNavigate()
  const hasSession = Boolean(getStoredAccessToken())

  return (
    <AppStatusScreen
      icon={Compass}
      title="页面不存在"
      description="你访问的地址不存在或已经移动。"
      actions={
        <Button
          variant="primary"
          onClick={() => navigate(hasSession ? '/' : '/auth/login', { replace: true })}
        >
          {hasSession ? '回到平台' : '前往登录'}
        </Button>
      }
    />
  )
}
