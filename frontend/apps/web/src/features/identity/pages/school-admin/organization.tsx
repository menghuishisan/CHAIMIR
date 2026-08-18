// 组织架构页(校管侧栏,/school-admin/organization)。
//
// 院系 → 专业 → 班级三层,一页维护:三张表量级本就适合一次读齐并在页面层组装成树,
// 逐个院系去调专业接口会产生 N+1。
//
// 升届与归档是按年份的批量动作(后端 promoteClasses 接班级编号 + 目标年份,
// archiveClasses 接入学年份),两者都会改变学生的班级归属,故各自确认并说明影响范围。
//
// 删除有前置条件:院系下有专业、专业下有班级、班级里有学生时后端会拒绝 ——
// 界面先说明这一点,不让管理员点了才发现删不掉。

import { useCallback, useMemo, useState } from 'react'
import {
  Archive,
  Building2,
  GraduationCap,
  MoveUp,
  Network,
  Pencil,
  Plus,
  Trash2,
  Upload,
  Users,
} from 'lucide-react'
import {
  ClassStatus,
  type Class,
  type Department,
  type Major,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  Empty,
  FormField,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Select,
  Stat,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CLASS_STATUS_LABELS } from '../../../../utils/labels/identity'
import {
  OrganizationClassSection,
  type OrganizationClassRow,
} from '../../components/OrganizationClassSection'
import {
  countClassesByMajor,
  loadOrganizationView,
  type OrganizationView,
} from '../../organizationView'
import { OrgImportModal } from './org-import'

/** 待删除目标:三层各自的删除前置条件不同,用一个联合类型统一确认弹窗。 */
type DeleteTarget =
  | { kind: 'department'; item: Department }
  | { kind: 'major'; item: Major }
  | { kind: 'class'; item: Class }

const DELETE_COPY: Record<DeleteTarget['kind'], { title: string; description: string }> = {
  department: {
    title: '确认删除院系',
    description: '院系下还有专业时无法删除。请先移除或调整这些专业。',
  },
  major: {
    title: '确认删除专业',
    description: '专业下还有班级时无法删除。请先移除或调整这些班级。',
  },
  class: {
    title: '确认删除班级',
    description: '班级里还有学生时无法删除。可以改为归档,归档后班级不再出现在常规名单里。',
  },
}

/**
 * SchoolAdminOrganizationPage 维护院系、专业与班级三层结构。
 */
export default function SchoolAdminOrganizationPage() {
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [keyword, setKeyword] = useState('')
  const [departmentForm, setDepartmentForm] = useState<{ item?: Department } | undefined>()
  const [majorForm, setMajorForm] = useState<{ item?: Major } | undefined>()
  const [classForm, setClassForm] = useState<{ item?: Class } | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget>()
  const [promoteOpen, setPromoteOpen] = useState(false)
  const [archiveOpen, setArchiveOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const view = useAsyncResource<OrganizationView>(
    loadOrganizationView,
    [],
    () => false,
  )

  const runDelete = useCallback(async () => {
    if (!deleteTarget) return
    setWorking(true)
    setActionError(undefined)
    try {
      if (deleteTarget.kind === 'department') await api.identity.deleteDepartment(deleteTarget.item.id)
      if (deleteTarget.kind === 'major') await api.identity.deleteMajor(deleteTarget.item.id)
      if (deleteTarget.kind === 'class') await api.identity.deleteClass(deleteTarget.item.id)
      toast.success('已删除')
      setDeleteTarget(undefined)
      view.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '删除没有成功。可能下级还有内容,请先处理下级。'))
    } finally {
      setWorking(false)
    }
  }, [deleteTarget, view])

  const data = view.data

  const stats = useMemo(() => {
    if (!data) return { departments: 0, majors: 0, classes: 0, active: 0 }
    return {
      departments: data.departments.length,
      majors: data.majors.length,
      classes: data.classes.length,
      active: data.classes.filter((item) => item.status === ClassStatus.ACTIVE).length,
    }
  }, [data])

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '用户与组织' }, { label: '组织架构' }]} />}
        title="组织架构"
        description="维护院系、专业与班级。账号开通时按这里的结构选择归属,所以建议先建好组织。"
        icon={Network}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" leftIcon={Upload} onClick={() => setImportOpen(true)}>
              批量导入
            </Button>
            <Button variant="primary" leftIcon={Plus} onClick={() => setDepartmentForm({})}>
              新建院系
            </Button>
          </div>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="院系" value={stats.departments} icon={Building2} />
          <Stat label="专业" value={stats.majors} icon={GraduationCap} />
          <Stat label="班级" value={stats.classes} icon={Users} />
          <Stat
            label="在读班级"
            value={stats.active}
            icon={Users}
            hint={`已归档 ${stats.classes - stats.active} 个`}
          />
        </div>
      </PageSection>

      <ResourceState
        resource={view}
        emptyIcon={Network}
        emptyTitle="还没有组织结构"
        emptyDescription="先建院系,再在院系下建专业,最后建班级。也可以用批量导入一次建好。"
        emptyAction={
          <Button variant="primary" leftIcon={Plus} onClick={() => setDepartmentForm({})}>
            新建院系
          </Button>
        }
      >
        {(value) => (
          <>
            {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

            <PageSection
              title="院系与专业"
              description="每个院系下设的专业。带的数字是该专业当前的班级数。"
              actions={
                <Button
                  variant="outline"
                  size="sm"
                  leftIcon={Plus}
                  disabled={value.departments.length === 0}
                  onClick={() => setMajorForm({})}
                >
                  新建专业
                </Button>
              }
            >
              {value.departments.length === 0 ? (
                <Empty
                  icon={Building2}
                  title="还没有院系"
                  description="院系是组织结构的第一层,先建院系。"
                  action={
                    <Button variant="primary" leftIcon={Plus} onClick={() => setDepartmentForm({})}>
                      新建院系
                    </Button>
                  }
                />
              ) : (
                <div className="grid gap-4 lg:grid-cols-2">
                  {value.departments.map((department) => (
                    <DepartmentCard
                      key={department.id}
                      department={department}
                      majors={value.majors.filter((major) => major.department_id === department.id)}
                      classes={value.classes}
                      onEdit={() => setDepartmentForm({ item: department })}
                      onDelete={() => setDeleteTarget({ kind: 'department', item: department })}
                      onEditMajor={(major) => setMajorForm({ item: major })}
                      onDeleteMajor={(major) => setDeleteTarget({ kind: 'major', item: major })}
                    />
                  ))}
                </div>
              )}
            </PageSection>

            <ClassesSection
              view={value}
              statusFilter={statusFilter}
              keyword={keyword}
              onStatusChange={setStatusFilter}
              onKeywordChange={setKeyword}
              onCreate={() => setClassForm({})}
              onEdit={(item) => setClassForm({ item })}
              onDelete={(item) => setDeleteTarget({ kind: 'class', item })}
              onPromote={() => setPromoteOpen(true)}
              onArchive={() => setArchiveOpen(true)}
            />
          </>
        )}
      </ResourceState>

      {departmentForm ? (
        <DepartmentFormModal
          department={departmentForm.item}
          onClose={() => setDepartmentForm(undefined)}
          onSaved={() => {
            setDepartmentForm(undefined)
            view.reload()
          }}
        />
      ) : null}

      {majorForm && data ? (
        <MajorFormModal
          major={majorForm.item}
          departments={data.departments}
          onClose={() => setMajorForm(undefined)}
          onSaved={() => {
            setMajorForm(undefined)
            view.reload()
          }}
        />
      ) : null}

      {classForm && data ? (
        <ClassFormModal
          item={classForm.item}
          majors={data.majors}
          onClose={() => setClassForm(undefined)}
          onSaved={() => {
            setClassForm(undefined)
            view.reload()
          }}
        />
      ) : null}

      {promoteOpen && data ? (
        <PromoteClassesModal
          classes={data.classes}
          onClose={() => setPromoteOpen(false)}
          onDone={() => {
            setPromoteOpen(false)
            view.reload()
          }}
        />
      ) : null}

      {archiveOpen && data ? (
        <ArchiveClassesModal
          classes={data.classes}
          onClose={() => setArchiveOpen(false)}
          onDone={() => {
            setArchiveOpen(false)
            view.reload()
          }}
        />
      ) : null}

      {importOpen ? (
        <OrgImportModal
          onClose={() => setImportOpen(false)}
          onCommitted={() => {
            setImportOpen(false)
            view.reload()
          }}
        />
      ) : null}

      <Modal open={deleteTarget !== undefined} onOpenChange={(open) => !open && setDeleteTarget(undefined)}>
        <ModalContent size="sm">
          {deleteTarget ? (
            <>
              <ModalHeader>
                <ModalTitle>{DELETE_COPY[deleteTarget.kind].title}</ModalTitle>
                <ModalDescription>{DELETE_COPY[deleteTarget.kind].description}</ModalDescription>
              </ModalHeader>
              <ModalBody>
                <p className="text-base text-ink">{deleteTarget.item.name}</p>
              </ModalBody>
              <ModalFooter>
                <Button variant="outline" onClick={() => setDeleteTarget(undefined)}>
                  取消
                </Button>
                <Button variant="danger" loading={working} onClick={() => void runDelete()}>
                  确认删除
                </Button>
              </ModalFooter>
            </>
          ) : null}
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}

interface DepartmentCardProps {
  department: Department
  majors: Major[]
  classes: Class[]
  onEdit: () => void
  onDelete: () => void
  onEditMajor: (major: Major) => void
  onDeleteMajor: (major: Major) => void
}

/**
 * DepartmentCard 展示单个院系及其专业,并承载两层的编辑与删除。
 */
function DepartmentCard({
  department,
  majors,
  classes,
  onEdit,
  onDelete,
  onEditMajor,
  onDeleteMajor,
}: DepartmentCardProps) {
  const classCountByMajor = useMemo(() => countClassesByMajor(classes), [classes])

  return (
    <Card>
      <CardHeader
        title={department.name}
        description={`院系代码 ${department.code}`}
        actions={
          <div className="flex items-center gap-1">
            <Badge tone="neutral">{majors.length} 个专业</Badge>
            <IconButton variant="ghost" size="sm" icon={Pencil} aria-label={`编辑 ${department.name}`} onClick={onEdit} />
            <IconButton
              variant="ghost"
              size="sm"
              icon={Trash2}
              aria-label={`删除 ${department.name}`}
              onClick={onDelete}
            />
          </div>
        }
      />
      <CardBody>
        {majors.length === 0 ? (
          <p className="text-sm text-ink-sub">这个院系下还没有专业。</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {majors.map((major) => (
              <li key={major.id} className="flex items-center justify-between gap-2">
                <span className="min-w-0 truncate text-sm text-ink">
                  {major.name}
                  <span className="ml-1.5 text-xs text-ink-sub">
                    {classCountByMajor.get(major.id) ?? 0} 个班
                  </span>
                </span>
                <span className="flex shrink-0 items-center gap-1">
                  <IconButton
                    variant="ghost"
                    size="sm"
                    icon={Pencil}
                    aria-label={`编辑 ${major.name}`}
                    onClick={() => onEditMajor(major)}
                  />
                  <IconButton
                    variant="ghost"
                    size="sm"
                    icon={Trash2}
                    aria-label={`删除 ${major.name}`}
                    onClick={() => onDeleteMajor(major)}
                  />
                </span>
              </li>
            ))}
          </ul>
        )}
      </CardBody>
    </Card>
  )
}

interface ClassesSectionProps {
  view: OrganizationView
  statusFilter: string
  keyword: string
  onStatusChange: (value: string) => void
  onKeywordChange: (value: string) => void
  onCreate: () => void
  onEdit: (item: Class) => void
  onDelete: (item: Class) => void
  onPromote: () => void
  onArchive: () => void
}

/**
 * ClassesSection 渲染班级明细与批量升届/归档入口。
 */
function ClassesSection({
  view,
  statusFilter,
  keyword,
  onStatusChange,
  onKeywordChange,
  onCreate,
  onEdit,
  onDelete,
  onPromote,
  onArchive,
}: ClassesSectionProps) {
  const columns: TableColumn<OrganizationClassRow>[] = [
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (row) => (
        <div className="flex items-center justify-end gap-1">
          <IconButton
            variant="ghost"
            size="sm"
            icon={Pencil}
            aria-label={`编辑 ${row.entity.name}`}
            onClick={() => onEdit(row.entity)}
          />
          <IconButton
            variant="ghost"
            size="sm"
            icon={Trash2}
            aria-label={`删除 ${row.entity.name}`}
            onClick={() => onDelete(row.entity)}
          />
        </div>
      ),
    },
  ]

  return (
    <OrganizationClassSection
      view={view}
      keyword={keyword}
      statusFilter={statusFilter}
      onKeywordChange={onKeywordChange}
      onStatusChange={onStatusChange}
      description={(count) => `共 ${count} 个班级。升届把班级整体升到下一年级,归档把某一届班级转为已归档。`}
      emptyDescription={
        view.majors.length === 0 ? '先建专业,才能在专业下建班级。' : '班级是学生账号的归属,开通学生账号前先建班级。'
      }
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" leftIcon={MoveUp} onClick={onPromote}>
            批量升届
          </Button>
          <Button variant="outline" size="sm" leftIcon={Archive} onClick={onArchive}>
            批量归档
          </Button>
          <Button
            variant="primary"
            size="sm"
            leftIcon={Plus}
            disabled={view.majors.length === 0}
            onClick={onCreate}
          >
            新建班级
          </Button>
        </div>
      }
      extraColumns={columns}
    />
  )
}

interface DepartmentFormModalProps {
  department?: Department
  onClose: () => void
  onSaved: () => void
}

/**
 * DepartmentFormModal 承载院系新建与编辑。
 */
function DepartmentFormModal({ department, onClose, onSaved }: DepartmentFormModalProps) {
  const editing = department !== undefined
  const [name, setName] = useState(department?.name ?? '')
  const [code, setCode] = useState(department?.code ?? '')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (name.trim() === '') {
        setFormError('请输入院系名称')
        return
      }
      if (code.trim() === '') {
        setFormError('请输入院系代码')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        const payload = { name: name.trim(), code: code.trim() }
        if (editing) await api.identity.updateDepartment(department.id, payload)
        else await api.identity.createDepartment(payload)
        toast.success(editing ? '院系已更新' : '院系已创建')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [code, department?.id, editing, name, onSaved],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑院系' : '新建院系'}</ModalTitle>
          <ModalDescription>院系是组织结构的第一层,下设专业。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="院系名称" htmlFor="dept-name" required>
              <Input id="dept-name" value={name} onChange={(event) => setName(event.target.value)} />
            </FormField>
            <FormField label="院系代码" htmlFor="dept-code" required helper="校内唯一,用于导入与对接">
              <Input id="dept-code" value={code} onChange={(event) => setCode(event.target.value)} />
            </FormField>
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              {editing ? '保存修改' : '创建院系'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface MajorFormModalProps {
  major?: Major
  departments: Department[]
  onClose: () => void
  onSaved: () => void
}

/**
 * MajorFormModal 承载专业新建与编辑。
 */
function MajorFormModal({ major, departments, onClose, onSaved }: MajorFormModalProps) {
  const editing = major !== undefined
  const [name, setName] = useState(major?.name ?? '')
  const [departmentId, setDepartmentId] = useState(major?.department_id ?? '')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (name.trim() === '') {
        setFormError('请输入专业名称')
        return
      }
      if (departmentId === '') {
        setFormError('请选择所属院系')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        const payload = { name: name.trim(), department_id: departmentId }
        if (editing) await api.identity.updateMajor(major.id, payload)
        else await api.identity.createMajor(payload)
        toast.success(editing ? '专业已更新' : '专业已创建')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [departmentId, editing, major?.id, name, onSaved],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑专业' : '新建专业'}</ModalTitle>
          <ModalDescription>专业挂在院系下,班级挂在专业下。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="所属院系" htmlFor="major-dept" required>
              <Select
                id="major-dept"
                options={departments.map((item) => ({ value: item.id, label: item.name }))}
                value={departmentId}
                placeholder="选择院系"
                onValueChange={setDepartmentId}
              />
            </FormField>
            <FormField label="专业名称" htmlFor="major-name" required>
              <Input id="major-name" value={name} onChange={(event) => setName(event.target.value)} />
            </FormField>
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              {editing ? '保存修改' : '创建专业'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface ClassFormModalProps {
  item?: Class
  majors: Major[]
  onClose: () => void
  onSaved: () => void
}

/**
 * ClassFormModal 承载班级新建与编辑。
 */
function ClassFormModal({ item, majors, onClose, onSaved }: ClassFormModalProps) {
  const editing = item !== undefined
  const [name, setName] = useState(item?.name ?? '')
  const [majorId, setMajorId] = useState(item?.major_id ?? '')
  const [year, setYear] = useState(String(item?.enrollment_year ?? new Date().getFullYear()))
  const [status, setStatus] = useState(String(item?.status ?? ClassStatus.ACTIVE))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (name.trim() === '') {
        setFormError('请输入班级名称')
        return
      }
      if (majorId === '') {
        setFormError('请选择所属专业')
        return
      }
      const yearValue = Number(year)
      if (!Number.isInteger(yearValue) || yearValue < 1990) {
        setFormError('请输入有效的入学年份')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        const payload = {
          name: name.trim(),
          major_id: majorId,
          enrollment_year: yearValue,
          status: Number(status) as ClassStatus,
        }
        if (editing) await api.identity.updateClass(item.id, payload)
        else await api.identity.createClass(payload)
        toast.success(editing ? '班级已更新' : '班级已创建')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [editing, item?.id, majorId, name, onSaved, status, year],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑班级' : '新建班级'}</ModalTitle>
          <ModalDescription>学生账号按班级归属,入学年份用于升届与归档。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="所属专业" htmlFor="class-major" required>
              <Select
                id="class-major"
                options={majors.map((major) => ({ value: major.id, label: major.name }))}
                value={majorId}
                placeholder="选择专业"
                onValueChange={setMajorId}
              />
            </FormField>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="班级名称" htmlFor="class-name" required>
                <Input id="class-name" value={name} onChange={(event) => setName(event.target.value)} />
              </FormField>
              <FormField label="入学年份" htmlFor="class-year" required>
                <Input
                  id="class-year"
                  type="number"
                  min="1990"
                  value={year}
                  onChange={(event) => setYear(event.target.value)}
                />
              </FormField>
            </div>
            <FormField label="状态" required helper="已归档的班级不出现在常规名单里">
              <SegmentedControl
                aria-label="班级状态"
                options={[
                  { value: String(ClassStatus.ACTIVE), label: CLASS_STATUS_LABELS[ClassStatus.ACTIVE] },
                  { value: String(ClassStatus.ARCHIVED), label: CLASS_STATUS_LABELS[ClassStatus.ARCHIVED] },
                ]}
                value={status}
                onValueChange={setStatus}
              />
            </FormField>
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              {editing ? '保存修改' : '创建班级'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface PromoteClassesModalProps {
  classes: Class[]
  onClose: () => void
  onDone: () => void
}

/**
 * PromoteClassesModal 批量升届。
 * 升届是把选中班级的入学年份改成目标年份 —— 后端按班级编号 + 目标年份处理,
 * 故这里勾选具体班级而不是笼统"全校升届"。
 */
function PromoteClassesModal({ classes, onClose, onDone }: PromoteClassesModalProps) {
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [targetYear, setTargetYear] = useState(String(new Date().getFullYear()))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const activeClasses = useMemo(
    () => classes.filter((item) => item.status === ClassStatus.ACTIVE),
    [classes],
  )

  const submit = useCallback(async () => {
    if (selected.size === 0) {
      setFormError('请至少勾选一个班级')
      return
    }
    const year = Number(targetYear)
    if (!Number.isInteger(year) || year < 1990) {
      setFormError('请输入有效的目标年份')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.identity.promoteClasses({ class_ids: Array.from(selected), target_year: year })
      toast.success(`已升届 ${selected.size} 个班级`)
      onDone()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '升届没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [onDone, selected, targetYear])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>批量升届</ModalTitle>
          <ModalDescription>
            把选中班级的入学年份统一改成目标年份。班级里的学生归属不变。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField label="目标入学年份" htmlFor="promote-year" required>
            <Input
              id="promote-year"
              type="number"
              min="1990"
              value={targetYear}
              onChange={(event) => setTargetYear(event.target.value)}
            />
          </FormField>

          {activeClasses.length === 0 ? (
            <Empty icon={Users} title="没有在读班级" description="只有在读班级可以升届。" />
          ) : (
            <FormField label="选择班级" required>
              <div className="flex max-h-72 flex-col gap-2 overflow-y-auto well p-3">
                {activeClasses.map((item) => (
                  <Checkbox
                    key={item.id}
                    checked={selected.has(item.id)}
                    label={`${item.name} · ${item.enrollment_year} 级`}
                    onCheckedChange={(checked) =>
                      setSelected((current) => {
                        const next = new Set(current)
                        if (checked === true) next.add(item.id)
                        else next.delete(item.id)
                        return next
                      })
                    }
                  />
                ))}
              </div>
            </FormField>
          )}

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" loading={working} onClick={() => void submit()}>
            确认升届 {selected.size > 0 ? `${selected.size} 个班级` : ''}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface ArchiveClassesModalProps {
  classes: Class[]
  onClose: () => void
  onDone: () => void
}

/**
 * ArchiveClassesModal 按入学年份批量归档。
 * 后端 archiveClasses 接入学年份而不是班级编号:整届毕业是一次性动作,
 * 故这里从现有年份里选,不让管理员手填一个可能不存在的年份。
 */
function ArchiveClassesModal({ classes, onClose, onDone }: ArchiveClassesModalProps) {
  const [year, setYear] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const yearOptions = useMemo(() => {
    const years = new Set<number>()
    for (const item of classes) {
      if (item.status === ClassStatus.ACTIVE) years.add(item.enrollment_year)
    }
    return Array.from(years)
      .sort((a, b) => a - b)
      .map((value) => ({
        value: String(value),
        label: `${value} 级 · ${classes.filter((item) => item.enrollment_year === value && item.status === ClassStatus.ACTIVE).length} 个班`,
      }))
  }, [classes])

  const submit = useCallback(async () => {
    if (year === '') {
      setFormError('请选择要归档的入学年份')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.identity.archiveClasses({ enrollment_year: Number(year) })
      toast.success(`${year} 级班级已归档`)
      onDone()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '归档没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [onDone, year])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>批量归档班级</ModalTitle>
          <ModalDescription>
            整届毕业时用这个:该年份的所有在读班级会转为已归档,不再出现在常规名单里。历史数据保留。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          {yearOptions.length === 0 ? (
            <Empty icon={Archive} title="没有可归档的年份" description="所有班级都已归档。" />
          ) : (
            <FormField label="入学年份" htmlFor="archive-year" required>
              <Select
                id="archive-year"
                options={yearOptions}
                value={year}
                placeholder="选择年份"
                onValueChange={setYear}
              />
            </FormField>
          )}
          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button
            variant="primary"
            loading={working}
            disabled={yearOptions.length === 0}
            onClick={() => void submit()}
          >
            确认归档
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
