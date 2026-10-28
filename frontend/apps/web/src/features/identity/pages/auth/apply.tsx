// ApplyPage 学校入驻申请(公共页,底层全裸露形态;仅 SaaS 形态注册):
// 提交学校与联系人信息(POST platform/applications,免认证),由平台运营人员线下审核。
// 单页表单:当前公开契约没有服务端草稿接口,不拆成会丢失中间状态的前端向导(规范 §9)。
// 机构类型枚举与 tenantApplicationSchoolTypeLabel 同源,避免前后端两套文案。

import React, { useCallback, useId, useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import { Select } from '@chaimir/ui'
import { api } from '../../../../app/api'
import {
  tenantApplicationSchoolTypeLabel,
} from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { emailError, phoneError, requiredError } from '../../authForm'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import {
  TENANT_APPLICATION_SCHOOL_TYPES,
  type TenantApplicationSchoolType,
} from '../../options'
import {
  AuthField,
  AuthFormError,
  AuthHeading,
  AuthPanel,
  AuthQuietLink,
  AuthSubmit,
  AuthSuccess,
  AuthTextField,
} from '../../components/AuthUI'

const APPLICATION_DRAFT_KEY = 'chaimir.tenant-application.draft'

interface ApplicationDraft {
  schoolName: string
  schoolType: TenantApplicationSchoolType
  contactName: string
  contactPhone: string
  contactEmail: string
}

interface ApplicationDraftState {
  draft: ApplicationDraft
  warning: string | null
}

/** readApplicationDraft 只恢复匿名申请字段,损坏或不可读的本机草稿显式告知申请人。 */
function readApplicationDraft(): ApplicationDraftState {
  const empty: ApplicationDraft = {
    schoolName: '',
    schoolType: TENANT_APPLICATION_SCHOOL_TYPES[0],
    contactName: '',
    contactPhone: '',
    contactEmail: '',
  }
  let stored: string | null
  try {
    stored = window.localStorage.getItem(APPLICATION_DRAFT_KEY)
  } catch {
    return { draft: empty, warning: '无法读取此浏览器中的草稿,请保持页面打开并完成申请。' }
  }
  if (stored === null) return { draft: empty, warning: null }

  try {
    const value: unknown = JSON.parse(stored)
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
      throw new Error('Invalid application draft')
    }
    const fields = value as Record<string, unknown>
    if (
      typeof fields.schoolName !== 'string'
      || typeof fields.contactName !== 'string'
      || typeof fields.contactPhone !== 'string'
      || typeof fields.contactEmail !== 'string'
      || !TENANT_APPLICATION_SCHOOL_TYPES.some((type) => type === fields.schoolType)
    ) {
      throw new Error('Invalid application draft fields')
    }
    return {
      draft: {
        schoolName: fields.schoolName,
        schoolType: fields.schoolType as TenantApplicationSchoolType,
        contactName: fields.contactName,
        contactPhone: fields.contactPhone,
        contactEmail: fields.contactEmail,
      },
      warning: null,
    }
  } catch {
    return { draft: empty, warning: '草稿无法恢复,请重新填写申请信息。' }
  }
}

/**
 * ApplyPage 处理学校入驻申请提交。
 */
export default function ApplyPage() {
  const fieldId = useId()
  const [draftState, setDraftState] = useState(readApplicationDraft)
  const { schoolName, schoolType, contactName, contactPhone, contactEmail } = draftState.draft
  const { errors, validate } = useFormValidation(() => ({
    schoolName: requiredError(schoolName, '请输入学校名称'),
    contactName: requiredError(contactName, '请输入联系人姓名'),
    contactPhone: phoneError(contactPhone, '联系电话'),
    contactEmail: emailError(contactEmail),
  }))
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)

  /** updateDraft 在输入事件结束前保存字段白名单,刷新或离开时不丢尚未提交的申请。 */
  const updateDraft = useCallback((change: Partial<ApplicationDraft>) => {
    const draft = { ...draftState.draft, ...change }
    let warning: string | null = null
    try {
      window.localStorage.setItem(APPLICATION_DRAFT_KEY, JSON.stringify(draft))
    } catch {
      warning = '无法在此浏览器保存草稿,请保持页面打开并完成申请。'
    }
    setDraftState({ draft, warning })
  }, [draftState.draft])

  /** handleSubmit 提交入驻申请 */
  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return

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
        try {
          window.localStorage.removeItem(APPLICATION_DRAFT_KEY)
          setDraftState((previous) => ({ ...previous, warning: null }))
        } catch {
          setDraftState((previous) => ({
            ...previous,
            warning: '申请已提交,但此浏览器未能清除草稿。请清除本站浏览数据后再关闭页面。',
          }))
        }
        setDone(true)
      } catch (applyError) {
        setFormError(userFacingErrorMessage(applyError, '申请提交失败,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [contactEmail, contactName, contactPhone, schoolName, schoolType, validate],
  )

  if (done) {
    return (
      <AuthPanel>
        {/* 尾链是弱化通路:申请人此刻还没有账号,登录不是他的下一步(下一步是等平台联系) */}
        <AuthSuccess
          title="申请已提交"
          description="平台运营人员会通过您留下的电话或邮箱与您联系,请留意来电与邮件。"
        >
          <AuthFormError message={draftState.warning} />
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
          onValueChange={(value) => updateDraft({ schoolName: value })}
          onBlur={() => validate(['schoolName'])}
        />

        {/* 机构类型:认证页是墨色底层,用设计系统 Select 的深色变体(不在页面手拼深色控件) */}
        <AuthField label="机构类型" htmlFor={`${fieldId}-type`}>
          <Select
            onDark
            id={`${fieldId}-type`}
            value={String(schoolType)}
            options={TENANT_APPLICATION_SCHOOL_TYPES.map((type) => ({
              value: String(type),
              label: tenantApplicationSchoolTypeLabel(type),
            }))}
            // 选项只由 TENANT_APPLICATION_SCHOOL_TYPES 渲染,取值必然落在枚举内
            onValueChange={(value) => updateDraft({ schoolType: Number(value) as TenantApplicationSchoolType })}
          />
        </AuthField>

        <AuthTextField
          label="联系人姓名"
          id={`${fieldId}-contact`}
          autoComplete="name"
          value={contactName}
          error={errors.contactName}
          onValueChange={(value) => updateDraft({ contactName: value })}
          onBlur={() => validate(['contactName'])}
        />

        <AuthTextField
          label="联系电话"
          id={`${fieldId}-phone`}
          type="tel"
          autoComplete="tel"
          inputMode="numeric"
          value={contactPhone}
          error={errors.contactPhone}
          onValueChange={(value) => updateDraft({ contactPhone: value })}
          onBlur={() => validate(['contactPhone'])}
        />

        <AuthTextField
          label="联系邮箱"
          id={`${fieldId}-email`}
          type="email"
          autoComplete="email"
          value={contactEmail}
          error={errors.contactEmail}
          onValueChange={(value) => updateDraft({ contactEmail: value })}
          onBlur={() => validate(['contactEmail'])}
        />

        <AuthFormError message={draftState.warning} />
        <AuthFormError message={formError} />

        <AuthSubmit loading={submitting}>提交申请</AuthSubmit>

        <AuthQuietLink to="/auth/login" icon={ArrowLeft}>
          返回登录
        </AuthQuietLink>
      </form>
    </AuthPanel>
  )
}
