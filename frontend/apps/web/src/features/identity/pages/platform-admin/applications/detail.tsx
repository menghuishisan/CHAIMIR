// ApplicationDetailPage 展示平台入驻申请详情，并调用后端完成通过或驳回。

import React, { useCallback, useMemo, useState } from 'react'
import { ApplicationStatus } from '@chaimir/api-client'
import { Button, Callout, DescriptionList, Input, Textarea } from '@chaimir/ui'
import { CheckCircle2, FileCheck, RefreshCw, XCircle } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from './review.module.css'
import { formatDateTime, tenantApplicationSchoolTypeLabel, tenantApplicationStatusLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'



const ApplicationDetailPage: React.FC = () => {
  const { id } = useParams()
  const resource = useAsyncResource(() => api.identity.getApplications(), [])
  const application = useMemo(
    () => resource.data?.find((item) => item.application_id === id),
    [id, resource.data],
  )
  const [tenantCode, setTenantCode] = useState('')
  const [adminName, setAdminName] = useState('')
  const [adminPhone, setAdminPhone] = useState('')
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState<'approve' | 'reject' | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /**
   * handleApprove 调用通过审核接口，后端负责创建租户与初始激活凭据。
   */
  const handleApprove = useCallback(async () => {
    if (!id) {
      return
    }
    if (!tenantCode.trim() || !adminName.trim() || !adminPhone.trim()) {
      setError('请填写租户代号、管理员姓名和管理员手机号。')
      return
    }
    setSubmitting('approve')
    setError(null)
    setMessage(null)
    try {
      const result = await api.identity.approveApplication(id, {
        tenant_code: tenantCode.trim(),
        admin_name: adminName.trim(),
        admin_phone: adminPhone.trim(),
      })
      setMessage(`已通过申请，租户 ${result.tenant.name} 已开通。${result.activation_code ? `激活码 ${result.activation_code}` : ''}`)
      resource.reload()
    } catch (approveError) {
      setError(userFacingErrorMessage(approveError, '审核通过失败，请稍后重试。'))
    } finally {
      setSubmitting(null)
    }
  }, [adminName, adminPhone, id, resource, tenantCode])

  /**
   * handleReject 调用驳回接口，原因只展示用户向说明。
   */
  const handleReject = useCallback(async () => {
    if (!id) {
      return
    }
    if (!reason.trim()) {
      setError('请填写驳回原因。')
      return
    }
    setSubmitting('reject')
    setError(null)
    setMessage(null)
    try {
      await api.identity.rejectApplication(id, { reason: reason.trim() })
      setMessage('申请已驳回，申请方可根据说明重新提交。')
      resource.reload()
    } catch (rejectError) {
      setError(userFacingErrorMessage(rejectError, '驳回申请失败，请稍后重试。'))
    } finally {
      setSubmitting(null)
    }
  }, [id, reason, resource])

  if (resource.status === 'loading') {
    return <LoadingState title="正在获取申请详情" />
  }
  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }
  if (!application) {
    return <EmptyState title="未找到申请" description="当前申请不存在，或您没有查看权限。" />
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <FileCheck size={28} />
            入驻审核详情
          </h1>
          <p className={styles.subtitle}>核对学校入驻申请，并完成学校开通或驳回。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="审核结果">
          {message}
        </Callout>
      )}

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>申请信息</h2>
          <DescriptionList
            items={[
              { key: 'schoolName', label: '学校名称', value: application.school_name },
              { key: 'schoolType', label: '机构类型', value: tenantApplicationSchoolTypeLabel(application.school_type) },
              { key: 'contactName', label: '联系人', value: application.contact_name },
              { key: 'contactPhone', label: '联系电话', value: application.contact_phone },
              { key: 'contactEmail', label: '联系邮箱', value: application.contact_email },
              { key: 'status', label: '审核状态', value: tenantApplicationStatusLabel(application.status) },
              { key: 'submittedAt', label: '提交时间', value: formatDateTime(application.submitted_at) },
              { key: 'reviewedAt', label: '审核时间', value: formatDateTime(application.reviewed_at) },
              { key: 'rejectReason', label: '驳回原因', value: application.reject_reason || '暂无' },
            ]}
          />
        </section>

        <section className={styles.panel}>
          <h2>审核动作</h2>
          {application.status === ApplicationStatus.PENDING ? <div className={styles.form}>
            <label>
              租户代号
              <Input fullWidth value={tenantCode} placeholder="请输入开通后的租户代号" onChange={(event) => setTenantCode(event.target.value)} />
            </label>
            <label>
              初始管理员姓名
              <Input fullWidth value={adminName} placeholder="请输入管理员姓名" onChange={(event) => setAdminName(event.target.value)} />
            </label>
            <label>
              初始管理员手机号
              <Input fullWidth value={adminPhone} placeholder="请输入管理员手机号" onChange={(event) => setAdminPhone(event.target.value)} />
            </label>
            <Button loading={submitting === 'approve'} icon={<CheckCircle2 size={16} />} onClick={handleApprove}>
              通过并开通
            </Button>
            <label>
              驳回原因
              <Textarea value={reason} placeholder="请说明需要补充或修正的信息" onChange={(event) => setReason(event.target.value)} />
            </label>
            <Button variant="outline" loading={submitting === 'reject'} icon={<XCircle size={16} />} onClick={handleReject}>
              驳回申请
            </Button>
          </div> : <Callout variant="info" title="审核已完成">该申请已进入终态，无需再次处理。</Callout>}
        </section>
      </div>
    </div>
  )
}

export default ApplicationDetailPage
