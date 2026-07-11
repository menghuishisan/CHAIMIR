// AnnouncementsPage 发布学校公告，并展示后端公告列表。

import React, { useCallback, useMemo, useState } from 'react'
import type { Announcement } from '@chaimir/api-client'
import { AnnouncementScope, UserRole } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table, Textarea } from '@chaimir/ui'
import { Megaphone, RefreshCw, Send } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from './announcements.module.css'
import { announcementScopeLabel, announcementScopeOptions, announcementTargetRoleOptions, formatDateTime } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


const AnnouncementsPage: React.FC = () => {
  const resource = useAsyncResource(() => api.notify.getAnnouncements({ page: 1, size: 20 }), [])
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [scope, setScope] = useState(String(AnnouncementScope.TENANT))
  const [targetRole, setTargetRole] = useState(String(UserRole.STUDENT))
  const [expireAt, setExpireAt] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /**
   * handleSubmit 调用公告发布接口，目标角色由后端负责鉴权和投递。
   */
  const handleSubmit = useCallback(async () => {
    if (!title.trim() || !content.trim()) {
      setError('请填写公告标题和正文。')
      return
    }
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      await api.notify.createAnnouncement({
        title: title.trim(),
        content: content.trim(),
        scope: Number(scope) as AnnouncementScope,
        target_roles: Number(scope) === AnnouncementScope.ROLES ? [Number(targetRole) as UserRole] : [],
        expire_at: expireAt || undefined,
      })
      setTitle('')
      setContent('')
      setExpireAt('')
      setMessage('公告已发布。')
      resource.reload()
    } catch (submitError) {
      setError(userFacingErrorMessage(submitError, '公告发布失败，请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [content, expireAt, resource, scope, targetRole, title])

  const columns = useMemo<TableColumn<Announcement>[]>(() => [
    { key: 'title', title: '标题', dataIndex: 'title', priority: 'primary' },
    {
      key: 'scope',
      title: '范围',
      render: (row) => announcementScopeLabel(row.scope),
    },
    {
      key: 'publishedAt',
      title: '发布时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.published_at)}</span>,
    },
    {
      key: 'expireAt',
      title: '过期时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.expire_at)}</span>,
    },
  ], [])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Megaphone className={styles.icon} size={28} />
            系统公告
          </h1>
          <p className={styles.subtitle}>发布学校公告，并查看已经投递的公告记录。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="公告已发布">
          {message}
        </Callout>
      )}

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>发布公告</h2>
          <label>
            公告标题
            <Input fullWidth value={title} placeholder="请输入公告标题" onChange={(event) => setTitle(event.target.value)} />
          </label>
          <label>
            覆盖范围
            <Select fullWidth value={scope} options={announcementScopeOptions} onChange={setScope} />
          </label>
          {Number(scope) === AnnouncementScope.ROLES && (
            <label>
              目标角色
              <Select fullWidth value={targetRole} options={announcementTargetRoleOptions} onChange={setTargetRole} />
            </label>
          )}
          <label>
            过期时间
            <Input fullWidth type="datetime-local" value={expireAt} onChange={(event) => setExpireAt(event.target.value)} />
          </label>
          <label>
            公告正文
            <Textarea value={content} placeholder="请输入公告正文" onChange={(event) => setContent(event.target.value)} />
          </label>
          <Button loading={submitting} icon={<Send size={16} />} onClick={handleSubmit}>
            发布公告
          </Button>
        </section>

        <section className={styles.panel}>
          <h2>公告记录</h2>
          {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
          {resource.status === 'loading' && <LoadingState title="正在获取公告记录" />}
          {(resource.status === 'success' || resource.status === 'empty') && (
            <Table
              columns={columns}
              rows={rows}
              rowKey="id"
              emptyTitle="暂无公告"
              emptyDescription="当前还没有发布过系统公告。"
              ariaLabel="系统公告列表"
            />
          )}
        </section>
      </div>
    </div>
  )
}

export default AnnouncementsPage
