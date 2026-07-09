// CourseDetailPage 展示课程章节课时，数据来自 teaching 课程大纲接口。

import React, { useMemo } from 'react'
import { ProgressStatus } from '@chaimir/api-client'
import { Button } from '@chaimir/ui'
import { CheckCircle, FileText, Info, Play } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'

const CourseDetailPage: React.FC = () => {
  const navigate = useNavigate()
  const { id } = useParams()
  const resource = useAsyncResource(() => api.teaching.getCourseOutline(String(id)), [id])
  const outline = resource.data

  const progressMap = useMemo(() => new Map((outline?.progress || []).map((item) => [item.lesson_id, item.status])), [outline])

  if (!id) {
    return <EmptyState title="缺少课程信息" description="当前链接没有课程编号。" />
  }
  if (resource.status === 'loading') {
    return <LoadingState title="正在获取课程详情" />
  }
  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }
  if (!outline) {
    return <EmptyState title="暂无课程详情" description="当前课程暂未开放章节课时。" />
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Info size={28} />
            {outline.course.name}
          </h1>
          <p className={styles.subtitle}>{outline.course.description || '暂无课程简介'}</p>
        </div>
        <Button onClick={() => navigate('/student/courses/assignment/1')}>查看课程作业</Button>
      </div>

      <div className={styles.outline}>
        {outline.chapters.map((chapter) => (
          <section className={styles.chapter} key={chapter.id}>
            <div className={styles.chapterHeader}>{chapter.title}</div>
            {outline.lessons.filter((lesson) => lesson.chapter_id === chapter.id).map((lesson) => {
              const done = progressMap.get(lesson.id) === ProgressStatus.DONE
              return (
                <button className={styles.lessonRow} key={lesson.id} type="button" onClick={() => navigate(`/student/courses/${outline.course.id}/lesson/${lesson.id}`)}>
                  {done ? <CheckCircle size={18} /> : <Play size={18} />}
                  <span>{lesson.title}</span>
                  <span className={styles.muted}>{done ? '已完成' : '继续学习'}</span>
                </button>
              )
            })}
          </section>
        ))}
        {outline.chapters.length === 0 && (
          <section className={styles.panel}>
            <FileText size={28} />
            <h2>暂无章节</h2>
            <p className={styles.muted}>教师尚未发布课程章节。</p>
          </section>
        )}
      </div>
    </div>
  )
}

export default CourseDetailPage
