// ApplyPage 提供学校入驻申请表单，字段与 identity 后端申请接口保持一致。

import React, { useCallback, useState } from 'react'
import type { ApiError } from '@chaimir/api-client'
import { Button, Callout, Input, Select } from '@chaimir/ui'
import { CheckCircle } from 'lucide-react'
import { api } from '../../../../app/api'
import styles from './public-auth.module.css'
import { tenantApplicationSchoolTypeOptions } from '../../../../utils/index'

const ApplyPage: React.FC = () => {
  const [schoolName, setSchoolName] = useState('')
  const [schoolType, setSchoolType] = useState('1')
  const [contactName, setContactName] = useState('')
  const [contactPhone, setContactPhone] = useState('')
  const [contactEmail, setContactEmail] = useState('')
  const [applicationId, setApplicationId] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  /**
   * handleSubmit 把入驻申请提交给后端，申请进度以后端记录为准。
   */
  const handleSubmit = useCallback(async () => {
    setError(null)
    if (!schoolName.trim() || !contactName.trim() || !contactPhone.trim() || !contactEmail.trim()) {
      setError('请填写机构名称、联系人、联系电话和邮箱。')
      return
    }

    setSubmitting(true)
    try {
      const application = await api.identity.createApplication({
        school_name: schoolName.trim(),
        school_type: Number(schoolType),
        contact_name: contactName.trim(),
        contact_phone: contactPhone.trim(),
        contact_email: contactEmail.trim(),
      })
      setApplicationId(application.application_id)
    } catch (applyError) {
      setError((applyError as ApiError).message || '申请提交失败，请检查信息后重试。')
    } finally {
      setSubmitting(false)
    }
  }, [contactEmail, contactName, contactPhone, schoolName, schoolType])

  return (
    <main className={styles.publicPage}>
      <section className={styles.publicCard} aria-label="学校入驻申请">
        <header className={styles.centerHeader}>
          <h1>申请入驻 Chaimir 平台</h1>
          <p>提交学校与联系人信息，平台审核通过后会下发租户开通信息。</p>
        </header>

        {error && <div className={styles.error}>{error}</div>}
        {applicationId ? (
          <div className={styles.successPanel}>
            <CheckCircle size={56} />
            <h2>申请已提交</h2>
            <p>申请编号 {applicationId}。审核结果会通过预留联系方式通知，请保持电话畅通。</p>
            <a href="/auth/login">返回登录页</a>
          </div>
        ) : (
          <>
            <Callout variant="info" title="申请材料说明">
              当前入驻申请接口接收学校基础信息和联系人信息；资质材料由平台审核人员按线下流程核验。
            </Callout>
            <div className={styles.formGrid}>
              <label className={styles.field}>
                机构名称
                <Input fullWidth value={schoolName} placeholder="请输入学校或机构名称" onChange={(event) => setSchoolName(event.target.value)} />
              </label>
              <label className={styles.field}>
                机构类型
                <Select fullWidth value={schoolType} options={tenantApplicationSchoolTypeOptions} onChange={setSchoolType} />
              </label>
              <label className={styles.field}>
                联系人姓名
                <Input fullWidth value={contactName} placeholder="请输入联系人姓名" onChange={(event) => setContactName(event.target.value)} />
              </label>
              <label className={styles.field}>
                联系电话
                <Input fullWidth value={contactPhone} placeholder="请输入联系人手机号" onChange={(event) => setContactPhone(event.target.value)} />
              </label>
              <label className={styles.fieldFull}>
                联系邮箱
                <Input fullWidth type="email" value={contactEmail} placeholder="请输入接收审核结果的邮箱" onChange={(event) => setContactEmail(event.target.value)} />
              </label>
            </div>
            <div className={styles.actions}>
              <a href="/auth/login">返回登录页</a>
              <Button loading={submitting} onClick={handleSubmit}>
                提交申请
              </Button>
            </div>
          </>
        )}
      </section>
    </main>
  )
}

export default ApplyPage
