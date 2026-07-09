// TeacherQuestionsPage 展示内容中心题库资源，数据来自 content 后端接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, ContentCategory, ContentItem } from '@chaimir/api-client'
import { ContentType } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table } from '@chaimir/ui'
import { Database, Edit2, FolderTree, RefreshCw, Share2 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../content.module.css'
import { contentDifficultyLabel, contentStatusLabel, contentTypeLabel, contentTypeOptions, withAllOption } from '../../../../../utils/index'



const TeacherQuestionsPage: React.FC = () => {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [type, setType] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const items = useAsyncResource(() => api.content.getItems({
    keyword: keyword || undefined,
    type: type ? Number(type) as ContentType : undefined,
    page: 1,
    size: 20,
  }), [keyword, type])
  const categories = useAsyncResource(() => api.content.listCategories(), [])

  /**
   * shareItem 调用后端共享接口，把资源加入共享库。
   */
  const shareItem = useCallback(async (item: ContentItem) => {
    setError(null)
    setMessage(null)
    try {
      await api.content.shareItem(String(item.id))
      setMessage('资源已加入共享库。')
      items.reload()
    } catch (shareError) {
      setError((shareError as ApiError).message || '共享资源失败，请稍后重试。')
    }
  }, [items])

  const categoryName = useCallback((categoryId?: number) => {
    return (categories.data || []).find((category: ContentCategory) => category.id === categoryId)?.name || '未分类'
  }, [categories.data])

  const columns = useMemo<TableColumn<ContentItem>[]>(() => [
    { key: 'title', title: '资源名称', dataIndex: 'title', priority: 'primary' },
    { key: 'type', title: '类型', render: (row) => <span className={styles.status}>{contentTypeLabel(row.type)}</span> },
    { key: 'category', title: '分类', render: (row) => categoryName(row.category_id) },
    { key: 'difficulty', title: '难度', render: (row) => contentDifficultyLabel(row.difficulty) },
    { key: 'status', title: '状态', render: (row) => contentStatusLabel(row.status) },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          <Button variant="ghost" size="sm" icon={<Edit2 size={14} />} onClick={() => navigate(`/teacher/questions/edit?id=${row.id}`)}>
            编辑
          </Button>
          <Button variant="outline" size="sm" icon={<Share2 size={14} />} onClick={() => shareItem(row)}>
            共享
          </Button>
        </div>
      ),
    },
  ], [categoryName, navigate, shareItem])

  const rows = items.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Database size={28} />
            题库内容
          </h1>
          <p className={styles.subtitle}>浏览和维护内容中心题目、实验模板与竞赛题。</p>
        </div>
        <div className={styles.toolbar}>
          <Button variant="outline" icon={<FolderTree size={16} />} onClick={() => navigate('/teacher/questions/categories')}>
            分类管理
          </Button>
          <Button onClick={() => navigate('/teacher/questions/edit')}>新建题目</Button>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="操作成功">
          {message}
        </Callout>
      )}

      <div className={styles.toolbar}>
        <Input placeholder="搜索资源名称" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <Select value={type} options={withAllOption('全部类型', contentTypeOptions)} onChange={setType} />
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={items.reload}>刷新</Button>
      </div>

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>分类</h2>
          {categories.status === 'loading' && <LoadingState title="正在获取分类" />}
          {(categories.data || []).map((category) => (
            <span className={styles.status} key={category.id}>{category.name}</span>
          ))}
        </section>
        <section>
          {items.status === 'error' && <ErrorState error={items.error} onRetry={items.reload} />}
          {items.status === 'loading' && <LoadingState title="正在获取题库资源" />}
          {(items.status === 'success' || items.status === 'empty') && (
            <div className={styles.tableWrap}>
              <Table columns={columns} rows={rows} rowKey={(row) => String(row.id)} emptyTitle="暂无资源" emptyDescription="当前没有可展示的题库资源。" ariaLabel="题库资源列表" />
            </div>
          )}
        </section>
      </div>
    </div>
  )
}

export default TeacherQuestionsPage
