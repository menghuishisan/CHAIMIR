// OrganizationPage 展示并维护当前租户的院系、专业和班级组织结构。

import React, { useCallback, useMemo, useState } from 'react'
import { ClassStatus } from '@chaimir/api-client'
import { Button, Callout, Checkbox, Input, Select } from '@chaimir/ui'
import { Archive, Network, Pencil, Plus, RefreshCw, Trash2, TrendingUp } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { classStatusOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { OrganizationImportPanel } from './OrganizationImportPanel'

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
  const [editingDepartmentId, setEditingDepartmentId] = useState('')
  const [editingMajorId, setEditingMajorId] = useState('')
  const [editingClassId, setEditingClassId] = useState('')
  const [archiveYear, setArchiveYear] = useState('')
  const [promotionYear, setPromotionYear] = useState('')
  const [selectedClassIds, setSelectedClassIds] = useState<Set<string>>(new Set())
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
      setError(userFacingErrorMessage(actionError, '组织信息保存失败，请稍后重试。'))
    } finally {
      setSubmitting(null)
    }
  }, [reloadAll])

  /** promoteSelectedClasses 仅升级管理员明确勾选且目标学年有效的班级。 */
  const promoteSelectedClasses = useCallback(() => {
    const targetYear = Number(promotionYear)
    if (selectedClassIds.size === 0 || !Number.isInteger(targetYear) || targetYear <= 0) {
      setError('请选择需要升级的班级并填写目标学年。')
      return
    }
    if (!window.confirm(`确定将选中的 ${selectedClassIds.size} 个班级升级到 ${targetYear} 学年吗？`)) return
    void submitOrgAction('promote', async () => {
      await api.identity.promoteClasses({ class_ids: Array.from(selectedClassIds), target_year: targetYear })
      setSelectedClassIds(new Set())
      setPromotionYear('')
      setEditingClassId('')
      setClassName('')
      setClassMajorId('')
      setEnrollmentYear('')
      setClassStatus(String(ClassStatus.ACTIVE))
    }, '所选班级已升级。')
  }, [promotionYear, selectedClassIds, submitOrgAction])

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
          <p className={styles.subtitle}>统一维护院系、专业和班级，供账号归属选择使用。</p>
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
                <div className={styles.actions}><Button variant="ghost" size="sm" icon={<Pencil size={13} />} onClick={() => { setEditingDepartmentId(department.id); setDepartmentName(department.name); setDepartmentCode(department.code) }}>编辑</Button><Button variant="ghost" size="sm" icon={<Trash2 size={13} />} onClick={() => submitOrgAction(`delete-department-${department.id}`, () => api.identity.deleteDepartment(department.id), '院系已删除。')}>删除</Button></div>
                <div className={styles.treeChildren}>
                  {(majors.data || []).filter((major) => major.department_id === department.id).map((major) => (
                    <div className={styles.treeItem} key={major.id}>
                      {major.name}
                      <div className={styles.actions}><Button variant="ghost" size="sm" icon={<Pencil size={13} />} onClick={() => { setEditingMajorId(major.id); setMajorName(major.name); setMajorDepartmentId(major.department_id) }}>编辑</Button><Button variant="ghost" size="sm" icon={<Trash2 size={13} />} onClick={() => submitOrgAction(`delete-major-${major.id}`, () => api.identity.deleteMajor(major.id), '专业已删除。')}>删除</Button></div>
                      <div className={styles.treeChildren}>
                        {(classes.data || []).filter((classItem) => classItem.major_id === major.id).map((classItem) => (
                          <span className={styles.status} key={classItem.id}>
                            {classItem.status === ClassStatus.ACTIVE && <Checkbox checked={selectedClassIds.has(classItem.id)} aria-label={`选择${classItem.name}`} onChange={(event) => setSelectedClassIds((current) => { const next = new Set(current); if (event.target.checked) next.add(classItem.id); else next.delete(classItem.id); return next })} />}
                            {classItem.name} · {classItem.enrollment_year}级
                            <Button variant="ghost" size="sm" icon={<Pencil size={13} />} onClick={() => { setEditingClassId(classItem.id); setClassName(classItem.name); setClassMajorId(classItem.major_id); setEnrollmentYear(String(classItem.enrollment_year)); setClassStatus(String(classItem.status)) }}>编辑</Button>
                            <Button variant="ghost" size="sm" icon={<Trash2 size={13} />} onClick={() => submitOrgAction(`delete-class-${classItem.id}`, () => api.identity.deleteClass(classItem.id), '班级已删除。')}>删除</Button>
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
          <h2>{editingDepartmentId ? '编辑院系' : '新增院系'}</h2>
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
            onClick={() => submitOrgAction('department', () => editingDepartmentId ? api.identity.updateDepartment(editingDepartmentId, { name: departmentName, code: departmentCode }) : api.identity.createDepartment({ name: departmentName, code: departmentCode }), editingDepartmentId ? '院系已更新。' : '院系已创建。')}
          >
            {editingDepartmentId ? '保存院系' : '创建院系'}
          </Button>
        </section>

        <section className={styles.panel}>
          <h2>{editingMajorId ? '编辑专业' : '新增专业'}</h2>
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
            onClick={() => submitOrgAction('major', () => editingMajorId ? api.identity.updateMajor(editingMajorId, { department_id: majorDepartmentId, name: majorName }) : api.identity.createMajor({ department_id: majorDepartmentId, name: majorName }), editingMajorId ? '专业已更新。' : '专业已创建。')}
          >
            {editingMajorId ? '保存专业' : '创建专业'}
          </Button>
        </section>

        <section className={styles.panel}>
          <h2>{editingClassId ? '编辑班级' : '新增班级'}</h2>
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
            onClick={() => submitOrgAction('class', () => (editingClassId ? api.identity.updateClass(editingClassId, {
              major_id: classMajorId,
              name: className,
              enrollment_year: Number(enrollmentYear),
              status: Number(classStatus) as ClassStatus,
            }) : api.identity.createClass({
              major_id: classMajorId,
              name: className,
              enrollment_year: Number(enrollmentYear),
              status: Number(classStatus) as ClassStatus,
            })), editingClassId ? '班级已更新。' : '班级已创建。')}
          >
            {editingClassId ? '保存班级' : '创建班级'}
          </Button>
        </section>
        <section className={styles.panel}>
          <h2>年级流转</h2>
          <label className={styles.field}>入学年份<Input fullWidth value={archiveYear} onChange={(event) => setArchiveYear(event.target.value)} /></label>
          <label className={styles.field}>升级目标学年<Input fullWidth value={promotionYear} onChange={(event) => setPromotionYear(event.target.value)} /></label>
          <div className={styles.actions}>
            <Button variant="outline" icon={<Archive size={15} />} disabled={!Number(archiveYear)} onClick={() => submitOrgAction('archive', () => api.identity.archiveClasses({ enrollment_year: Number(archiveYear) }), '对应年级班级已归档。')}>归档年级</Button>
            <Button variant="outline" icon={<TrendingUp size={15} />} disabled={selectedClassIds.size === 0 || !Number(promotionYear)} onClick={promoteSelectedClasses}>升级所选班级</Button>
          </div>
        </section>
        <OrganizationImportPanel onCompleted={reloadAll} />
      </div>
    </div>
  )
}

export default OrganizationPage
