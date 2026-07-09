// CoursesPage 展示学生可见课程列表，数据来自 teaching 课程接口。

import React, { useState } from 'react'
import { CourseStatus } from '@chaimir/api-client'
import { Button, Input, Select } from '@chaimir/ui'
import { BookOpen, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { studentCourseStatusFilterOptions } from '../../../../../utils/index'

const CoursesPage: React.FC = () => {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState('')
  const resource = useAsyncResource(() => api.teaching.getCourses({
    status: status ? Number(status) as CourseStatus : undefined,
    page: 1,
    size: 20,
  }), [keyword, status])
  const rows = (resource.data?.list || []).filter((course) => (
    !keyword || course.name.includes(keyword) || course.description.includes(keyword)
  ))

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <BookOpen size={28} />
            我的课程
          </h1>
          <p className={styles.subtitle}>查看并进入已加入的安全课程。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>

      <div className={styles.toolbar}>
        <Input placeholder="搜索课程" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <Select value={status} options={studentCourseStatusFilterOptions} onChange={setStatus} />
      </div>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取课程列表" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.cardGrid}>
          {rows.map((course) => (
            <article className={styles.card} key={course.id}>
              <span className={styles.status}>{course.semester}</span>
              <h2>{course.name}</h2>
              <p className={styles.muted}>{course.description || '暂无课程简介'}</p>
              <div className={styles.actions}>
                <span className={styles.muted}>{course.credits} 学分</span>
                <span className={styles.muted}>{course.invite_code ? `邀请码 ${course.invite_code}` : '已加入'}</span>
              </div>
              <Button variant="outline" onClick={() => navigate(`/student/courses/${course.id}`)}>
                继续学习
              </Button>
            </article>
          ))}
          {rows.length === 0 && (
            <section className={styles.panel}>
              <h2>暂无课程</h2>
              <p className={styles.muted}>当前还没有可学习的课程。</p>
            </section>
          )}
        </div>
      )}
    </div>
  )
}

export default CoursesPage
