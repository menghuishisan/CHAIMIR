// TeacherSharedPage 浏览跨课共享资源库，数据来自 content 共享库接口。

import React, { useMemo, useState } from 'react'
import type { ContentItem } from '@chaimir/api-client'
import { ContentType } from '@chaimir/api-client'
import { Button, Input, Select } from '@chaimir/ui'
import { RefreshCw, Share2 } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../content.module.css'
import { contentTypeLabel, contentTypeOptions, withAllOption } from '../../../../../utils/index'


const TeacherSharedPage: React.FC = () => {
  const [keyword, setKeyword] = useState('')
  const [type, setType] = useState('')
  const resource = useAsyncResource(() => api.content.listShared({
    keyword: keyword || undefined,
    type: type ? Number(type) as ContentType : undefined,
    page: 1,
    size: 20,
  }), [keyword, type])

  const rows = useMemo<ContentItem[]>(() => resource.data?.list || [], [resource.data])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Share2 size={28} />
            共享资源库
          </h1>
          <p className={styles.subtitle}>发现其他课程共享的题目、竞赛题和实验模板。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>

      <div className={styles.toolbar}>
        <Input placeholder="搜索共享资源" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <Select value={type} options={withAllOption('全部类型', contentTypeOptions)} onChange={setType} />
      </div>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取共享资源" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.cardGrid}>
          {rows.map((item) => (
            <article className={styles.card} key={item.id}>
              <div className={styles.actions}>
                <span className={styles.status}>{contentTypeLabel(item.type)}</span>
                <span className={styles.muted}>使用 {item.usage_count} 次</span>
              </div>
              <h2>{item.title}</h2>
              <p className={styles.muted}>编号 {item.code} / 版本 {item.version}</p>
              <div className={styles.actions}>
                {item.tags.map((tag) => <span className={styles.status} key={tag}>{tag}</span>)}
              </div>
            </article>
          ))}
          {rows.length === 0 && (
            <section className={styles.panel}>
              <h2>暂无共享资源</h2>
              <p className={styles.muted}>当前筛选条件下没有共享资源。</p>
            </section>
          )}
        </div>
      )}
    </div>
  )
}

export default TeacherSharedPage
