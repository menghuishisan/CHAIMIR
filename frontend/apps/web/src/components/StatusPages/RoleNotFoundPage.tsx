// RoleNotFoundPage:角色区内未匹配路径的 404 页(保留导航壳与顶栏,审查 S6 根治点之一)。
// 界面上的可达入口永远指向真实页面,本页只服务手输或外部粘贴的地址。

import { useNavigate } from 'react-router'
import { Compass } from 'lucide-react'
import { Button, Empty } from '@chaimir/ui'

export interface RoleNotFoundPageProps {
  /** 本角色区的首页落点,由所属区的懒加载壳传入(壳层不查全角色清单,见铁律 2) */
  homePath: string
}

/**
 * RoleNotFoundPage 在光面内渲染 404 引导,导航与顶栏保持可用。
 */
export function RoleNotFoundPage({ homePath }: RoleNotFoundPageProps) {
  const navigate = useNavigate()

  return (
    <div className="flex min-h-96 items-center justify-center px-6 py-16">
      <Empty
        icon={Compass}
        title="页面不存在"
        description="地址可能有误,请从左侧导航进入需要的功能,或先回到首页。"
        action={
          <Button variant="primary" onClick={() => navigate(homePath, { replace: true })}>
            回到首页
          </Button>
        }
      />
    </div>
  )
}
