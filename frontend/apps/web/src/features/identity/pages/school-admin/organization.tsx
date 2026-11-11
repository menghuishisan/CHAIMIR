// 组织架构页(校管侧栏,/school-admin/organization)。设计 §7.5:
// 左栏院系→专业树(搜索、新建院系、在院系下新建专业、重命名、删除);右栏是所选节点的内容:
// 选中专业时为班级表格(勾选、班级名、入学年份、状态),上方「新建班级」,勾选后出现「批量升届」;
// 选中院系时是它的专业列表,上方「新建专业」。主操作随选中节点变化,树为空时为「新建院系」。
// 页头「批量导入」与「整届归档」;选中的节点与搜索词写进地址;删除有下级的节点时先说明原因。
//
// 三层结构一次并行读齐(loadOrganizationView),在页面层组装成树(organizationTree,与教师组织查看共用)。
// 升届接班级编号 + 目标年份(只对在读班级,目标年份须晚于原入学年份);归档按入学年份整届进行,
// 会同时归档该届学生账号并让其下线 —— 两者都各自确认并说明影响范围。

import { useCallback, useMemo, useState } from 'react'
import { Archive, Building2, GraduationCap, MoreVertical, MoveUp, Network, Pencil, Plus, Trash2, Upload, Users } from 'lucide-react'
import { ClassStatus, type Class, type Department, type Major } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Checkbox,
  Empty,
  FormField,
  IconButton,
  Input,
  ListRow,
  Menu,
  MenuContent,
  MenuItem,
  MenuSeparator,
  MenuTrigger,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  SegmentedControl,
  Select,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CLASS_STATUS_LABELS } from '../../../../utils/labels/identity'
import { OrgTree } from '../../components/OrgTree'
import { loadOrganizationView, type OrganizationView } from '../../organizationView'
import { buildOrgTree, classesOfMajor, resolveOrgSelection } from '../../organizationTree'
import { CLASS_STATUS_TONES } from '../../statusPresentation'
import { OrgImportModal } from './org-import'

/** 地址参数:dept / major 是选中的节点(专业优先),q 是树上方的搜索词 */
const ORG_QUERY = {
  dept: { defaultValue: '' },
  major: { defaultValue: '' },
  q: { defaultValue: '' },
} as const

/** 待删除目标:三层各自的删除前置条件不同,用一个联合类型统一确认弹窗。 */
type DeleteTarget =
  | { kind: 'department'; item: Department }
  | { kind: 'major'; item: Major }
  | { kind: 'class'; item: Class }

const DELETE_TITLES: Record<DeleteTarget['kind'], string> = {
  department: '删除这个院系?',
  major: '删除这个专业?',
  class: '删除这个班级?',
}

/**
 * deleteBlocker 有下级时不能删除,返回原因(后端同样会拒绝);班级里有没有学生页面拿不到,由后端判断。
 */
function deleteBlocker(target: DeleteTarget, view: OrganizationView): string | undefined {
  if (target.kind === 'department') {
    const count = view.majors.filter((major) => major.department_id === target.item.id).length
    return count > 0 ? `这个院系下还有 ${count} 个专业,要先删除或调整这些专业才能删除院系。` : undefined
  }
  if (target.kind === 'major') {
    const count = view.classes.filter((entity) => entity.major_id === target.item.id).length
    return count > 0 ? `这个专业下还有 ${count} 个班级,要先删除或调整这些班级才能删除专业。` : undefined
  }
  return undefined
}

/**
 * SchoolAdminOrganizationPage 维护院系、专业与班级三层结构。
 */
export default function SchoolAdminOrganizationPage() {
  const [importOpen, setImportOpen] = useState(false)
  const [archiveOpen, setArchiveOpen] = useState(false)
  const view = useAsyncResource<OrganizationView>(loadOrganizationView, [], () => false)

  return (
    <PageScaffold>
      <PageHeader
        title="组织架构"
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="ghost" leftIcon={Archive} disabled={view.status !== 'success'} onClick={() => setArchiveOpen(true)}>
              整届归档
            </Button>
            <Button variant="outline" leftIcon={Upload} onClick={() => setImportOpen(true)}>
              批量导入
            </Button>
          </div>
        }
      />

      <ResourceState resource={view} emptyIcon={Network} emptyTitle="还没有组织结构" emptyDescription="先建院系,再在院系下建专业,最后建班级。">
        {(data) => <OrgManager view={data} onChanged={view.reload} />}
      </ResourceState>

      {importOpen ? (
        <OrgImportModal
          onClose={() => setImportOpen(false)}
          onCommitted={() => {
            setImportOpen(false)
            view.reload()
          }}
        />
      ) : null}

      {archiveOpen && view.data ? (
        <ArchiveClassesModal
          classes={view.data.classes}
          onClose={() => setArchiveOpen(false)}
          onDone={() => {
            setArchiveOpen(false)
            view.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

/** OrgManager 左栏树 + 右栏所选节点;表单与删除确认都从这里打开 */
function OrgManager({ view, onChanged }: { view: OrganizationView; onChanged: () => void }) {
  const [query, setQuery] = useUrlState(ORG_QUERY)
  const [departmentForm, setDepartmentForm] = useState<{ item?: Department } | undefined>()
  const [majorForm, setMajorForm] = useState<{ item?: Major; departmentId?: string } | undefined>()
  const [classForm, setClassForm] = useState<{ item?: Class; majorId?: string } | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget>()
  const [promoteIds, setPromoteIds] = useState<string[]>()
  const [working, setWorking] = useState(false)
  const [deleteError, setDeleteError] = useState<string>()

  const tree = useMemo(
    () => buildOrgTree(view.departments, view.majors, view.classes, query.q),
    [query.q, view.classes, view.departments, view.majors],
  )
  const selection = resolveOrgSelection(tree, { dept: query.dept, major: query.major })
  const departmentNode = selection ? tree.find((node) => node.department.id === selection.departmentId) : undefined
  const selectedMajor =
    selection?.kind === 'major' ? departmentNode?.majors.find((child) => child.major.id === selection.majorId)?.major : undefined

  const runDelete = useCallback(async () => {
    if (!deleteTarget) return
    setWorking(true)
    setDeleteError(undefined)
    try {
      if (deleteTarget.kind === 'department') await api.identity.deleteDepartment(deleteTarget.item.id)
      if (deleteTarget.kind === 'major') await api.identity.deleteMajor(deleteTarget.item.id)
      if (deleteTarget.kind === 'class') await api.identity.deleteClass(deleteTarget.item.id)
      toast.success(`已删除:${deleteTarget.item.name}`)
      setDeleteTarget(undefined)
      onChanged()
    } catch (error) {
      setDeleteError(userFacingErrorMessage(error, '删除没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [deleteTarget, onChanged])

  const nodeMenu = (items: { label: string; icon: typeof Pencil; onSelect: () => void; danger?: boolean }[], name: string) => (
    <Menu>
      <MenuTrigger asChild>
        <IconButton variant="ghost" size="sm" icon={MoreVertical} aria-label={`${name} 的更多操作`} />
      </MenuTrigger>
      <MenuContent align="end">
        {items.map((entry, index) =>
          entry.danger ? (
            <span key={entry.label}>
              {index > 0 ? <MenuSeparator /> : null}
              <MenuItem danger icon={entry.icon} onSelect={entry.onSelect}>
                {entry.label}
              </MenuItem>
            </span>
          ) : (
            <MenuItem key={entry.label} icon={entry.icon} onSelect={entry.onSelect}>
              {entry.label}
            </MenuItem>
          ),
        )}
      </MenuContent>
    </Menu>
  )

  const blocker = deleteTarget ? deleteBlocker(deleteTarget, view) : undefined

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
          emptyText="还没有院系。院系是组织结构的第一层。"
          action={
            <>
              <span className="text-sm font-semibold text-ink">院系与专业</span>
              <Button variant={view.departments.length === 0 ? 'primary' : 'ghost'} size="sm" leftIcon={Plus} onClick={() => setDepartmentForm({})}>
                新建院系
              </Button>
            </>
          }
          renderDepartmentActions={(department) =>
            nodeMenu(
              [
                { label: '在此院系下新建专业', icon: Plus, onSelect: () => setMajorForm({ departmentId: department.id }) },
                { label: '重命名', icon: Pencil, onSelect: () => setDepartmentForm({ item: department }) },
                { label: '删除', icon: Trash2, danger: true, onSelect: () => setDeleteTarget({ kind: 'department', item: department }) },
              ],
              department.name,
            )
          }
          renderMajorActions={(major) =>
            nodeMenu(
              [
                { label: '重命名', icon: Pencil, onSelect: () => setMajorForm({ item: major }) },
                { label: '删除', icon: Trash2, danger: true, onSelect: () => setDeleteTarget({ kind: 'major', item: major }) },
              ],
              major.name,
            )
          }
        />
      </div>

      <section aria-live="polite" className="min-w-0 flex-1">
        {view.departments.length === 0 ? (
          <div className="rounded-lg bg-surface shadow-xs">
            <Empty
              icon={Building2}
              title="先建院系"
              description="先建院系,再在院系下建专业,最后建班级。也可以用批量导入一次建好。"
              action={
                <Button variant="primary" leftIcon={Plus} onClick={() => setDepartmentForm({})}>
                  新建院系
                </Button>
              }
            />
          </div>
        ) : !selection || !departmentNode ? (
          <div className="rounded-lg bg-surface shadow-xs">
            <Empty icon={Network} title="没有匹配的内容" description="换个搜索词,或清空搜索查看全部。" />
          </div>
        ) : selection.kind === 'major' && selectedMajor ? (
          <MajorPane
            department={departmentNode.department}
            major={selectedMajor}
            classes={view.classes}
            keyword={query.q}
            onCreate={() => setClassForm({ majorId: selectedMajor.id })}
            onEdit={(item) => setClassForm({ item })}
            onDelete={(item) => setDeleteTarget({ kind: 'class', item })}
            onPromote={setPromoteIds}
          />
        ) : (
          <DepartmentPane
            department={departmentNode.department}
            majors={departmentNode.majors}
            onCreate={() => setMajorForm({ departmentId: departmentNode.department.id })}
            onOpen={(id) => setQuery({ major: id, dept: '' })}
          />
        )}
      </section>

      {departmentForm ? (
        <DepartmentFormModal
          department={departmentForm.item}
          onClose={() => setDepartmentForm(undefined)}
          onSaved={() => {
            setDepartmentForm(undefined)
            onChanged()
          }}
        />
      ) : null}

      {majorForm ? (
        <MajorFormModal
          major={majorForm.item}
          departments={view.departments}
          initialDepartmentId={majorForm.departmentId}
          onClose={() => setMajorForm(undefined)}
          onSaved={() => {
            setMajorForm(undefined)
            onChanged()
          }}
        />
      ) : null}

      {classForm ? (
        <ClassFormModal
          item={classForm.item}
          majors={view.majors}
          initialMajorId={classForm.majorId}
          onClose={() => setClassForm(undefined)}
          onSaved={() => {
            setClassForm(undefined)
            onChanged()
          }}
        />
      ) : null}

      {promoteIds ? (
        <PromoteClassesModal
          classes={view.classes.filter((entity) => promoteIds.includes(entity.id))}
          onClose={() => setPromoteIds(undefined)}
          onDone={() => {
            setPromoteIds(undefined)
            onChanged()
          }}
        />
      ) : null}

      <Modal
        open={deleteTarget !== undefined}
        onOpenChange={(open) => {
          if (!open && !working) {
            setDeleteTarget(undefined)
            setDeleteError(undefined)
          }
        }}
      >
        <ModalContent size="sm">
          {deleteTarget ? (
            <>
              <ModalHeader>
                <ModalTitle>{blocker ? `暂时不能删除「${deleteTarget.item.name}」` : DELETE_TITLES[deleteTarget.kind]}</ModalTitle>
                <ModalDescription>
                  {blocker ??
                    (deleteTarget.kind === 'class'
                      ? '班级里还有学生时不能删除,可以改为归档。删除不能撤销。'
                      : '删除不能撤销。')}
                </ModalDescription>
              </ModalHeader>
              {blocker ? null : (
                <ModalBody className="flex flex-col gap-3">
                  <p className="text-base font-medium text-ink">{deleteTarget.item.name}</p>
                  {deleteError ? <Callout tone="danger">{deleteError}</Callout> : null}
                </ModalBody>
              )}
              <ModalFooter>
                <Button variant="outline" disabled={working} onClick={() => setDeleteTarget(undefined)}>
                  {blocker ? '知道了' : '取消'}
                </Button>
                {blocker ? null : (
                  <Button variant="danger" loading={working} onClick={() => void runDelete()}>
                    确认删除
                  </Button>
                )}
              </ModalFooter>
            </>
          ) : null}
        </ModalContent>
      </Modal>
    </div>
  )
}

interface MajorPaneProps {
  department: Department
  major: Major
  classes: Class[]
  keyword: string
  onCreate: () => void
  onEdit: (item: Class) => void
  onDelete: (item: Class) => void
  /** 以勾选的班级打开批量升届 */
  onPromote: (classIds: string[]) => void
}

/**
 * MajorPane 所选专业的班级:「新建班级」为主操作;在读班级可勾选,勾选后出现「批量升届」。
 * 已归档的班级不能升届(后端同一口径),不给勾选框。
 */
function MajorPane({ department, major, classes, keyword, onCreate, onEdit, onDelete, onPromote }: MajorPaneProps) {
  const { list, filtered } = classesOfMajor(classes, major.id, keyword)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  // 切换专业或列表变化后,只保留仍在当前列表里的在读班级
  const selectable = list.filter((entity) => entity.status === ClassStatus.ACTIVE)
  const picked = selectable.filter((entity) => selected.has(entity.id))
  const allPicked = selectable.length > 0 && picked.length === selectable.length

  return (
    <div className="rounded-lg bg-surface p-5 shadow-xs">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-lg font-semibold text-ink">{major.name}</h2>
          <p className="mt-0.5 text-sm text-ink-sub">
            {department.name} · {filtered ? `匹配的 ${list.length} 个班级` : `${list.length} 个班级`}
          </p>
        </div>
        <Button variant="primary" leftIcon={Plus} onClick={onCreate}>
          新建班级
        </Button>
      </div>

      {picked.length > 0 ? (
        <div role="region" aria-label="批量操作" className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-md bg-primary-soft px-3 py-2">
          <span className="text-sm font-medium text-ink">已选 {picked.length} 个班级</span>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" leftIcon={MoveUp} onClick={() => onPromote(picked.map((entity) => entity.id))}>
              批量升届
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>
              取消选择
            </Button>
          </div>
        </div>
      ) : null}

      {list.length === 0 ? (
        <Empty icon={Users} title="这个专业下还没有班级" description="班级是学生账号的归属,开通学生账号前先建班级。" />
      ) : (
        <>
          <div className="mt-4 flex items-center gap-3 border-b border-line px-1 pb-2 text-xs text-ink-sub">
            <Checkbox
              checked={allPicked}
              disabled={selectable.length === 0}
              aria-label="选择全部在读班级"
              onCheckedChange={(checked) => setSelected(checked === true ? new Set(selectable.map((entity) => entity.id)) : new Set())}
            />
            <span>班级 · 入学年份 · 状态</span>
          </div>
          <ul className="divide-y divide-line">
            {list.map((entity) => {
              const active = entity.status === ClassStatus.ACTIVE
              return (
                <li key={entity.id} className="flex items-center gap-3 px-1">
                  {active ? (
                    <Checkbox
                      checked={selected.has(entity.id)}
                      aria-label={`选择 ${entity.name}`}
                      onCheckedChange={(checked) => {
                        const next = new Set(selected)
                        if (checked === true) next.add(entity.id)
                        else next.delete(entity.id)
                        setSelected(next)
                      }}
                    />
                  ) : (
                    <span aria-hidden="true" className="size-4 shrink-0" />
                  )}
                  <div className="min-w-0 flex-1">
                    <ListRow
                      title={entity.name}
                      meta={`${entity.enrollment_year} 级`}
                      trailing={
                        <span className="flex items-center gap-2">
                          <StatusIndicator tone={CLASS_STATUS_TONES[entity.status]} label={CLASS_STATUS_LABELS[entity.status]} />
                          <IconButton variant="ghost" size="sm" icon={Pencil} aria-label={`编辑 ${entity.name}`} onClick={() => onEdit(entity)} />
                          <IconButton variant="ghost" size="sm" icon={Trash2} aria-label={`删除 ${entity.name}`} onClick={() => onDelete(entity)} />
                        </span>
                      }
                    />
                  </div>
                </li>
              )
            })}
          </ul>
        </>
      )}
    </div>
  )
}

/** DepartmentPane 所选院系的专业列表:「新建专业」为主操作,整行切到该专业的班级 */
function DepartmentPane({
  department,
  majors,
  onCreate,
  onOpen,
}: {
  department: Department
  majors: { major: Major; classCount: number }[]
  onCreate: () => void
  onOpen: (majorId: string) => void
}) {
  return (
    <div className="rounded-lg bg-surface p-5 shadow-xs">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-lg font-semibold text-ink">{department.name}</h2>
          <p className="mt-0.5 text-sm text-ink-sub">
            院系代码 {department.code} · {majors.length} 个专业
          </p>
        </div>
        <Button variant="primary" leftIcon={Plus} onClick={onCreate}>
          新建专业
        </Button>
      </div>
      {majors.length === 0 ? (
        <Empty icon={GraduationCap} title="这个院系下还没有专业" description="在院系下建专业,再在专业下建班级。" />
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
  const { errors, validate } = useFormValidation(() => ({
    name: name.trim() ? null : '请输入院系名称',
    code: code.trim() ? null : '请输入院系代码',
  }))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return
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
    [code, department?.id, editing, name, onSaved, validate],
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
            <FormField label="院系名称" htmlFor="dept-name" required error={errors.name}>
              <Input
                id="dept-name"
                value={name}
                invalid={Boolean(errors.name)}
                onChange={(event) => setName(event.target.value)}
                onBlur={() => validate(['name'])}
              />
            </FormField>
            <FormField label="院系代码" htmlFor="dept-code" required error={errors.code} helper="校内唯一,用于导入与对接">
              <Input
                id="dept-code"
                value={code}
                invalid={Boolean(errors.code)}
                onChange={(event) => setCode(event.target.value)}
                onBlur={() => validate(['code'])}
              />
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
  /** 新建时预选的院系(从树上「在此院系下新建专业」或院系页进入) */
  initialDepartmentId?: string
  onClose: () => void
  onSaved: () => void
}

/**
 * MajorFormModal 承载专业新建与编辑。
 */
function MajorFormModal({ major, departments, initialDepartmentId, onClose, onSaved }: MajorFormModalProps) {
  const editing = major !== undefined
  const [name, setName] = useState(major?.name ?? '')
  const [departmentId, setDepartmentId] = useState(major?.department_id ?? initialDepartmentId ?? '')
  const { errors, validate } = useFormValidation(() => ({
    name: name.trim() ? null : '请输入专业名称',
    department: departmentId ? null : '请选择所属院系',
  }))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return
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
    [departmentId, editing, major?.id, name, onSaved, validate],
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
            <FormField label="所属院系" htmlFor="major-dept" required error={errors.department}>
              <Select
                id="major-dept"
                options={departments.map((item) => ({ value: item.id, label: item.name }))}
                value={departmentId}
                invalid={Boolean(errors.department)}
                placeholder="选择院系"
                onValueChange={setDepartmentId}
                onBlur={() => validate(['department'])}
              />
            </FormField>
            <FormField label="专业名称" htmlFor="major-name" required error={errors.name}>
              <Input
                id="major-name"
                value={name}
                invalid={Boolean(errors.name)}
                onChange={(event) => setName(event.target.value)}
                onBlur={() => validate(['name'])}
              />
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
  /** 新建时预选的专业(从专业页进入) */
  initialMajorId?: string
  onClose: () => void
  onSaved: () => void
}

/**
 * ClassFormModal 承载班级新建与编辑。
 */
function ClassFormModal({ item, majors, initialMajorId, onClose, onSaved }: ClassFormModalProps) {
  const editing = item !== undefined
  const [name, setName] = useState(item?.name ?? '')
  const [majorId, setMajorId] = useState(item?.major_id ?? initialMajorId ?? '')
  const [year, setYear] = useState(String(item?.enrollment_year ?? new Date().getFullYear()))
  const [status, setStatus] = useState(String(item?.status ?? ClassStatus.ACTIVE))
  const { errors, validate } = useFormValidation(() => ({
    name: name.trim() ? null : '请输入班级名称',
    major: majorId ? null : '请选择所属专业',
    year: Number.isInteger(Number(year)) && Number(year) >= 1990 ? null : '请输入有效的入学年份',
  }))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return
      const yearValue = Number(year)
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
    [editing, item?.id, majorId, name, onSaved, status, validate, year],
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
            <FormField label="所属专业" htmlFor="class-major" required error={errors.major}>
              <Select
                id="class-major"
                options={majors.map((major) => ({ value: major.id, label: major.name }))}
                value={majorId}
                invalid={Boolean(errors.major)}
                placeholder="选择专业"
                onValueChange={setMajorId}
                onBlur={() => validate(['major'])}
              />
            </FormField>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="班级名称" htmlFor="class-name" required error={errors.name}>
                <Input
                  id="class-name"
                  value={name}
                  invalid={Boolean(errors.name)}
                  onChange={(event) => setName(event.target.value)}
                  onBlur={() => validate(['name'])}
                />
              </FormField>
              <FormField label="入学年份" htmlFor="class-year" required error={errors.year}>
                <Input
                  id="class-year"
                  type="number"
                  min="1990"
                  value={year}
                  invalid={Boolean(errors.year)}
                  onChange={(event) => setYear(event.target.value)}
                  onBlur={() => validate(['year'])}
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
  // 从专业页勾选进入:传入的班级默认全选,在这里还可以再去掉几个
  const [selected, setSelected] = useState<Set<string>>(() => new Set(classes.map((item) => item.id)))
  const [targetYear, setTargetYear] = useState(String(new Date().getFullYear()))
  const { errors, validate } = useFormValidation(() => ({
    classes: selected.size > 0 ? null : '请至少勾选一个班级',
    year: Number.isInteger(Number(targetYear)) && Number(targetYear) >= 1990 ? null : '请输入有效的目标年份',
  }))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const activeClasses = useMemo(
    () => classes.filter((item) => item.status === ClassStatus.ACTIVE),
    [classes],
  )

  const submit = useCallback(async () => {
    if (!validate()) return
    const year = Number(targetYear)
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
  }, [onDone, selected, targetYear, validate])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>批量升届</ModalTitle>
          <ModalDescription>
            把选中班级的入学年份统一改成目标年份(须晚于原来的入学年份),班里学生的入学年份一并更新;学生归属不变。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField label="目标入学年份" htmlFor="promote-year" required error={errors.year}>
            <Input
              id="promote-year"
              type="number"
              min="1990"
              value={targetYear}
              invalid={Boolean(errors.year)}
              onChange={(event) => setTargetYear(event.target.value)}
              onBlur={() => validate(['year'])}
            />
          </FormField>

          {activeClasses.length === 0 ? (
            <Empty icon={Users} title="没有在读班级" description="只有在读班级可以升届。" />
          ) : (
            <FormField label="选择班级" required error={errors.classes}>
              <div className="flex max-h-72 flex-col gap-2 overflow-y-auto rounded-md bg-surface-hover p-3">
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
  const { errors, validate } = useFormValidation(() => ({
    year: year ? null : '请选择要归档的入学年份',
  }))
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
    if (!validate()) return
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
  }, [onDone, validate, year])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>批量归档班级</ModalTitle>
          <ModalDescription>
            整届毕业时用这个:该年份的所有在读班级转为已归档,该届学生账号也一并归档并立即下线。历史数据保留,账号可以在账号管理里恢复。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          {yearOptions.length === 0 ? (
            <Empty icon={Archive} title="没有可归档的年份" description="所有班级都已归档。" />
          ) : (
            <FormField label="入学年份" htmlFor="archive-year" required error={errors.year}>
              <Select
                id="archive-year"
                options={yearOptions}
                value={year}
                invalid={Boolean(errors.year)}
                placeholder="选择年份"
                onValueChange={setYear}
                onBlur={() => validate(['year'])}
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
