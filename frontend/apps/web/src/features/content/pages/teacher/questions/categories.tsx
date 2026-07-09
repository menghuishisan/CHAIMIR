// TeacherQuestionCategoriesPage 维护内容中心分类树，调用 content 分类接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, ContentCategory } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table } from '@chaimir/ui'
import { FolderTree, Plus, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../content.module.css'

const TeacherQuestionCategoriesPage: React.FC = () => {
  const resource = useAsyncResource(() => api.content.listCategories(), [])
  const [name, setName] = useState('')
  const [parentId, setParentId] = useState('0')
  const [sort, setSort] = useState('0')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const parentOptions = useMemo(() => [
    { value: '0', label: '顶级分类' },
    ...(resource.data || []).map((category) => ({ value: String(category.id), label: category.name })),
  ], [resource.data])

  /**
   * handleCreate 创建新分类。
   */
  const handleCreate = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      await api.content.createCategory({
        parent_id: Number(parentId),
        name: name.trim(),
        sort: Number(sort),
      })
      setName('')
      setMessage('分类已创建。')
      resource.reload()
    } catch (createError) {
      setError((createError as ApiError).message || '分类创建失败，请稍后重试。')
    } finally {
      setSubmitting(false)
    }
  }, [name, parentId, resource, sort])

  const columns = useMemo<TableColumn<ContentCategory>[]>(() => [
    { key: 'name', title: '分类名称', dataIndex: 'name', priority: 'primary' },
    { key: 'parent', title: '上级分类', render: (row) => row.parent_id ? String(row.parent_id) : '顶级' },
    { key: 'sort', title: '排序', dataIndex: 'sort' },
  ], [])

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
        <h2>新建分类</h2>
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
        <Button loading={submitting} icon={<Plus size={16} />} onClick={handleCreate}>创建分类</Button>
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
