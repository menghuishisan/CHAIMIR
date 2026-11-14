// 组织查看页(教师侧栏,/teacher/organization)。设计 §7.4:
// 只读:教师查看院系、专业、班级结构(例如按班级添加课程成员前)。页头一句「调整组织结构请联系学校管理员」;
// 左栏院系→专业树(上方搜索),右栏所选专业的班级列表(班级名、入学年份、状态);选中院系时右栏列它的专业。
// 选中的节点与搜索词写进地址。后端组织路由把读写分组,教师只在读组,本页没有任何写操作。
// 三层结构一次并行读齐(loadOrganizationView):三张表的量级适合一次取回,逐个院系去取专业会产生 N+1。

import { useMemo } from 'react'
import { GraduationCap, Network, Users } from 'lucide-react'
import type { Class, Department, Major } from '@chaimir/api-client'
import { Empty, ListRow, PageHeader, StatusIndicator, PageScaffold } from '@chaimir/ui'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { CLASS_STATUS_LABELS } from '../../../../utils/labels/identity'
import { OrgTree } from '../../components/OrgTree'
import { loadOrganizationView, type OrganizationView } from '../../organizationView'
import { buildOrgTree, classesOfMajor, resolveOrgSelection } from '../../organizationTree'
import { CLASS_STATUS_TONES } from '../../statusPresentation'

/** 地址参数:dept / major 是选中的节点(专业优先),q 是树上方的搜索词 */
const ORG_QUERY = {
  dept: { defaultValue: '' },
  major: { defaultValue: '' },
  q: { defaultValue: '' },
} as const

/**
 * TeacherOrganizationPage 只读呈现院系、专业与班级三层结构。
 */
export default function TeacherOrganizationPage() {
  const view = useAsyncResource<OrganizationView>(
    loadOrganizationView,
    [],
    (value) => value.departments.length === 0 && value.majors.length === 0 && value.classes.length === 0,
  )

  return (
    <PageScaffold>
      <PageHeader title="组织查看" description="学校的院系、专业与班级结构。调整组织结构请联系学校管理员。" />
      <ResourceState
        resource={view}
        emptyIcon={Network}
        emptyTitle="还没有组织结构"
        emptyDescription="学校管理员建立院系、专业与班级后会显示在这里。"
      >
        {(data) => <OrgBrowser view={data} />}
      </ResourceState>
    </PageScaffold>
  )
}

/** OrgBrowser 左栏树 + 右栏所选节点的内容 */
function OrgBrowser({ view }: { view: OrganizationView }) {
  const [query, setQuery] = useUrlState(ORG_QUERY)
  const tree = useMemo(
    () => buildOrgTree(view.departments, view.majors, view.classes, query.q),
    [query.q, view.classes, view.departments, view.majors],
  )
  const selection = resolveOrgSelection(tree, { dept: query.dept, major: query.major })
  const departmentNode = selection ? tree.find((node) => node.department.id === selection.departmentId) : undefined

  return (
    <div className="flex flex-col gap-6 lg:flex-row lg:items-start">
      <div className="lg:sticky lg:top-6 lg:w-80 lg:shrink-0">
        <OrgTree
          tree={tree}
          selection={selection}
          keyword={query.q}
          onSearch={(value) => setQuery({ q: value })}
          onSelectDepartment={(id) => setQuery({ dept: id, major: '' })}
          onSelectMajor={(id) => setQuery({ major: id, dept: '' })}
          emptyText="还没有院系。"
        />
      </div>
      <section aria-live="polite" className="min-w-0 flex-1">
        {!selection || !departmentNode ? (
          <div className="rounded-lg bg-surface shadow-xs">
            <Empty icon={Network} title="没有可以查看的内容" description="换个搜索词,或清空搜索查看全部。" />
          </div>
        ) : selection.kind === 'major' ? (
          <MajorClasses
            department={departmentNode.department}
            major={departmentNode.majors.find((child) => child.major.id === selection.majorId)?.major}
            classes={view.classes}
            keyword={query.q}
          />
        ) : (
          <DepartmentMajors
            department={departmentNode.department}
            majors={departmentNode.majors.map((child) => ({ major: child.major, classCount: child.classCount }))}
            onOpen={(id) => setQuery({ major: id, dept: '' })}
          />
        )}
      </section>
    </div>
  )
}

/** MajorClasses 所选专业的班级:班级名、入学年份、状态;搜索词命中班级名时只列命中的 */
function MajorClasses({
  department,
  major,
  classes,
  keyword,
}: {
  department: Department
  major: Major | undefined
  classes: Class[]
  keyword: string
}) {
  if (!major) return null
  const { list, filtered } = classesOfMajor(classes, major.id, keyword)
  return (
    <div className="rounded-lg bg-surface p-5 shadow-xs">
      <h2 className="text-lg font-semibold text-ink">{major.name}</h2>
      <p className="mt-0.5 text-sm text-ink-sub">
        {department.name} · {filtered ? `匹配的 ${list.length} 个班级` : `${list.length} 个班级`}
      </p>
      {list.length === 0 ? (
        <Empty icon={Users} title="这个专业下还没有班级" description="学校管理员建立班级后会显示在这里。" />
      ) : (
        <ul className="mt-3 divide-y divide-line">
          {list.map((entity) => (
            <li key={entity.id}>
              <ListRow
                title={entity.name}
                meta={`${entity.enrollment_year} 级`}
                trailing={<StatusIndicator tone={CLASS_STATUS_TONES[entity.status]} label={CLASS_STATUS_LABELS[entity.status]} />}
              />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

/** DepartmentMajors 所选院系的专业列表,整行切到该专业的班级 */
function DepartmentMajors({
  department,
  majors,
  onOpen,
}: {
  department: Department
  majors: { major: Major; classCount: number }[]
  onOpen: (majorId: string) => void
}) {
  return (
    <div className="rounded-lg bg-surface p-5 shadow-xs">
      <h2 className="text-lg font-semibold text-ink">{department.name}</h2>
      <p className="mt-0.5 text-sm text-ink-sub">
        院系代码 {department.code} · {majors.length} 个专业
      </p>
      {majors.length === 0 ? (
        <Empty icon={GraduationCap} title="这个院系下还没有专业" description="学校管理员建立专业后会显示在这里。" />
      ) : (
        <ul className="mt-3 divide-y divide-line">
          {majors.map(({ major, classCount }) => (
            <li key={major.id}>
              <ListRow title={major.name} meta={`${classCount} 个班级`} onSelect={() => onOpen(major.id)} />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
