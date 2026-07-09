// TeacherCoursesPage 展示教师课程列表，数据来自 teaching 课程接口。

import React, { useState } from 'react'
import { CourseStatus } from '@chaimir/api-client'
import { Button, Input, Select } from '@chaimir/ui'
import { Book, Edit, Plus, RefreshCw, Users } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { courseStatusLabel, courseStatusOptions, withAllOption } from '../../../../../utils/index'


const TeacherCoursesPage: React.FC = () => {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState('')
  const resource = useAsyncResource(() => api.teaching.getCourses({
    role: 'teacher',
    status: status ? Number(status) as CourseStatus : undefined,
    page: 1,
    size: 20,
  }), [status])
  const rows = (resource.data?.list || []).filter((course) => !keyword || course.name.includes(keyword) || course.description.includes(keyword))

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Book size={28} />
            课程管理
          </h1>
          <p className={styles.subtitle}>维护当前教师负责的课程、章节和成员。</p>
        </div>
        <Button icon={<Plus size={16} />} onClick={() => navigate('/teacher/courses/edit')}>新建课程</Button>
      </div>

      <div className={styles.toolbar}>
        <Input placeholder="搜索课程" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <Select value={status} options={withAllOption('全部状态', courseStatusOptions)} onChange={setStatus} />
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取课程列表" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.cardGrid}>
          {rows.map((course) => (
            <article className={styles.card} key={course.id}>
              <div className={styles.actions}>
                <span className={styles.status}>{courseStatusLabel(course.status)}</span>
                <span className={styles.muted}>{course.semester}</span>
              </div>
              <h2>{course.name}</h2>
              <p className={styles.muted}>{course.description || '暂无课程简介'}</p>
              <div className={styles.actions}>
                <Button variant="outline" size="sm" icon={<Edit size={14} />} onClick={() => navigate(`/teacher/courses/${course.id}/outline`)}>大纲</Button>
                <Button variant="outline" size="sm" icon={<Users size={14} />} onClick={() => navigate(`/teacher/courses/${course.id}/members`)}>成员</Button>
                <Button variant="ghost" size="sm" onClick={() => navigate(`/teacher/courses/edit?id=${course.id}`)}>编辑</Button>
              </div>
            </article>
          ))}
          {rows.length === 0 && (
            <section className={styles.panel}>
              <h2>暂无课程</h2>
              <p className={styles.muted}>当前筛选条件下没有课程。</p>
            </section>
          )}
        </div>
      )}
    </div>
  )
}

export default TeacherCoursesPage
