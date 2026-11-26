// 管理多级题库分类；仅允许删除无子分类和题目引用的空分类。

import { useState } from 'react'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { FolderTree, Pencil, Plus, Trash2 } from 'lucide-react'
import type { ContentCategory } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  DataPanel,
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

/** CategoryRow 保留完整路径,使深层分类在窄屏仍可辨认。 */
interface CategoryRow {
  category: ContentCategory
  depth: number
  childCount: number
  path: string
}

/** ContentCategories 管理题库分类树。 */
export function ContentCategories({ onChanged }: { onChanged?: () => void }) {
  const [formTarget, setFormTarget] = useState<{ category?: ContentCategory } | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<CategoryRow>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const categories = useAsyncResource(() => api.content.listCategories(), [])

  const rows = buildRows(categories.data ?? [])

  // 删除失败保留当前选择,便于处理引用后重试。
  async function removeCategory() {
    if (!deleteTarget) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.content.deleteCategory(deleteTarget.category.id)
      toast.success('分类已删除')
      setDeleteTarget(undefined)
      categories.reload()
      onChanged?.()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '删除没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }

  const columns: TableColumn<CategoryRow>[] = [
    {
      key: 'name',
      header: '分类路径',
      render: (row) => (
        <div className="flex flex-wrap items-center gap-2">
          <span className="break-words font-medium text-ink">{row.path}</span>
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
            disabled={row.childCount > 0}
            onClick={() => {
              setActionError(undefined)
              setDeleteTarget(row)
            }}
          />
        </div>
      ),
    },
  ]

  return (
    <PageSection
      description="按知识领域分层组织题目。删除前请先调整子分类和题目归类。"
      actions={
        <Button variant="secondary" leftIcon={Plus} onClick={() => setFormTarget({})}>
          新建分类
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <DataPanel label="题库分类">
          <ResourceState
            resource={categories}
            emptyIcon={FolderTree}
            emptyTitle="还没有分类"
            emptyDescription="分类不是必填项,但按领域分类后检索与组卷会更方便。"
            emptyAction={
              <Button variant="secondary" leftIcon={Plus} onClick={() => setFormTarget({})}>
                新建分类
              </Button>
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {() => (
              <Table
                columns={columns}
                data={rows}
                rowKey={(row) => row.category.id}
                elevated={false}
                mobileCard={(row) => ({
                  title: row.path,
                  meta: `第 ${row.depth + 1} 层 · 排序 ${row.category.sort}${row.childCount > 0 ? ` · ${row.childCount} 个子分类` : ''}`,
                  action: (
                    <div className="flex gap-1">
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
                })}
                empty={<Empty icon={FolderTree} title="还没有分类" />}
              />
            )}
          </ResourceState>
        </DataPanel>
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
            onChanged?.()
          }}
        />
      ) : null}

      <Modal open={deleteTarget !== undefined} onOpenChange={(open) => !open && setDeleteTarget(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>确认删除分类</ModalTitle>
            <ModalDescription>
              仅可删除没有子分类、没有题目引用的空分类。
            </ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="text-base text-ink">{deleteTarget?.category.name}</p>
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

/** buildRows 按父子顺序展开完整路径,选择父分类时排除自身及整棵子树。 */
function buildRows(categories: ContentCategory[], excludedId?: string): CategoryRow[] {
  const childrenByParent = new Map<string, ContentCategory[]>()

  for (const category of categories) {
    const parentId = category.parent_id ?? ''
    const children = childrenByParent.get(parentId) ?? []
    children.push(category)
    childrenByParent.set(parentId, children)
  }
  for (const children of childrenByParent.values()) children.sort((a, b) => a.sort - b.sort)
  const pending = (childrenByParent.get('') ?? [])
    .map((category) => ({ category, depth: 0, path: category.name }))
    .reverse()
  const rows: CategoryRow[] = []

  while (pending.length > 0) {
    const row = pending.pop()!
    if (row.category.id === excludedId) continue
    const children = childrenByParent.get(row.category.id) ?? []
    rows.push({ ...row, childCount: children.length })
    for (let index = children.length - 1; index >= 0; index--) {
      const category = children[index]!
      pending.push({ category, depth: row.depth + 1, path: `${row.path} / ${category.name}` })
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

/** CategoryFormModal 编辑名称、排序和可用父分类,防环仍由服务端确认。 */
function CategoryFormModal({
  category,
  categories,
  nextSort,
  onClose,
  onSaved,
}: CategoryFormModalProps) {
  const editing = category !== undefined
  const [name, setName] = useState(category?.name ?? '')
  const [parentId, setParentId] = useState(category?.parent_id ?? '__root__')
  const [sort, setSort] = useState(String(category?.sort ?? nextSort))
  const [submitError, setSubmitError] = useState<string>()
  const [working, setWorking] = useState(false)
  const sortValue = sort.trim() === '' ? nextSort : Number(sort)
  const { errors, validate } = useFormValidation(() => ({
    name: !name.trim() ? '请输入分类名称' : null,
    sort: !Number.isInteger(sortValue) || sortValue < -2147483648 || sortValue > 2147483647
      ? '排序请填写有效范围内的整数'
      : null,
  }))

  const parentOptions = [
    { value: '__root__', label: '作为顶级分类' },
    ...buildRows(categories, category?.id).map((row) => ({ value: row.category.id, label: row.path })),
  ]

  // 保存失败保留表单输入,不清除用户选择的父分类。
  async function submit(event: React.FormEvent<HTMLFormElement>) {
      event.preventDefault()
      if (!validate()) return
      setSubmitError(undefined)
      setWorking(true)
      try {
        const payload = {
          parent_id: parentId === '__root__' ? undefined : parentId,
          name: name.trim(),
          sort: sortValue,
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
        setSubmitError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
  }

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑分类' : '新建分类'}</ModalTitle>
          <ModalDescription>可在已有分类下继续分层,不能将分类移入自己的下级。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="分类名称" htmlFor="category-name" required error={errors.name}>
              <Input
                id="category-name"
                value={name}
                invalid={Boolean(errors.name)}
                onChange={(event) => setName(event.target.value)}
                onBlur={() => validate(['name'])}
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
            <FormField label="排序" htmlFor="category-sort" error={errors.sort} helper="数字越小越靠前">
              <Input
                id="category-sort"
                type="number"
                min="-2147483648"
                max="2147483647"
                value={sort}
                invalid={Boolean(errors.sort)}
                onChange={(event) => setSort(event.target.value)}
                onBlur={() => validate(['sort'])}
              />
            </FormField>
            {submitError ? <Callout tone="danger">{submitError}</Callout> : null}
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
