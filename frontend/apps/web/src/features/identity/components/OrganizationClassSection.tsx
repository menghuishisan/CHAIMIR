// identity 组织班级明细组件:统一多角色页面的班级关联映射、筛选与基础列。

import { useId, useMemo, type ReactNode } from 'react'
import { Users } from 'lucide-react'
import type { Class } from '@chaimir/api-client'
import {
  Empty,
  FilterBar,
  FilterField,
  Input,
  PageSection,
  SegmentedControl,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { CLASS_STATUS_LABELS } from '../../../utils/labels/identity'
import { CLASS_STATUS_FILTERS } from '../options'
import { CLASS_STATUS_TONES } from '../statusPresentation'
import type { OrganizationView } from '../organizationView'

/** OrganizationClassRow 把班级关联的专业与院系名称补齐为可展示行。 */
export interface OrganizationClassRow {
  entity: Class
  majorName: string
  departmentName: string
}

interface OrganizationClassSectionProps {
  view: OrganizationView
  keyword: string
  statusFilter: string
  onKeywordChange: (value: string) => void
  onStatusChange: (value: string) => void
  description: (count: number) => string
  emptyDescription: string
  actions?: ReactNode
  extraColumns?: TableColumn<OrganizationClassRow>[]
}

/** OrganizationClassSection 呈现 identity 域内可复用的班级筛选和明细表。 */
export function OrganizationClassSection({
  view,
  keyword,
  statusFilter,
  onKeywordChange,
  onStatusChange,
  description,
  emptyDescription,
  actions,
  extraColumns = [],
}: OrganizationClassSectionProps) {
  const keywordId = useId()
  const departmentNameById = useMemo(
    () => new Map(view.departments.map((department) => [department.id, department.name])),
    [view.departments],
  )
  const majorById = useMemo(() => new Map(view.majors.map((major) => [major.id, major])), [view.majors])

  const rows = useMemo<OrganizationClassRow[]>(() => {
    const trimmed = keyword.trim()
    return view.classes
      .map((entity) => {
        const major = majorById.get(entity.major_id)
        return {
          entity,
          majorName: major ? major.name : '已撤销的专业',
          departmentName: major
            ? (departmentNameById.get(major.department_id) ?? '已撤销的院系')
            : '已撤销的院系',
        }
      })
      .filter((row) => {
        if (statusFilter && String(row.entity.status) !== statusFilter) return false
        if (trimmed === '') return true
        return (
          row.entity.name.includes(trimmed) ||
          row.majorName.includes(trimmed) ||
          row.departmentName.includes(trimmed)
        )
      })
  }, [departmentNameById, keyword, majorById, statusFilter, view.classes])

  const columns: TableColumn<OrganizationClassRow>[] = [
    {
      key: 'name',
      header: '班级',
      render: (row) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{row.entity.name}</div>
          <div className="truncate text-xs text-ink-sub">
            {row.departmentName} · {row.majorName}
          </div>
        </div>
      ),
    },
    {
      key: 'enrollment_year',
      header: '入学年份',
      align: 'right',
      mono: true,
      render: (row) => `${row.entity.enrollment_year} 级`,
    },
    {
      key: 'status',
      header: '状态',
      render: (row) => (
        <StatusIndicator
          tone={CLASS_STATUS_TONES[row.entity.status]}
          label={CLASS_STATUS_LABELS[row.entity.status]}
        />
      ),
    },
    ...extraColumns,
  ]

  return (
    <PageSection title="班级明细" description={description(rows.length)} actions={actions}>
      <div className="flex flex-col gap-4">
        <FilterBar label="班级筛选">
          <FilterField label="班级状态" group>
            <SegmentedControl
              aria-label="按班级状态筛选"
              size="sm"
              options={CLASS_STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
              value={statusFilter}
              onValueChange={onStatusChange}
            />
          </FilterField>
          <FilterField label="名称" htmlFor={keywordId}>
            <Input
              id={keywordId}
              value={keyword}
              placeholder="班级、专业或院系名"
              onChange={(event) => onKeywordChange(event.target.value)}
            />
          </FilterField>
        </FilterBar>

        <Table
          columns={columns}
          data={rows}
          rowKey={(row) => row.entity.id}
          empty={
            <Empty
              icon={Users}
              title={keyword || statusFilter ? '没有匹配的班级' : '还没有班级'}
              description={
                keyword || statusFilter ? '换个条件再试,或清空筛选查看全部班级。' : emptyDescription
              }
            />
          }
        />
      </div>
    </PageSection>
  )
}
