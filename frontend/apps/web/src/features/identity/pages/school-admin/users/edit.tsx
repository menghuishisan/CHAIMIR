// UserEditPage 新建或编辑租户账号，复用 identity 后端账号与组织接口。

import React, { useCallback, useEffect, useMemo, useState } from 'react'
import type { Account } from '@chaimir/api-client'
import { BaseIdentity, ClassStatus } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Switch } from '@chaimir/ui'
import { Edit, Save, ShieldCheck } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { baseIdentityOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

/**
 * firstRoleIdentity 根据账号角色推断基础身份。
 */
function firstRoleIdentity(account?: Account): BaseIdentity {
  if (!account) {
    return BaseIdentity.STUDENT
  }
  return account.base_identity
}

const UserEditPage: React.FC = () => {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const accountId = searchParams.get('id')
  const accountsResource = useAsyncResource(() => api.identity.getAccounts({ page: 1, size: 100 }), [])
  const departmentsResource = useAsyncResource(() => api.identity.listDepartments(), [])
  const majorsResource = useAsyncResource(() => api.identity.listMajors(), [])
  const classesResource = useAsyncResource(() => api.identity.listClasses(), [])
  const tenantConfigResource = useAsyncResource(() => api.identity.getTenantConfig(), [])

  const account = useMemo(
    () => accountsResource.data?.list.find((item) => item.id === accountId),
    [accountId, accountsResource.data],
  )

  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [no, setNo] = useState('')
  const [baseIdentity, setBaseIdentity] = useState(String(BaseIdentity.STUDENT))
  const [orgId, setOrgId] = useState('')
  const [enrollmentYear, setEnrollmentYear] = useState('')
  const [title, setTitle] = useState('')
  const [initialPassword, setInitialPassword] = useState('')
  const [useActivation, setUseActivation] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [resetPassword, setResetPassword] = useState('')
  const [mustChangePassword, setMustChangePassword] = useState(true)

  useEffect(() => {
    if (!accountId && tenantConfigResource.data) {
      setUseActivation(tenantConfigResource.data.enable_activation_code)
    }
  }, [accountId, tenantConfigResource.data])

  const loadedIdentity = firstRoleIdentity(account)
  const effectiveName = name || account?.name || ''
  const effectiveNo = no || account?.no || ''
  const effectiveTitle = title || account?.title || ''
  const effectiveBaseIdentity = baseIdentity || String(loadedIdentity)

  const orgOptions = useMemo(() => {
    const departmentOptions = (departmentsResource.data || []).map((department) => ({
      value: department.id,
      label: `${department.name}（院系）`,
    }))
    const classOptions = (classesResource.data || []).map((classItem) => ({
      value: classItem.id,
      label: `${classItem.name}（${classItem.enrollment_year}级）`,
    }))
    return [{ value: '', label: '请选择归属组织' }, ...departmentOptions, ...classOptions]
  }, [classesResource.data, departmentsResource.data])

  /**
   * hydrateForm 从后端账号填充编辑表单，避免渲染时直接 setState。
   */
  const hydrateForm = useCallback(() => {
    if (!account) {
      return
    }
    setName(account.name)
    setNo(account.no || '')
    setBaseIdentity(String(account.base_identity))
    setTitle(account.title || '')
  }, [account])

  /**
   * handleSubmit 根据是否有账号 id 调用创建或更新接口。
   */
  const handleSubmit = useCallback(async () => {
    setError(null)
    setMessage(null)
    if (!effectiveName.trim() || !effectiveNo.trim() || !orgId) {
      setError('请填写姓名、学号工号并选择归属组织。')
      return
    }
    setSubmitting(true)
    try {
      if (accountId) {
        await api.identity.updateAccount(accountId, {
          name: effectiveName.trim(),
          org_id: orgId,
          enrollment_year: enrollmentYear ? Number(enrollmentYear) : undefined,
          title: effectiveTitle.trim() || undefined,
        })
        setMessage('账号资料已保存。')
      } else {
        const result = await api.identity.createAccount({
          phone: phone.trim(),
          name: effectiveName.trim(),
          no: effectiveNo.trim(),
          base_identity: Number(effectiveBaseIdentity) as BaseIdentity,
          org_id: orgId,
          enrollment_year: enrollmentYear ? Number(enrollmentYear) : undefined,
          title: effectiveTitle.trim() || undefined,
          initial_password: initialPassword || undefined,
          use_activation: useActivation,
        })
        setMessage(result.activation_code ? `账号已创建，激活码 ${result.activation_code}` : '账号已创建。')
      }
    } catch (submitError) {
      setError(userFacingErrorMessage(submitError, '账号保存失败，请检查信息后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [accountId, effectiveBaseIdentity, effectiveName, effectiveNo, effectiveTitle, enrollmentYear, initialPassword, orgId, phone, useActivation])

  /** runAccountAdminAction 执行账号权限或密码动作并统一展示结果。 */
  const runAccountAdminAction = async (action: () => Promise<void>, success: string) => {
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(success)
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '账号操作失败，请稍后重试。'))
    }
  }

  if (accountId && accountsResource.status === 'loading') {
    return <LoadingState title="正在获取账号资料" />
  }
  if (accountsResource.status === 'error') {
    return <ErrorState error={accountsResource.error} onRetry={accountsResource.reload} />
  }
  if (!accountId && tenantConfigResource.status === 'loading') {
    return <LoadingState title="正在获取账号开通策略" />
  }
  if (!accountId && tenantConfigResource.status === 'error') {
    return <ErrorState error={tenantConfigResource.error} onRetry={tenantConfigResource.reload} />
  }
  if (accountId && !account) {
    return (
      <div className={styles.page}>
        <Callout variant="warning" title="未找到账号">
          当前账号不存在，或您没有查看权限。
        </Callout>
        <Button variant="outline" onClick={() => navigate('/school-admin/users')}>
          返回账号列表
        </Button>
      </div>
    )
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Edit size={28} />
            {accountId ? '编辑账号' : '新增账号'}
          </h1>
          <p className={styles.subtitle}>维护当前学校账号资料和组织归属。</p>
        </div>
        {accountId && (
          <Button variant="outline" onClick={hydrateForm}>
            使用当前资料填充
          </Button>
        )}
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="保存成功">
          {message}
        </Callout>
      )}

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>基础资料</h2>
          <div className={styles.formGrid}>
            <label className={styles.field}>
              姓名
              <Input fullWidth value={effectiveName} onChange={(event) => setName(event.target.value)} />
            </label>
            <label className={styles.field}>
              学号或工号
              <Input fullWidth value={effectiveNo} onChange={(event) => setNo(event.target.value)} />
            </label>
            {!accountId && (
              <label className={styles.field}>
                手机号
                <Input fullWidth value={phone} onChange={(event) => setPhone(event.target.value)} />
              </label>
            )}
            <label className={styles.field}>
              基础身份
              <Select fullWidth disabled={Boolean(accountId)} value={effectiveBaseIdentity} options={baseIdentityOptions} onChange={setBaseIdentity} />
            </label>
            <label className={styles.field}>
              归属组织
              <Select fullWidth value={orgId} options={orgOptions} onChange={setOrgId} />
            </label>
            <label className={styles.field}>
              入学年份
              <Input fullWidth value={enrollmentYear} placeholder="学生账号可填写" onChange={(event) => setEnrollmentYear(event.target.value)} />
            </label>
            <label className={styles.fieldFull}>
              职称
              <Input fullWidth value={effectiveTitle} placeholder="教师账号可填写" onChange={(event) => setTitle(event.target.value)} />
            </label>
          </div>
          <Button loading={submitting} icon={<Save size={16} />} onClick={handleSubmit}>
            保存账号
          </Button>
        </section>

        {!accountId && (
          <section className={styles.panel}>
            <h2>开通方式</h2>
            <Switch checked={useActivation} disabled={!tenantConfigResource.data?.enable_activation_code} label={useActivation ? '使用激活码开通' : '使用初始密码开通'} onChange={(event) => setUseActivation(event.target.checked)} />
            {!useActivation && (
              <label className={styles.field}>
                初始密码
                <Input fullWidth type="password" value={initialPassword} onChange={(event) => setInitialPassword(event.target.value)} />
              </label>
            )}
            <Callout variant="info" title="开通说明">
              {useActivation ? '激活码只在创建结果中展示一次，请按学校流程安全下发。' : '账号创建后使用初始密码登录，并按学校安全要求及时修改密码。'}
            </Callout>
          </section>
        )}

        {accountId && (
          <section className={styles.panel}>
            <h2>权限动作</h2>
            <Button variant="outline" icon={<ShieldCheck size={16} />} onClick={() => void runAccountAdminAction(() => api.identity.grantSchoolAdmin(accountId), '已授予学校管理员权限。')}>
              授权为学校管理员
            </Button>
            <Button variant="outline" onClick={() => void runAccountAdminAction(() => api.identity.revokeSchoolAdmin(accountId), '已撤销学校管理员权限。')}>
              撤销学校管理员
            </Button>
            <label className={styles.field}>新密码<Input fullWidth type="password" value={resetPassword} onChange={(event) => setResetPassword(event.target.value)} /></label>
            <Switch checked={mustChangePassword} label="下次登录必须修改密码" onChange={(event) => setMustChangePassword(event.target.checked)} />
            <Button variant="outline" disabled={!resetPassword} onClick={() => void runAccountAdminAction(() => api.identity.resetAccountPassword(accountId, { new_password: resetPassword, must_change_pwd: mustChangePassword }), '账号密码已重置。')}>重置密码</Button>
          </section>
        )}
      </div>

      {majorsResource.status === 'loading' || classesResource.status === 'loading' ? (
        <LoadingState title="正在同步组织信息" />
      ) : null}
      {classesResource.data?.some((item) => item.status === ClassStatus.ARCHIVED) && (
        <Callout variant="info" title="组织提示">
          已归档班级仍会展示在组织列表中，账号保存时由后端判断是否允许绑定。
        </Callout>
      )}
    </div>
  )
}

export default UserEditPage
