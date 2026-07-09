// TeacherOrganizationPage 只读展示当前租户组织结构，复用 identity 后端组织数据。

import React, { useCallback } from 'react'
import { Button } from '@chaimir/ui'
import { FolderTree, RefreshCw, Users } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'

const TeacherOrganizationPage: React.FC = () => {
  const departments = useAsyncResource(() => api.identity.listDepartments(), [])
  const majors = useAsyncResource(() => api.identity.listMajors(), [])
  const classes = useAsyncResource(() => api.identity.listClasses(), [])

  /**
   * reloadAll 刷新教师端只读组织树。
   */
  const reloadAll = useCallback(() => {
    departments.reload()
    majors.reload()
    classes.reload()
  }, [classes, departments, majors])

  const isLoading = departments.status === 'loading' || majors.status === 'loading' || classes.status === 'loading'
  const firstError = departments.error || majors.error || classes.error

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Users size={28} />校内组织树</h1>
          <p className={styles.subtitle}>教师端只读查看院系、专业和班级结构。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={reloadAll}>刷新</Button>
      </div>

      {firstError && <ErrorState error={firstError} onRetry={reloadAll} />}
      {isLoading && <LoadingState title="正在同步组织结构" />}

      <section className={styles.panel}>
        <h2><FolderTree size={18} />组织架构</h2>
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
          {!isLoading && (departments.data || []).length === 0 && (
            <span className={styles.muted}>暂无组织结构数据。</span>
          )}
        </div>
      </section>
    </div>
  )
}

export default TeacherOrganizationPage
