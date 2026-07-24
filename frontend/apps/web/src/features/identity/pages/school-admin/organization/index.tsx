// OrganizationPage 展示并维护当前租户的院系、专业和班级组织结构。

import React, { useCallback, useMemo, useState } from 'react'
import { ClassStatus } from '@chaimir/api-client'
import { Button, Callout, Checkbox, Input, Select, Tabs, useConfirm, ResourceState, FormField } from '@chaimir/ui'
import { Archive, Network, Pencil, Plus, RefreshCw, Trash2, TrendingUp } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { classStatusOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { OrganizationImportPanel } from './OrganizationImportPanel'

const OrganizationPage: React.FC = () => {
  const confirm = useConfirm()
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
  const [activePanel, setActivePanel] = useState('tree')

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

  /** confirmOrgAction 在删除或归档组织节点前说明级联影响。 */
  const confirmOrgAction = useCallback(async (title: string, description: string, key: string, action: () => Promise<unknown>, successMessage: string) => {
    const confirmed = await confirm({ title, description, confirmLabel: '确认继续' })
    if (confirmed) await submitOrgAction(key, action, successMessage)
  }, [confirm, submitOrgAction])

  /** promoteSelectedClasses 仅升级管理员明确勾选且目标学年有效的班级。 */
  const promoteSelectedClasses = useCallback(async () => {
    const targetYear = Number(promotionYear)
    if (selectedClassIds.size === 0 || !Number.isInteger(targetYear) || targetYear <= 0) {
      setError('请选择需要升级的班级并填写目标学年。')
      return
    }
    const confirmed = await confirm({
      title: '升级所选班级',
      description: `将 ${selectedClassIds.size} 个班级升级到 ${targetYear} 学年，确定继续吗？`,
      confirmLabel: '确认升级',
      confirmVariant: 'primary',
    })
    if (!confirmed) return
    await submitOrgAction('promote', async () => {
      await api.identity.promoteClasses({ class_ids: Array.from(selectedClassIds), target_year: targetYear })
      setSelectedClassIds(new Set())
      setPromotionYear('')
      setEditingClassId('')
      setClassName('')
      setClassMajorId('')
      setEnrollmentYear('')
      setClassStatus(String(ClassStatus.ACTIVE))
    }, '所选班级已升级。')
  }, [confirm, promotionYear, selectedClassIds, submitOrgAction])

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
      {firstError && <ResourceState status="error" error={firstError} onRetry={reloadAll} />}
      {isLoading && <ResourceState status="loading" title="正在同步组织结构" />}

      <Tabs
        activeKey={activePanel}
        ariaLabel="组织管理任务"
        items={[
          { key: 'tree', label: '组织结构' },
          { key: 'department', label: '院系' },
          { key: 'major', label: '专业' },
          { key: 'class', label: '班级' },
          { key: 'lifecycle', label: '年级流转' },
          { key: 'import', label: '批量导入' },
        ]}
        onChange={setActivePanel}
      >
        <div className={styles.organizationWorkspace}>
        {activePanel === 'tree' && <section className={styles.panel}>
          <h2>组织树</h2>
          <div className={styles.tree}>
            {(departments.data || []).map((department) => (
              <div className={styles.treeItem} key={department.id}>
                <strong>{department.name}</strong>
                <span className={styles.muted}> 编码 {department.code}</span>
                <div className={styles.actions}><Button variant="ghost" size="sm" icon={<Pencil size={13} />} onClick={() => { setEditingDepartmentId(department.id); setDepartmentName(department.name); setDepartmentCode(department.code) }}>编辑</Button><Button variant="ghost" size="sm" icon={<Trash2 size={13} />} onClick={() => confirmOrgAction('删除院系', `只有不再包含专业的院系“${department.name}”才能删除。`, `delete-department-${department.id}`, () => api.identity.deleteDepartment(department.id), '院系已删除。')}>删除</Button></div>
                <div className={styles.treeChildren}>
                  {(majors.data || []).filter((major) => major.department_id === department.id).map((major) => (
                    <div className={styles.treeItem} key={major.id}>
                      {major.name}
                      <div className={styles.actions}><Button variant="ghost" size="sm" icon={<Pencil size={13} />} onClick={() => { setEditingMajorId(major.id); setMajorName(major.name); setMajorDepartmentId(major.department_id) }}>编辑</Button><Button variant="ghost" size="sm" icon={<Trash2 size={13} />} onClick={() => confirmOrgAction('删除专业', `只有不再包含班级的专业“${major.name}”才能删除。`, `delete-major-${major.id}`, () => api.identity.deleteMajor(major.id), '专业已删除。')}>删除</Button></div>
                      <div className={styles.treeChildren}>
                        {(classes.data || []).filter((classItem) => classItem.major_id === major.id).map((classItem) => (
                          <span className={styles.status} key={classItem.id}>
                            {classItem.status === ClassStatus.ACTIVE && <Checkbox checked={selectedClassIds.has(classItem.id)} aria-label={`选择${classItem.name}`} onChange={(event) => setSelectedClassIds((current) => { const next = new Set(current); if (event.target.checked) next.add(classItem.id); else next.delete(classItem.id); return next })} />}
                            {classItem.name} · {classItem.enrollment_year}级
                            <Button variant="ghost" size="sm" icon={<Pencil size={13} />} onClick={() => { setEditingClassId(classItem.id); setClassName(classItem.name); setClassMajorId(classItem.major_id); setEnrollmentYear(String(classItem.enrollment_year)); setClassStatus(String(classItem.status)) }}>编辑</Button>
                            <Button variant="ghost" size="sm" icon={<Trash2 size={13} />} onClick={() => confirmOrgAction('删除班级', `只有没有关联账号的班级“${classItem.name}”才能删除。`, `delete-class-${classItem.id}`, () => api.identity.deleteClass(classItem.id), '班级已删除。')}>删除</Button>
                          </span>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </section>}

        {activePanel === 'department' && <section className={styles.panel}>
          <h2>{editingDepartmentId ? '编辑院系' : '新增院系'}</h2>
          <FormField className={styles.field} label="院系名称"><Input fullWidth value={departmentName} onChange={(event) => setDepartmentName(event.target.value)} /></FormField>
          <FormField className={styles.field} label="院系编码"><Input fullWidth value={departmentCode} onChange={(event) => setDepartmentCode(event.target.value)} /></FormField>
          <Button
            loading={submitting === 'department'}
            icon={<Plus size={16} />}
            onClick={() => submitOrgAction('department', () => editingDepartmentId ? api.identity.updateDepartment(editingDepartmentId, { name: departmentName, code: departmentCode }) : api.identity.createDepartment({ name: departmentName, code: departmentCode }), editingDepartmentId ? '院系已更新。' : '院系已创建。')}
          >
            {editingDepartmentId ? '保存院系' : '创建院系'}
          </Button>
        </section>}

        {activePanel === 'major' && <section className={styles.panel}>
          <h2>{editingMajorId ? '编辑专业' : '新增专业'}</h2>
          <FormField className={styles.field} label="所属院系"><Select fullWidth value={majorDepartmentId} options={departmentOptions} onChange={setMajorDepartmentId} /></FormField>
          <FormField className={styles.field} label="专业名称"><Input fullWidth value={majorName} onChange={(event) => setMajorName(event.target.value)} /></FormField>
          <Button
            loading={submitting === 'major'}
            icon={<Plus size={16} />}
            onClick={() => submitOrgAction('major', () => editingMajorId ? api.identity.updateMajor(editingMajorId, { department_id: majorDepartmentId, name: majorName }) : api.identity.createMajor({ department_id: majorDepartmentId, name: majorName }), editingMajorId ? '专业已更新。' : '专业已创建。')}
          >
            {editingMajorId ? '保存专业' : '创建专业'}
          </Button>
        </section>}

        {activePanel === 'class' && <section className={styles.panel}>
          <h2>{editingClassId ? '编辑班级' : '新增班级'}</h2>
          <FormField className={styles.field} label="所属专业"><Select fullWidth value={classMajorId} options={majorOptions} onChange={setClassMajorId} /></FormField>
          <FormField className={styles.field} label="班级名称"><Input fullWidth value={className} onChange={(event) => setClassName(event.target.value)} /></FormField>
          <FormField className={styles.field} label="入学年份"><Input fullWidth value={enrollmentYear} onChange={(event) => setEnrollmentYear(event.target.value)} /></FormField>
          <FormField className={styles.field} label="班级状态"><Select fullWidth value={classStatus} options={classStatusOptions} onChange={setClassStatus} /></FormField>
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
        </section>}
        {activePanel === 'lifecycle' && <section className={styles.panel}>
          <h2>年级流转</h2>
          <FormField className={styles.field} label="入学年份"><Input fullWidth value={archiveYear} onChange={(event) => setArchiveYear(event.target.value)} /></FormField>
          <FormField className={styles.field} label="升级目标学年"><Input fullWidth value={promotionYear} onChange={(event) => setPromotionYear(event.target.value)} /></FormField>
          <div className={styles.actions}>
            <Button variant="outline" icon={<Archive size={15} />} disabled={!Number(archiveYear)} onClick={() => confirmOrgAction('归档年级班级', `将 ${archiveYear} 级的全部班级移出当前组织结构。`, 'archive', () => api.identity.archiveClasses({ enrollment_year: Number(archiveYear) }), '对应年级班级已归档。')}>归档年级</Button>
            <Button variant="outline" icon={<TrendingUp size={15} />} disabled={selectedClassIds.size === 0 || !Number(promotionYear)} onClick={promoteSelectedClasses}>升级所选班级</Button>
          </div>
        </section>}
        {activePanel === 'import' && <OrganizationImportPanel onCompleted={reloadAll} />}
        </div>
      </Tabs>
    </div>
  )
}

export default OrganizationPage
