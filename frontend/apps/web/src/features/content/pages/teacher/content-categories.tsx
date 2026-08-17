// 题库分类管理(题库内容页内区块)。
// 分类是树:父分类下挂子分类。删除父分类会连带下级,故走确认弹层
// (旧前端删除院系/专业/班级全无确认,是被审查列为 P0 的问题)。

import { useCallback, useMemo, useState } from 'react'
import { FolderTree, Pencil, Plus, Trash2 } from 'lucide-react'
import type { ContentCategory } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
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
  PageSection,
  Select,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** CategoryRow 是分类表的一行:带层级深度用于缩进呈现树形。 */
interface CategoryRow {
  category: ContentCategory
  depth: number
  childCount: number
}

/**
 * ContentCategories 管理题库分类树。
 */
export function ContentCategories() {
  const [formTarget, setFormTarget] = useState<{ category?: ContentCategory } | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<CategoryRow>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const categories = useAsyncResource(() => api.content.listCategories(), [])

  const rows = useMemo(() => buildRows(categories.data ?? []), [categories.data])

  const removeCategory = useCallback(async () => {
    if (!deleteTarget) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.content.deleteCategory(deleteTarget.category.id)
      toast.success('分类已删除')
      setDeleteTarget(undefined)
      categories.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '删除没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [categories, deleteTarget])

  const columns: TableColumn<CategoryRow>[] = [
    {
      key: 'name',
      header: '分类名称',
      render: (row) => (
        <div className="flex items-center gap-2" style={{ paddingLeft: `${row.depth * 20}px` }}>
          <span className="font-medium text-ink">{row.category.name}</span>
          {row.childCount > 0 ? <Badge tone="neutral">{row.childCount} 个子分类</Badge> : null}
        </div>
      ),
    },
    { key: 'sort', header: '排序', align: 'right', mono: true, render: (row) => row.category.sort },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (row) => (
        <div className="flex justify-end gap-1">
          <IconButton
            variant="ghost"
            size="sm"
            icon={Pencil}
            aria-label={`编辑分类 ${row.category.name}`}
            onClick={() => setFormTarget({ category: row.category })}
          />
          <IconButton
            variant="ghost"
            size="sm"
            icon={Trash2}
            aria-label={`删除分类 ${row.category.name}`}
            onClick={() => setDeleteTarget(row)}
          />
        </div>
      ),
    },
  ]

  return (
    <PageSection
      title="题库分类"
      description="按知识领域组织题目。分类支持两级,便于检索与组卷。"
      actions={
        <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
          新建分类
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <ResourceState
          resource={categories}
          emptyIcon={FolderTree}
          emptyTitle="还没有分类"
          emptyDescription="分类不是必填项,但按领域分类后检索与组卷会更方便。"
          emptyAction={
            <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
              新建分类
            </Button>
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {() => (
            <Table
              columns={columns}
              data={rows}
              rowKey={(row) => row.category.id}
              empty={<Empty icon={FolderTree} title="还没有分类" />}
            />
          )}
        </ResourceState>
      </div>

      {formTarget ? (
        <CategoryFormModal
          category={formTarget.category}
          categories={categories.data ?? []}
          nextSort={(categories.data?.length ?? 0) * 10 + 10}
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            categories.reload()
          }}
        />
      ) : null}

      <Modal open={deleteTarget !== undefined} onOpenChange={(open) => !open && setDeleteTarget(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>确认删除分类</ModalTitle>
            <ModalDescription>
              {deleteTarget && deleteTarget.childCount > 0
                ? '这个分类下还有子分类,删除会连带删除它们。已归入这些分类的题目不会被删除,只是失去分类。'
                : '已归入这个分类的题目不会被删除,只是失去分类。'}
            </ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="text-base text-ink">{deleteTarget?.category.name}</p>
            {deleteTarget && deleteTarget.childCount > 0 ? (
              <Callout tone="warning" className="mt-3">
                将连带删除 {deleteTarget.childCount} 个子分类。
              </Callout>
            ) : null}
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(undefined)}>
              取消
            </Button>
            <Button variant="danger" loading={working} onClick={() => void removeCategory()}>
              确认删除
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </PageSection>
  )
}

/**
 * buildRows 把扁平分类列表按父子关系摊平成带缩进的行。
 * 后端只回扁平数组与 parent_id,树形结构在展示层构建。
 */
function buildRows(categories: ContentCategory[]): CategoryRow[] {
  const childrenByParent = new Map<string, ContentCategory[]>()
  const roots: ContentCategory[] = []

  for (const category of categories) {
    if (category.parent_id) {
      const list = childrenByParent.get(category.parent_id) ?? []
      list.push(category)
      childrenByParent.set(category.parent_id, list)
    } else {
      roots.push(category)
    }
  }

  const sortBySort = (a: ContentCategory, b: ContentCategory) => a.sort - b.sort
  const rows: CategoryRow[] = []

  for (const root of roots.sort(sortBySort)) {
    const children = (childrenByParent.get(root.id) ?? []).sort(sortBySort)
    rows.push({ category: root, depth: 0, childCount: children.length })
    for (const child of children) {
      rows.push({ category: child, depth: 1, childCount: 0 })
    }
  }

  return rows
}

interface CategoryFormModalProps {
  category?: ContentCategory
  categories: ContentCategory[]
  nextSort: number
  onClose: () => void
  onSaved: () => void
}

/**
 * CategoryFormModal 承载分类创建与编辑。
 * 父分类只能选顶级分类:分类支持两级,避免层级过深让检索变复杂。
 */
function CategoryFormModal({
  category,
  categories,
  nextSort,
  onClose,
  onSaved,
}: CategoryFormModalProps) {
  const editing = category !== undefined
  const [name, setName] = useState(category?.name ?? '')
  const [parentId, setParentId] = useState(category?.parent_id ?? '')
  const [sort, setSort] = useState(String(category?.sort ?? nextSort))
  const [fieldError, setFieldError] = useState<string>()
  const [working, setWorking] = useState(false)

  const parentOptions = useMemo(
    () => [
      { value: '', label: '作为顶级分类' },
      ...categories
        .filter((item) => !item.parent_id && item.id !== category?.id)
        .map((item) => ({ value: item.id, label: item.name })),
    ],
    [categories, category?.id],
  )

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (name.trim() === '') {
        setFieldError('请输入分类名称')
        return
      }
      setFieldError(undefined)
      setWorking(true)
      try {
        const payload = {
          parent_id: parentId === '' ? undefined : parentId,
          name: name.trim(),
          sort: Number(sort) || nextSort,
        }
        if (editing) {
          await api.content.updateCategory(category.id, payload)
          toast.success('分类已更新')
        } else {
          await api.content.createCategory(payload)
          toast.success('分类已创建')
        }
        onSaved()
      } catch (error) {
        setFieldError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [category?.id, editing, name, nextSort, onSaved, parentId, sort],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑分类' : '新建分类'}</ModalTitle>
          <ModalDescription>分类用于按知识领域组织题目,支持两级。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="分类名称" htmlFor="category-name" required error={fieldError}>
              <Input
                id="category-name"
                value={name}
                invalid={Boolean(fieldError)}
                onChange={(event) => setName(event.target.value)}
              />
            </FormField>
            <FormField label="父分类" htmlFor="category-parent">
              <Select
                id="category-parent"
                options={parentOptions}
                value={parentId}
                onValueChange={setParentId}
              />
            </FormField>
            <FormField label="排序" htmlFor="category-sort" helper="数字越小越靠前">
              <Input
                id="category-sort"
                type="number"
                min="0"
                value={sort}
                onChange={(event) => setSort(event.target.value)}
              />
            </FormField>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              {editing ? '保存修改' : '创建分类'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}
