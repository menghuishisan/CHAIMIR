// 组织查看页(教师侧栏,/teacher/organization)。
//
// 只读:后端 identity 组织路由把读写分成两组,教师只在读组里
// (registerRead 挂 GET /org/departments|majors|classes,写操作是学校管理员的)。
// 因此本页不出现任何新增、编辑、删除、归档、升届动作 —— 教师需要改组织结构要找学校管理员。
//
// 三层结构一次读齐并在页面层组装成树:后端三个接口各自返回全量列表(带父级编号),
// 逐个院系去调专业接口会产生 N+1,而三张表的量级本就适合一次取回。

import { useMemo, useState } from 'react'
import { Building2, Network, Users } from 'lucide-react'
import { ClassStatus, type Class, type Department, type Major } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Empty,
  MetricStrip,
  PageHeader,
  PageScaffold,
  PageSection,
} from '@chaimir/ui'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import {
  OrganizationClassSection,
} from '../../components/OrganizationClassSection'
import {
  countClassesByMajor,
  loadOrganizationView,
  type OrganizationView,
} from '../../organizationView'

/**
 * TeacherOrganizationPage 只读呈现院系、专业与班级三层结构。
 */
export default function TeacherOrganizationPage() {
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('')

  const view = useAsyncResource<OrganizationView>(
    loadOrganizationView,
    [],
    (value) => value.departments.length === 0 && value.majors.length === 0 && value.classes.length === 0,
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '组织与成绩' }]} />}
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
  view: OrganizationView
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

  const activeClassCount = useMemo(
    () => classes.filter((entity) => entity.status === ClassStatus.ACTIVE).length,
    [classes],
  )

  return (
    <>
      {/*
        归族:资源列表族(§6.5.3 第 ①)。四项由一次取齐的全量列表算出,
        接口不分页,故客户端聚合就是全量口径(§6.5.4)。指标降为内联摘要:
        这一页的主体是院系专业卡与班级明细表,不是四个数字。
      */}
      <MetricStrip
        label="组织规模摘要"
        className="mb-5"
        items={[
          { label: '院系', value: departments.length, hint: '已建立的院系' },
          { label: '专业', value: majors.length, hint: '各院系下设专业' },
          { label: '班级', value: classes.length, hint: '含已归档' },
          {
            label: '在读班级',
            value: activeClassCount,
            hint: `已归档 ${classes.length - activeClassCount} 个`,
          },
        ]}
      />

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

      <OrganizationClassSection
        view={view}
        keyword={keyword}
        statusFilter={statusFilter}
        onKeywordChange={onKeywordChange}
        onStatusChange={onStatusChange}
        description={(count) => `共 ${count} 个班级`}
        emptyDescription="学校管理员建立班级后会显示在这里。"
      />

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
  const classCountByMajor = useMemo(() => countClassesByMajor(classes), [classes])

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
