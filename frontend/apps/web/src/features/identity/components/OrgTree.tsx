// OrgTree 组织结构左栏树(教师组织查看 / 校管组织架构共用,设计 §7.4 / §7.5):上方搜索,下方院系→专业两级。
// 院系与专业都是可选节点(按钮),当前选中的用玉浅底 + aria-current;选中状态与搜索词由页面写进地址。
// 节点尾部的操作(校管的重命名、删除、在院系下新建专业)由调用方通过 renderActions 传入,教师端不传。

import type { ReactNode } from 'react'
import { Building2, GraduationCap } from 'lucide-react'
import type { Department, Major } from '@chaimir/api-client'
import { cn, Icon } from '@chaimir/ui'
import { SearchBox } from '../../../components/SearchBox'
import type { DepartmentNode, OrgSelection } from '../organizationTree'

export interface OrgTreeProps {
  tree: DepartmentNode<Department, Major>[]
  selection: OrgSelection | undefined
  keyword: string
  onSearch: (value: string) => void
  onSelectDepartment: (departmentId: string) => void
  onSelectMajor: (majorId: string) => void
  /** 节点尾部的操作(只在校管端);按钮要自己停止冒泡之外不需要做任何事,节点本身是独立按钮 */
  renderDepartmentActions?: (department: Department) => ReactNode
  renderMajorActions?: (major: Major) => ReactNode
  /** 树为空且没有搜索词时的说明 */
  emptyText: string
  /** 搜索框上方的动作(校管的「新建院系」) */
  action?: ReactNode
}

/** 节点按钮的共同样式:整行可点,悬停有底色,选中时玉浅底 */
const NODE_CLASS =
  'flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors duration-fast focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent'

/** OrgTree 渲染搜索框与院系→专业树 */
export function OrgTree({
  tree,
  selection,
  keyword,
  onSearch,
  onSelectDepartment,
  onSelectMajor,
  renderDepartmentActions,
  renderMajorActions,
  emptyText,
  action,
}: OrgTreeProps) {
  return (
    <nav aria-label="院系与专业" className="flex flex-col gap-3 rounded-lg bg-surface p-3 shadow-xs">
      {action ? <div className="flex items-center justify-between gap-2 px-1">{action}</div> : null}
      <SearchBox label="搜索院系、专业或班级" placeholder="搜索院系、专业或班级" value={keyword} onSearch={onSearch} className="sm:w-full" />
      {tree.length === 0 ? (
        <p className="px-2 py-4 text-sm text-ink-sub">{keyword ? '没有匹配的院系、专业或班级。' : emptyText}</p>
      ) : (
        <ul className="flex flex-col gap-1">
          {tree.map((node) => {
            const departmentSelected = selection?.kind === 'department' && selection.departmentId === node.department.id
            return (
              <li key={node.department.id}>
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => onSelectDepartment(node.department.id)}
                    aria-current={departmentSelected ? 'true' : undefined}
                    className={cn(NODE_CLASS, 'font-medium text-ink', departmentSelected ? 'bg-primary-soft' : 'hover:bg-surface-hover')}
                  >
                    <Icon icon={Building2} size="sm" className="shrink-0 text-ink-sub" />
                    <span className="min-w-0 truncate">{node.department.name}</span>
                  </button>
                  {renderDepartmentActions?.(node.department)}
                </div>
                {node.majors.length > 0 ? (
                  <ul className="ml-4 mt-0.5 flex flex-col gap-0.5 border-l border-line pl-2">
                    {node.majors.map((child) => {
                      const majorSelected = selection?.kind === 'major' && selection.majorId === child.major.id
                      return (
                        <li key={child.major.id} className="flex items-center gap-1">
                          <button
                            type="button"
                            onClick={() => onSelectMajor(child.major.id)}
                            aria-current={majorSelected ? 'true' : undefined}
                            className={cn(NODE_CLASS, 'text-ink', majorSelected ? 'bg-primary-soft' : 'hover:bg-surface-hover')}
                          >
                            <Icon icon={GraduationCap} size="sm" className="shrink-0 text-ink-sub" />
                            <span className="min-w-0 flex-1 truncate">{child.major.name}</span>
                            <span className="shrink-0 text-xs tabular-nums text-ink-sub">{child.classCount} 个班</span>
                          </button>
                          {renderMajorActions?.(child.major)}
                        </li>
                      )
                    })}
                  </ul>
                ) : null}
              </li>
            )
          })}
        </ul>
      )}
    </nav>
  )
}
