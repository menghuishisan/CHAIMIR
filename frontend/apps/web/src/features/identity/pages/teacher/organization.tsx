// 组织查看页(教师侧栏,/teacher/organization)。
//
// 只读:后端 identity 组织路由把读写分成两组,教师只在读组里
// (registerRead 挂 GET /org/departments|majors|classes,写操作是学校管理员的)。
// 因此本页不出现任何新增、编辑、删除、归档、升届动作 —— 教师需要改组织结构要找学校管理员。
//
// 三层结构一次读齐并在页面层组装成树:后端三个接口各自返回全量列表(带父级编号),
// 逐个院系去调专业接口会产生 N+1,而三张表的量级本就适合一次取回。

import { useMemo, useState } from 'react'
import { Building2, GraduationCap, Network, Users } from 'lucide-react'
import { ClassStatus, type Class, type Department, type Major } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Empty,
  FilterBar,
  FilterField,
  Input,
  PageHeader,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Stat,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { CLASS_STATUS_FILTERS, CLASS_STATUS_LABELS, CLASS_STATUS_TONES } from '../../../../utils/labels/identity'

/** OrgView 是组织结构一次读齐的三层数据。 */
interface OrgView {
  departments: Department[]
  majors: Major[]
  classes: Class[]
}

/** ClassRow 是班级表格行:把院系与专业名挂到班级上,避免界面出现内部编号。 */
interface ClassRow {
  entity: Class
  majorName: string
  departmentName: string
}

/**
 * TeacherOrganizationPage 只读呈现院系、专业与班级三层结构。
 */
export default function TeacherOrganizationPage() {
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('')

  const view = useAsyncResource<OrgView>(
    () =>
      Promise.all([
        api.identity.listDepartments(),
        api.identity.listMajors(),
        api.identity.listClasses(),
      ]).then(([departments, majors, classes]) => ({ departments, majors, classes })),
    [],
    (value) => value.departments.length === 0 && value.majors.length === 0 && value.classes.length === 0,
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '组织与成绩' }, { label: '组织查看' }]} />}
        title="组织查看"
        description="学校的院系、专业与班级结构。这里只做查看,调整组织结构请联系学校管理员。"
        icon={Users}
      />

      <ResourceState
        resource={view}
        emptyIcon={Network}
        emptyTitle="还没有组织结构"
        emptyDescription="学校管理员建立院系、专业与班级后会显示在这里。"
      >
        {(data) => (
          <OrgContent
            view={data}
            keyword={keyword}
            statusFilter={statusFilter}
            onKeywordChange={setKeyword}
            onStatusChange={setStatusFilter}
          />
        )}
      </ResourceState>
    </PageScaffold>
  )
}

interface OrgContentProps {
  view: OrgView
  keyword: string
  statusFilter: string
  onKeywordChange: (keyword: string) => void
  onStatusChange: (status: string) => void
}

/**
 * OrgContent 渲染指标带、院系专业概览与班级明细。
 */
function OrgContent({ view, keyword, statusFilter, onKeywordChange, onStatusChange }: OrgContentProps) {
  const { departments, majors, classes } = view

  const departmentNameById = useMemo(
    () => new Map(departments.map((department) => [department.id, department.name])),
    [departments],
  )

  const majorById = useMemo(() => new Map(majors.map((major) => [major.id, major])), [majors])

  // 班级行:补齐专业与院系名,并按关键词与状态过滤
  const classRows = useMemo<ClassRow[]>(() => {
    const trimmed = keyword.trim()
    return classes
      .map((entity) => {
        const major = majorById.get(entity.major_id)
        return {
          entity,
          majorName: major ? major.name : '已撤销的专业',
          departmentName: major ? (departmentNameById.get(major.department_id) ?? '已撤销的院系') : '已撤销的院系',
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
  }, [classes, departmentNameById, keyword, majorById, statusFilter])

  const activeClassCount = useMemo(
    () => classes.filter((entity) => entity.status === ClassStatus.ACTIVE).length,
    [classes],
  )

  const columns: TableColumn<ClassRow>[] = [
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
  ]

  return (
    <>
      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="院系" value={departments.length} icon={Building2} />
          <Stat label="专业" value={majors.length} icon={GraduationCap} />
          <Stat label="班级" value={classes.length} icon={Users} />
          <Stat
            label="在读班级"
            value={activeClassCount}
            icon={Users}
            hint={`已归档 ${classes.length - activeClassCount} 个`}
          />
        </div>
      </PageSection>

      <PageSection title="院系与专业" description="每个院系下设的专业。带的数字是该专业当前的班级数。">
        {departments.length === 0 ? (
          <Empty icon={Building2} title="还没有院系" description="学校管理员建立院系后会显示在这里。" />
        ) : (
          <div className="grid gap-4 lg:grid-cols-2">
            {departments.map((department) => (
              <DepartmentCard
                key={department.id}
                department={department}
                majors={majors.filter((major) => major.department_id === department.id)}
                classes={classes}
              />
            ))}
          </div>
        )}
      </PageSection>

      <PageSection title="班级明细" description={`共 ${classRows.length} 个班级`}>
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
            <FilterField label="名称" htmlFor="org-keyword">
              <Input
                id="org-keyword"
                value={keyword}
                placeholder="班级、专业或院系名"
                onChange={(event) => onKeywordChange(event.target.value)}
              />
            </FilterField>
          </FilterBar>

          <Table
            columns={columns}
            data={classRows}
            rowKey={(row) => row.entity.id}
            empty={
              <Empty
                icon={Users}
                title={keyword || statusFilter ? '没有匹配的班级' : '还没有班级'}
                description={
                  keyword || statusFilter
                    ? '换个条件再试,或清空筛选查看全部班级。'
                    : '学校管理员建立班级后会显示在这里。'
                }
              />
            }
          />
        </div>
      </PageSection>

      <Callout tone="info">
        需要新增班级、调整专业归属或办理升届,请联系学校管理员 —— 组织结构的维护权限在学校管理端。
      </Callout>
    </>
  )
}

interface DepartmentCardProps {
  department: Department
  majors: Major[]
  classes: Class[]
}

/**
 * DepartmentCard 展示单个院系及其专业与班级规模。
 */
function DepartmentCard({ department, majors, classes }: DepartmentCardProps) {
  const classCountByMajor = useMemo(() => {
    const counts = new Map<string, number>()
    for (const entity of classes) {
      counts.set(entity.major_id, (counts.get(entity.major_id) ?? 0) + 1)
    }
    return counts
  }, [classes])

  return (
    <Card>
      <CardHeader
        title={department.name}
        description={`院系代码 ${department.code}`}
        actions={<Badge tone="neutral">{majors.length} 个专业</Badge>}
      />
      <CardBody>
        {majors.length === 0 ? (
          <p className="text-sm text-ink-sub">这个院系下还没有专业。</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {majors.map((major) => (
              <Badge key={major.id} tone="jade">
                {major.name} · {classCountByMajor.get(major.id) ?? 0} 个班
              </Badge>
            ))}
          </div>
        )}
      </CardBody>
    </Card>
  )
}
