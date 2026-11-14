// SimVersionMenu 仿真工作台场景名旁的版本切换(设计 §7.2:「换一组条件」与版本切换放在场景名旁)。
// 只用于自由推演(没有会话):带会话的推演由课程实验指定版本,不能换。
// 列出该场景已发布、可在浏览器里运行的版本;切换就是换地址里的 version 参数,刷新后仍是那个版本。
// 菜单打开时才读版本列表,不为一个不点的菜单打接口。

import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Check, History } from 'lucide-react'
import { SIM_COMPUTE, SIM_PACKAGE_STATUS, type SimPackageMeta } from '@chaimir/api-client'
import { IconButton, Menu, MenuContent, MenuItem, MenuLabel, MenuTrigger } from '@chaimir/ui'
import { api } from '../../../app/api'
import { formatDate } from '../../../utils/formatters'
import { errorDiagnostics } from '../../../utils/userFacingError'

export interface SimVersionMenuProps {
  packageCode: string
  /** 当前版本(地址里的 version) */
  version: string
}

/** SimVersionMenu 渲染一个切换场景版本的小菜单 */
export function SimVersionMenu({ packageCode, version }: SimVersionMenuProps) {
  const navigate = useNavigate()
  const [versions, setVersions] = useState<SimPackageMeta[]>()
  const [failed, setFailed] = useState(false)

  /** load 第一次打开菜单时读取版本列表;失败只在菜单里说明,不打扰推演 */
  const load = () => {
    if (versions !== undefined) return
    setFailed(false)
    api.sim
      .getPackageVersions(packageCode)
      .then((list) =>
        setVersions(
          list.filter((item) => item.status === SIM_PACKAGE_STATUS.PUBLISHED && item.compute === SIM_COMPUTE.BROWSER),
        ),
      )
      .catch((error) => {
        setFailed(true)
        console.warn('仿真场景版本列表读取失败', { operation: 'sim.workspace.versions', error: errorDiagnostics(error) })
      })
  }

  return (
    <Menu onOpenChange={(open) => (open ? load() : undefined)}>
      <MenuTrigger asChild>
        <IconButton variant="on-dark" size="sm" icon={History} aria-label="切换场景版本" title="切换场景版本" />
      </MenuTrigger>
      <MenuContent align="start">
        <MenuLabel>场景版本</MenuLabel>
        {failed ? (
          <MenuItem disabled>版本列表暂时读不到,稍后再试</MenuItem>
        ) : versions === undefined ? (
          <MenuItem disabled>正在读取…</MenuItem>
        ) : versions.length === 0 ? (
          <MenuItem disabled>没有其他可用版本</MenuItem>
        ) : (
          versions.map((item) => (
            <MenuItem
              key={item.version}
              icon={item.version === version ? Check : undefined}
              disabled={item.version === version}
              onSelect={() =>
                navigate(
                  `/student/simulations/${encodeURIComponent(packageCode)}/workspace?version=${encodeURIComponent(item.version)}`,
                  { replace: true },
                )
              }
            >
              <span className="font-mono">{item.version}</span>
              <span className="ml-auto pl-4 text-xs text-ink-sub">{formatDate(item.updated_at)}</span>
            </MenuItem>
          ))
        )}
      </MenuContent>
    </Menu>
  )
}
