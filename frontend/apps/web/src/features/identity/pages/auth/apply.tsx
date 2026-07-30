// ApplyPage 学校入驻申请(公共页,底层全裸露形态;仅 SaaS 形态注册):
// 提交学校与联系人信息(POST platform/applications,免认证),由平台运营人员线下审核。
// 单页表单:当前公开契约没有服务端草稿接口,不拆成会丢失中间状态的前端向导(规范 §9)。
// 机构类型枚举与 tenantApplicationSchoolTypeLabel 同源,避免前后端两套文案。

import React, { useCallback, useId, useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import { api } from '../../../../app/api'
import {
  TENANT_APPLICATION_SCHOOL_TYPES,
  tenantApplicationSchoolTypeLabel,
  type TenantApplicationSchoolType,
} from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { emailError, phoneError, requiredError, useFieldErrors } from './auth-form'
import {
  AuthField,
  AuthFormError,
  AuthHeading,
  AuthPanel,
  AuthQuietLink,
  AuthSubmit,
  AuthSuccess,
  AuthTextField,
} from './auth-ui'

/**
 * ApplyPage 处理学校入驻申请提交。
 */
export default function ApplyPage() {
  const fieldId = useId()
  const [schoolName, setSchoolName] = useState('')
  const [schoolType, setSchoolType] = useState<TenantApplicationSchoolType>(TENANT_APPLICATION_SCHOOL_TYPES[0])
  const [contactName, setContactName] = useState('')
  const [contactPhone, setContactPhone] = useState('')
  const [contactEmail, setContactEmail] = useState('')
  const { errors, setError } = useFieldErrors()
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)

  /** handleSubmit 提交入驻申请 */
  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const nameOk = setError('schoolName', requiredError(schoolName, '请输入学校名称'))
      const contactOk = setError('contactName', requiredError(contactName, '请输入联系人姓名'))
      const phoneOk = setError('contactPhone', phoneError(contactPhone, '联系电话'))
      const emailOk = setError('contactEmail', emailError(contactEmail))
      if (!nameOk || !contactOk || !phoneOk || !emailOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        await api.identity.createApplication({
          school_name: schoolName.trim(),
          school_type: schoolType,
          contact_name: contactName.trim(),
          contact_phone: contactPhone.trim(),
          contact_email: contactEmail.trim(),
        })
        setDone(true)
      } catch (applyError) {
        setFormError(userFacingErrorMessage(applyError, '申请提交失败,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [contactEmail, contactName, contactPhone, schoolName, schoolType, setError],
  )

  if (done) {
    return (
      <AuthPanel>
        {/* 尾链是弱化通路:申请人此刻还没有账号,登录不是他的下一步(下一步是等平台联系) */}
        <AuthSuccess
          title="申请已提交"
          description="平台运营人员会通过您留下的电话或邮箱与您联系,请留意来电与邮件。"
        >
          <AuthQuietLink to="/auth/login" icon={ArrowLeft}>
            返回登录
          </AuthQuietLink>
        </AuthSuccess>
      </AuthPanel>
    )
  }

  return (
    <AuthPanel>
      <form className="w-full max-w-md" onSubmit={handleSubmit} noValidate>
        <AuthHeading
          title="申请学校入驻"
          description="留下学校与联系人信息,平台运营人员会与您联系确认后开通。"
        />

        <AuthTextField
          label="学校名称"
          id={`${fieldId}-school`}
          autoComplete="organization"
          value={schoolName}
          error={errors.schoolName}
          onValueChange={setSchoolName}
          onBlur={() => setError('schoolName', requiredError(schoolName, '请输入学校名称'))}
        />

        {/* 机构类型:深色语境下用原生 select(设计系统 Select 为浅面板专用) */}
        <AuthField label="机构类型" htmlFor={`${fieldId}-type`}>
          <select
            id={`${fieldId}-type`}
            value={String(schoolType)}
            // 选项只由 TENANT_APPLICATION_SCHOOL_TYPES 渲染,取值必然落在枚举内
            onChange={(event) => setSchoolType(Number(event.target.value) as TenantApplicationSchoolType)}
            className="h-10 w-full border-b border-dark-line bg-transparent text-md text-on-dark transition-colors duration-fast hover:border-on-dark-faint focus-visible:border-accent focus-visible:outline-none"
          >
            {TENANT_APPLICATION_SCHOOL_TYPES.map((type) => (
              <option key={type} value={String(type)} className="bg-dark-surface text-on-dark">
                {tenantApplicationSchoolTypeLabel(type)}
              </option>
            ))}
          </select>
        </AuthField>

        <AuthTextField
          label="联系人姓名"
          id={`${fieldId}-contact`}
          autoComplete="name"
          value={contactName}
          error={errors.contactName}
          onValueChange={setContactName}
          onBlur={() => setError('contactName', requiredError(contactName, '请输入联系人姓名'))}
        />

        <AuthTextField
          label="联系电话"
          id={`${fieldId}-phone`}
          type="tel"
          autoComplete="tel"
          inputMode="numeric"
          value={contactPhone}
          error={errors.contactPhone}
          onValueChange={setContactPhone}
          onBlur={() => setError('contactPhone', phoneError(contactPhone, '联系电话'))}
        />

        <AuthTextField
          label="联系邮箱"
          id={`${fieldId}-email`}
          type="email"
          autoComplete="email"
          value={contactEmail}
          error={errors.contactEmail}
          onValueChange={setContactEmail}
          onBlur={() => setError('contactEmail', emailError(contactEmail))}
        />

        <AuthFormError message={formError} />

        <AuthSubmit loading={submitting}>提交申请</AuthSubmit>

        <AuthQuietLink to="/auth/login" icon={ArrowLeft}>
          返回登录
        </AuthQuietLink>
      </form>
    </AuthPanel>
  )
}
