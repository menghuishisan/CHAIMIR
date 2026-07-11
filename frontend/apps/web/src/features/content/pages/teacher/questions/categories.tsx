// TeacherQuestionCategoriesPage 维护内容中心分类树，调用 content 分类接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ContentCategory } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table } from '@chaimir/ui'
import { FolderTree, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../content.module.css'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherQuestionCategoriesPage: React.FC = () => {
  const resource = useAsyncResource(() => api.content.listCategories(), [])
  const [name, setName] = useState('')
  const [parentId, setParentId] = useState('0')
  const [sort, setSort] = useState('0')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [editingId, setEditingId] = useState('')

  const parentOptions = useMemo(() => [
    { value: '0', label: '顶级分类' },
    ...(resource.data || []).map((category) => ({ value: String(category.id), label: category.name })),
  ], [resource.data])

  /**
   * handleSave 创建或更新分类。
   */
  const handleSave = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      const payload = {
        parent_id: Number(parentId),
        name: name.trim(),
        sort: Number(sort),
      }
      if (editingId) await api.content.updateCategory(editingId, payload)
      else await api.content.createCategory(payload)
      setName('')
      setEditingId('')
      setMessage(editingId ? '分类已更新。' : '分类已创建。')
      resource.reload()
    } catch (createError) {
      setError(userFacingErrorMessage(createError, '分类保存失败，请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [editingId, name, parentId, resource, sort])

  /** deleteCategory 删除未被资源或子分类引用的分类。 */
  const deleteCategory = useCallback(async (category: ContentCategory) => {
    if (!window.confirm(`确定删除分类“${category.name}”吗？`)) return
    setError(null)
    try {
      await api.content.deleteCategory(String(category.id))
      setMessage('分类已删除。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '分类删除失败，请先处理其下资源。'))
    }
  }, [resource])

  const columns = useMemo<TableColumn<ContentCategory>[]>(() => [
    { key: 'name', title: '分类名称', dataIndex: 'name', priority: 'primary' },
    { key: 'parent', title: '上级分类', render: (row) => row.parent_id ? String(row.parent_id) : '顶级' },
    { key: 'sort', title: '排序', dataIndex: 'sort' },
    { key: 'actions', title: '操作', render: (row) => <div className={styles.actions}><Button variant="outline" size="sm" icon={<Pencil size={14} />} onClick={() => { setEditingId(String(row.id)); setName(row.name); setParentId(String(row.parent_id || 0)); setSort(String(row.sort)) }}>编辑</Button><Button variant="ghost" size="sm" icon={<Trash2 size={14} />} onClick={() => void deleteCategory(row)}>删除</Button></div> },
  ], [deleteCategory])

  const rows = resource.data || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <FolderTree size={28} />
            分类层级维护
          </h1>
          <p className={styles.subtitle}>分类由后端统一维护，题目创建和筛选复用同一分类 ID。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}

      <section className={styles.panel}>
        <h2>{editingId ? '编辑分类' : '新建分类'}</h2>
        <div className={styles.formGrid}>
          <label className={styles.field}>
            上级分类
            <Select fullWidth value={parentId} options={parentOptions} onChange={setParentId} />
          </label>
          <label className={styles.field}>
            分类名称
            <Input fullWidth value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <label className={styles.field}>
            排序值
            <Input fullWidth value={sort} onChange={(event) => setSort(event.target.value)} />
          </label>
        </div>
        <Button loading={submitting} icon={<Plus size={16} />} onClick={handleSave}>{editingId ? '保存分类' : '创建分类'}</Button>
      </section>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取分类" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey={(row) => String(row.id)} emptyTitle="暂无分类" emptyDescription="当前还没有内容分类。" ariaLabel="内容分类列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherQuestionCategoriesPage
