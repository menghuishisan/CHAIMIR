// OrganizationPage 展示并维护当前租户的院系、专业和班级组织结构。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError } from '@chaimir/api-client'
import { ClassStatus } from '@chaimir/api-client'
import { Button, Callout, Input, Select } from '@chaimir/ui'
import { Network, Plus, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { classStatusOptions } from '../../../../../utils/index'

const OrganizationPage: React.FC = () => {
  const departments = useAsyncResource(() => api.identity.listDepartments(), [])
  const majors = useAsyncResource(() => api.identity.listMajors(), [])
  const classes = useAsyncResource(() => api.identity.listClasses(), [])
  const [departmentName, setDepartmentName] = useState('')
  const [departmentCode, setDepartmentCode] = useState('')
  const [majorName, setMajorName] = useState('')
  const [majorDepartmentId, setMajorDepartmentId] = useState('')
  const [className, setClassName] = useState('')
  const [classMajorId, setClassMajorId] = useState('')
  const [enrollmentYear, setEnrollmentYear] = useState('')
  const [classStatus, setClassStatus] = useState(String(ClassStatus.ACTIVE))
  const [submitting, setSubmitting] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const departmentOptions = useMemo(() => [
    { value: '', label: '选择院系' },
    ...(departments.data || []).map((department) => ({ value: department.id, label: department.name })),
  ], [departments.data])

  const majorOptions = useMemo(() => [
    { value: '', label: '选择专业' },
    ...(majors.data || []).map((major) => ({ value: major.id, label: major.name })),
  ], [majors.data])

  /**
   * reloadAll 刷新组织三层数据。
   */
  const reloadAll = useCallback(() => {
    departments.reload()
    majors.reload()
    classes.reload()
  }, [classes, departments, majors])

  /**
   * submitOrgAction 执行组织结构写入并统一反馈。
   */
  const submitOrgAction = useCallback(async (key: string, action: () => Promise<unknown>, successMessage: string) => {
    setSubmitting(key)
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(successMessage)
      reloadAll()
    } catch (actionError) {
      setError((actionError as ApiError).message || '组织信息保存失败，请稍后重试。')
    } finally {
      setSubmitting(null)
    }
  }, [reloadAll])

  const isLoading = departments.status === 'loading' || majors.status === 'loading' || classes.status === 'loading'
  const firstError = departments.error || majors.error || classes.error

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Network size={28} />
            院系专业管理
          </h1>
          <p className={styles.subtitle}>组织结构数据来自后端，账号绑定时复用同一套组织 ID。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={reloadAll}>
          刷新
        </Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="保存成功">
          {message}
        </Callout>
      )}
      {firstError && <ErrorState error={firstError} onRetry={reloadAll} />}
      {isLoading && <LoadingState title="正在同步组织结构" />}

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>组织树</h2>
          <div className={styles.tree}>
            {(departments.data || []).map((department) => (
              <div className={styles.treeItem} key={department.id}>
                <strong>{department.name}</strong>
                <span className={styles.muted}> 编码 {department.code}</span>
                <div className={styles.treeChildren}>
                  {(majors.data || []).filter((major) => major.department_id === department.id).map((major) => (
                    <div className={styles.treeItem} key={major.id}>
                      {major.name}
                      <div className={styles.treeChildren}>
                        {(classes.data || []).filter((classItem) => classItem.major_id === major.id).map((classItem) => (
                          <span className={styles.status} key={classItem.id}>
                            {classItem.name} · {classItem.enrollment_year}级
                          </span>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className={styles.panel}>
          <h2>新增院系</h2>
          <label className={styles.field}>
            院系名称
            <Input fullWidth value={departmentName} onChange={(event) => setDepartmentName(event.target.value)} />
          </label>
          <label className={styles.field}>
            院系编码
            <Input fullWidth value={departmentCode} onChange={(event) => setDepartmentCode(event.target.value)} />
          </label>
          <Button
            loading={submitting === 'department'}
            icon={<Plus size={16} />}
            onClick={() => submitOrgAction('department', () => api.identity.createDepartment({ name: departmentName, code: departmentCode }), '院系已创建。')}
          >
            创建院系
          </Button>
        </section>

        <section className={styles.panel}>
          <h2>新增专业</h2>
          <label className={styles.field}>
            所属院系
            <Select fullWidth value={majorDepartmentId} options={departmentOptions} onChange={setMajorDepartmentId} />
          </label>
          <label className={styles.field}>
            专业名称
            <Input fullWidth value={majorName} onChange={(event) => setMajorName(event.target.value)} />
          </label>
          <Button
            loading={submitting === 'major'}
            icon={<Plus size={16} />}
            onClick={() => submitOrgAction('major', () => api.identity.createMajor({ department_id: majorDepartmentId, name: majorName }), '专业已创建。')}
          >
            创建专业
          </Button>
        </section>

        <section className={styles.panel}>
          <h2>新增班级</h2>
          <label className={styles.field}>
            所属专业
            <Select fullWidth value={classMajorId} options={majorOptions} onChange={setClassMajorId} />
          </label>
          <label className={styles.field}>
            班级名称
            <Input fullWidth value={className} onChange={(event) => setClassName(event.target.value)} />
          </label>
          <label className={styles.field}>
            入学年份
            <Input fullWidth value={enrollmentYear} onChange={(event) => setEnrollmentYear(event.target.value)} />
          </label>
          <label className={styles.field}>
            班级状态
            <Select fullWidth value={classStatus} options={classStatusOptions} onChange={setClassStatus} />
          </label>
          <Button
            loading={submitting === 'class'}
            icon={<Plus size={16} />}
            onClick={() => submitOrgAction('class', () => api.identity.createClass({
              major_id: classMajorId,
              name: className,
              enrollment_year: Number(enrollmentYear),
              status: Number(classStatus) as ClassStatus,
            }), '班级已创建。')}
          >
            创建班级
          </Button>
        </section>
      </div>
    </div>
  )
}

export default OrganizationPage
