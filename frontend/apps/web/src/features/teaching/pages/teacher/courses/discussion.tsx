// TeacherCourseDiscussionPage 维护课程公告和讨论帖，所有动作调用 teaching 后端。

import React, { useCallback, useMemo, useState } from 'react'
import type { TeachingAnnouncement, TeachingPost } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Table, Textarea } from '@chaimir/ui'
import { Heart, MessageSquare, Pin, RefreshCw, Send, Trash2 } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { formatDateTime } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherCourseDiscussionPage: React.FC = () => {
  const { id } = useParams()
  const courseId = String(id || '')
  const announcements = useAsyncResource(() => api.teaching.listAnnouncements(courseId), [courseId])
  const posts = useAsyncResource(() => api.teaching.listPosts(courseId, { page: 1, size: 20 }), [courseId])
  const [title, setTitle] = useState('')
  const [announcementContent, setAnnouncementContent] = useState('')
  const [postContent, setPostContent] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const reloadAll = useCallback(() => {
    announcements.reload()
    posts.reload()
  }, [announcements, posts])

  const createAnnouncement = useCallback(async () => {
    setError(null)
    setMessage(null)
    try {
      await api.teaching.createAnnouncement(courseId, { title, content: announcementContent, is_pinned: false })
      setTitle('')
      setAnnouncementContent('')
      setMessage('课程公告已发布。')
      announcements.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '课程公告发布失败，请稍后重试。'))
    }
  }, [announcementContent, announcements, courseId, title])

  const createPost = useCallback(async () => {
    setError(null)
    setMessage(null)
    try {
      await api.teaching.createPost(courseId, { content: postContent })
      setPostContent('')
      setMessage('讨论帖已发布。')
      posts.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '讨论帖发布失败，请稍后重试。'))
    }
  }, [courseId, postContent, posts])

  const postAction = useCallback(async (action: () => Promise<TeachingPost | void>, successMessage: string) => {
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(successMessage)
      posts.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '讨论帖操作失败，请稍后重试。'))
    }
  }, [posts])

  /**
   * announcementAction 单独处理公告动作，避免和讨论帖返回类型混用。
   */
  const announcementAction = useCallback(async (action: () => Promise<TeachingAnnouncement>, successMessage: string) => {
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(successMessage)
      announcements.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '公告操作失败，请稍后重试。'))
    }
  }, [announcements])

  const announcementColumns = useMemo<TableColumn<TeachingAnnouncement>[]>(() => [
    { key: 'title', title: '公告标题', dataIndex: 'title', priority: 'primary' },
    { key: 'content', title: '内容', dataIndex: 'content' },
    { key: 'pinned', title: '置顶', render: (row) => (row.is_pinned ? '是' : '否') },
    { key: 'created', title: '发布时间', render: (row) => formatDateTime(row.created_at) },
    { key: 'actions', title: '操作', render: (row) => <Button variant="outline" size="sm" icon={<Pin size={14} />} onClick={() => announcementAction(() => api.teaching.pinAnnouncement(String(row.id)), '公告已置顶。')}>置顶</Button> },
  ], [announcementAction])

  const postColumns = useMemo<TableColumn<TeachingPost>[]>(() => [
    { key: 'content', title: '讨论内容', dataIndex: 'content', priority: 'primary' },
    { key: 'likes', title: '点赞', dataIndex: 'like_count' },
    { key: 'pinned', title: '置顶', render: (row) => (row.is_pinned ? '是' : '否') },
    { key: 'created', title: '发布时间', render: (row) => formatDateTime(row.created_at) },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          <Button variant="outline" size="sm" icon={<Pin size={14} />} onClick={() => postAction(() => api.teaching.pinPost(String(row.id)), '讨论帖已置顶。')}>置顶</Button>
          <Button variant="ghost" size="sm" icon={<Heart size={14} />} onClick={() => postAction(() => api.teaching.likePost(String(row.id)), '已点赞讨论帖。')}>点赞</Button>
          <Button variant="ghost" size="sm" icon={<Trash2 size={14} />} onClick={() => postAction(() => api.teaching.deletePost(String(row.id)), '讨论帖已删除。')}>删除</Button>
        </div>
      ),
    },
  ], [postAction])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><MessageSquare size={28} />答疑与公告</h1>
          <p className={styles.subtitle}>发布课程公告并维护学生讨论帖。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={reloadAll}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>发布公告</h2>
          <label className={styles.field}>标题<Input fullWidth value={title} onChange={(event) => setTitle(event.target.value)} /></label>
          <label className={styles.field}>正文<Textarea value={announcementContent} onChange={(event) => setAnnouncementContent(event.target.value)} /></label>
          <Button icon={<Send size={16} />} onClick={createAnnouncement}>发布公告</Button>
        </section>
        <section className={styles.panel}>
          <h2>发布讨论帖</h2>
          <label className={styles.field}>内容<Textarea value={postContent} onChange={(event) => setPostContent(event.target.value)} /></label>
          <Button icon={<Send size={16} />} onClick={createPost}>发布讨论帖</Button>
        </section>
      </div>

      {announcements.status === 'error' && <ErrorState error={announcements.error} onRetry={announcements.reload} />}
      {announcements.status === 'loading' && <LoadingState title="正在获取课程公告" />}
      {(announcements.status === 'success' || announcements.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={announcementColumns} rows={announcements.data || []} rowKey="id" emptyTitle="暂无公告" emptyDescription="当前课程还没有公告。" ariaLabel="课程公告列表" />
        </div>
      )}

      {posts.status === 'error' && <ErrorState error={posts.error} onRetry={posts.reload} />}
      {posts.status === 'loading' && <LoadingState title="正在获取讨论帖" />}
      {(posts.status === 'success' || posts.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={postColumns} rows={posts.data?.list || []} rowKey="id" emptyTitle="暂无讨论" emptyDescription="当前课程还没有讨论帖。" ariaLabel="课程讨论帖列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherCourseDiscussionPage
