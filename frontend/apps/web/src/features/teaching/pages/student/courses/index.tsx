// CoursesPage 展示学生可见课程列表，数据来自 teaching 课程接口。

import React, { useCallback, useState } from 'react'
import { CourseStatus } from '@chaimir/api-client'
import { Button, Callout, FormField, Input, Select, ResourceState } from '@chaimir/ui'
import { BookOpen, RefreshCw, UserPlus } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { studentCourseStatusFilterOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const CoursesPage: React.FC = () => {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [joining, setJoining] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => api.teaching.getCourses({
    status: status ? Number(status) as CourseStatus : undefined,
    page: 1,
    size: 20,
  }), [keyword, status])
  const rows = (resource.data?.list || []).filter((course) => (
    !keyword || course.name.includes(keyword) || course.description.includes(keyword)
  ))

  /**
   * handleJoinCourse 使用课程邀请码加入课程，并以服务端列表确认最终结果。
   */
  const handleJoinCourse = useCallback(async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const normalizedCode = inviteCode.trim()
    if (!normalizedCode) {
      setError('请输入课程邀请码。')
      return
    }
    setJoining(true)
    setMessage(null)
    setError(null)
    try {
      await api.teaching.joinCourse({ invite_code: normalizedCode })
      setInviteCode('')
      setMessage('已加入课程，课程列表已刷新。')
      resource.reload()
    } catch (joinError) {
      setError(userFacingErrorMessage(joinError, '暂时无法加入课程，请检查邀请码后重试。'))
    } finally {
      setJoining(false)
    }
  }, [inviteCode, resource])

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

      <form className={styles.joinBar} onSubmit={handleJoinCourse}>
        <FormField label="课程邀请码" htmlFor="course-invite-code" helperText="由任课教师提供">
          <Input id="course-invite-code" fullWidth value={inviteCode} onChange={(event) => setInviteCode(event.target.value)} />
        </FormField>
        <Button type="submit" icon={<UserPlus size={16} />} loading={joining}>加入课程</Button>
      </form>

      {message && <Callout variant="success" title="加入成功">{message}</Callout>}
      {error && <div className={styles.error} role="alert">{error}</div>}

      <div className={styles.toolbar}>
        <FormField label="搜索课程" htmlFor="course-keyword">
          <Input id="course-keyword" placeholder="输入课程名称" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        </FormField>
        <FormField label="课程状态" htmlFor="course-status">
          <Select id="course-status" value={status} options={studentCourseStatusFilterOptions} onChange={setStatus} />
        </FormField>
      </div>

      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取课程列表" />}
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
