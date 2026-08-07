// 入驻审核详情页(平台深页,/platform-admin/applications/:applicationId)。
//
// 通过审核会在一个事务里创建学校租户、开通首个学校管理员并签发一次性激活码
// (后端 ApproveApplication),不可撤销;故开通信息在这里逐项填写并核对,不做列表内联审批。
//
// 学校编码与管理员手机号的格式要求与后端 ValidateTenantCode / ValidatePhone 一致,
// 在前端先挡住 —— 编码写错要等到提交才发现,而这次提交已经建了一半的东西再回滚。
//
// 后端只有列表接口(GET /platform/applications),没有单条读取;故本页从全量列表里定位。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { CircleCheck, CircleX, Inbox, ShieldCheck } from 'lucide-react'
import { ApplicationStatus, type TenantApplication } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  StatusIndicator,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  applicationStatusLabel,
  applicationStatusTone,
  schoolTypeLabel,
} from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 学校编码规则,与后端 ValidateTenantCode 的正则一致。 */
const TENANT_CODE_PATTERN = /^[a-z][a-z0-9-]{1,30}[a-z0-9]$/

/** 手机号规则,与后端 ValidatePhone 一致。 */
const PHONE_PATTERN = /^1[3-9]\d{9}$/

/**
 * PlatformApplicationDetailPage 核对一份入驻申请并做出开通或驳回决定。
 */
export default function PlatformApplicationDetailPage() {
  const { applicationId = '' } = useParams<{ applicationId: string }>()
  const navigate = useNavigate()

  const applications = useAsyncResource(() => api.identity.getApplications(), [], () => false)

  const application = useMemo(
    () => (applications.data ?? []).find((item) => item.application_id === applicationId),
    [applicationId, applications.data],
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '租户' },
              { label: '入驻申请', href: '/platform-admin/applications' },
              { label: application ? application.school_name : '审核详情' },
            ]}
          />
        }
        title={application ? application.school_name : '审核详情'}
        description="核对申请资料后决定是否开通。通过会立即创建学校与首个管理员账号,不能撤销。"
        icon={Inbox}
        actions={
          <Button variant="outline" onClick={() => navigate('/platform-admin/applications')}>
            返回申请列表
          </Button>
        }
      />

      <ResourceState
        resource={applications}
        emptyIcon={Inbox}
        emptyTitle="还没有入驻申请"
        emptyDescription="学校在公共入驻页提交申请后会出现在这里。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="block" />
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {() =>
          application ? (
            <ApplicationReview application={application} onReviewed={applications.reload} />
          ) : (
            <Callout tone="warning" title="没有找到这份申请">
              申请可能已被移除。回申请列表重新选择一份。
            </Callout>
          )
        }
      </ResourceState>
    </PageScaffold>
  )
}

interface ApplicationReviewProps {
  application: TenantApplication
  onReviewed: () => void
}

/**
 * ApplicationReview 渲染申请资料与审核动作。
 * 已处理的申请只读:通过后不能改判(租户已建),驳回后学校需要重新提交。
 */
function ApplicationReview({ application, onReviewed }: ApplicationReviewProps) {
  const navigate = useNavigate()
  const [action, setAction] = useState<'approve' | 'reject'>()
  const pending = application.status === ApplicationStatus.PENDING

  return (
    <>
      <PageSection title="申请资料" description="联系方式由学校自行填写,开通前请另行核实身份。">
        <Card>
          <CardHeader
            title={application.school_name}
            description={schoolTypeLabel(application.school_type)}
            actions={
              <StatusIndicator
                tone={applicationStatusTone(application.status)}
                label={applicationStatusLabel(application.status)}
              />
            }
          />
          <CardBody className="flex flex-col gap-4">
            <DescriptionList
              columns={2}
              items={[
                { term: '联系人', description: application.contact_name },
                { term: '联系电话', description: application.contact_phone, mono: true },
                { term: '联系邮箱', description: application.contact_email, mono: true },
                { term: '提交时间', description: formatDateTime(application.submitted_at), mono: true },
                ...(application.reviewed_at
                  ? [
                      {
                        term: '处理时间',
                        description: formatDateTime(application.reviewed_at),
                        mono: true,
                      },
                    ]
                  : []),
              ]}
            />

            {application.reject_reason ? (
              <Callout tone="warning" title="驳回理由">
                {application.reject_reason}
              </Callout>
            ) : null}

            {application.tenant_id ? (
              <div className="flex flex-wrap items-center gap-2 rounded-md border border-line bg-surface-sunken p-3">
                <Badge tone="success">已开通</Badge>
                <span className="text-sm text-ink-sub">这份申请对应的学校已经创建。</span>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => navigate(`/platform-admin/schools/${application.tenant_id}`)}
                >
                  去学校详情
                </Button>
              </div>
            ) : null}
          </CardBody>
        </Card>
      </PageSection>

      <PageSection
        title="审核决定"
        description={
          pending
            ? '通过需要指定学校编码与首个管理员;驳回需要写明理由,学校会看到这段话。'
            : '这份申请已经处理过,不能再改判。'
        }
      >
        {pending ? (
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="seal" leftIcon={CircleCheck} onClick={() => setAction('approve')}>
              通过并开通学校
            </Button>
            <Button variant="outline" leftIcon={CircleX} onClick={() => setAction('reject')}>
              驳回申请
            </Button>
          </div>
        ) : (
          <Callout tone="info">
            {application.status === ApplicationStatus.APPROVED
              ? '学校已开通。后续的停用、续期与配额调整在学校管理里做。'
              : '申请已驳回。学校可以按驳回理由修改后重新提交。'}
          </Callout>
        )}
      </PageSection>

      {action === 'approve' ? (
        <ApproveModal
          application={application}
          onClose={() => setAction(undefined)}
          onDone={() => {
            setAction(undefined)
            onReviewed()
          }}
        />
      ) : null}

      {action === 'reject' ? (
        <RejectModal
          application={application}
          onClose={() => setAction(undefined)}
          onDone={() => {
            setAction(undefined)
            onReviewed()
          }}
        />
      ) : null}
    </>
  )
}

interface ReviewModalProps {
  application: TenantApplication
  onClose: () => void
  onDone: () => void
}

/**
 * ApproveModal 填写开通信息并通过申请。
 * 激活码只在响应里出现一次,拿到后先展示给平台管理员转交学校,再关闭刷新。
 */
function ApproveModal({ application, onClose, onDone }: ReviewModalProps) {
  const [tenantCode, setTenantCode] = useState('')
  const [adminName, setAdminName] = useState(application.contact_name)
  const [adminPhone, setAdminPhone] = useState(application.contact_phone)
  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const [activation, setActivation] = useState<{ code?: string; tenantId: string }>()

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const next: Record<string, string | null> = {
        tenantCode: TENANT_CODE_PATTERN.test(tenantCode.trim())
          ? null
          : '用小写字母开头,只含小写字母、数字与连字符,长度 3 到 32 位',
        adminName: adminName.trim() === '' ? '请填写管理员姓名' : null,
        adminPhone: PHONE_PATTERN.test(adminPhone.trim()) ? null : '请填写 11 位手机号',
      }
      setErrors(next)
      if (Object.values(next).some((value) => value !== null)) return

      setFormError(undefined)
      setWorking(true)
      try {
        const result = await api.identity.approveApplication(application.application_id, {
          tenant_code: tenantCode.trim(),
          admin_name: adminName.trim(),
          admin_phone: adminPhone.trim(),
        })
        setActivation({ code: result.activation_code, tenantId: result.tenant.id })
        toast.success('学校已开通')
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '开通没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [adminName, adminPhone, application.application_id, tenantCode],
  )

  // 激活码展示态:关闭即刷新,不让管理员以为没开通成功
  if (activation) {
    return (
      <Modal open onOpenChange={() => onDone()}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>学校已开通</ModalTitle>
            <ModalDescription>
              把激活码交给这位管理员。本人用激活码设置密码后即可登录。关闭后无法再次查看。
            </ModalDescription>
          </ModalHeader>
          <ModalBody className="flex flex-col gap-4">
            <DescriptionList
              dense
              items={[
                { term: '学校', description: application.school_name },
                { term: '学校编码', description: tenantCode.trim(), mono: true },
                { term: '管理员', description: adminName.trim() },
                { term: '管理员手机号', description: adminPhone.trim(), mono: true },
                {
                  term: '激活码',
                  description: activation.code ?? '本次未签发,请在学校管理里为管理员重置密码',
                  mono: true,
                },
              ]}
            />
            <Callout tone="warning">
              激活码等同于首次登录凭据,请通过可信渠道转交,不要在群聊里公开。
            </Callout>
          </ModalBody>
          <ModalFooter>
            <Button variant="seal" onClick={() => onDone()}>
              我已记录,关闭
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    )
  }

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>通过并开通学校</ModalTitle>
          <ModalDescription>
            确认后会创建学校、开通首个管理员并签发激活码。这一步不能撤销,请核对信息。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <DescriptionList
              dense
              items={[
                { term: '学校名称', description: application.school_name },
                { term: '机构类型', description: schoolTypeLabel(application.school_type) },
                { term: '申请联系人', description: application.contact_name },
              ]}
            />

            <FormField
              label="学校编码"
              htmlFor="approve-code"
              required
              error={errors.tenantCode}
              helper="学校在平台内的唯一标识,开通后不能修改。例如 nanjing-tech"
            >
              <Input
                id="approve-code"
                value={tenantCode}
                placeholder="nanjing-tech"
                invalid={Boolean(errors.tenantCode)}
                onChange={(event) => setTenantCode(event.target.value)}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="首个管理员姓名"
                htmlFor="approve-admin"
                required
                error={errors.adminName}
                helper="默认取申请联系人,可以改"
              >
                <Input
                  id="approve-admin"
                  value={adminName}
                  invalid={Boolean(errors.adminName)}
                  onChange={(event) => setAdminName(event.target.value)}
                />
              </FormField>
              <FormField
                label="管理员手机号"
                htmlFor="approve-phone"
                required
                error={errors.adminPhone}
                helper="这个号码就是登录账号"
              >
                <Input
                  id="approve-phone"
                  value={adminPhone}
                  inputMode="numeric"
                  invalid={Boolean(errors.adminPhone)}
                  onChange={(event) => setAdminPhone(event.target.value)}
                />
              </FormField>
            </div>

            <Callout tone="warning" title="开通后立即可登录">
              这个手机号将成为该校第一个管理员账号,由他继续开通教师与学生。请确认身份无误。
            </Callout>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" leftIcon={ShieldCheck} loading={working}>
              确认开通
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/**
 * RejectModal 驳回申请并写明理由。
 */
function RejectModal({ application, onClose, onDone }: ReviewModalProps) {
  const [reason, setReason] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (reason.trim() === '') {
        setFormError('请写下驳回理由,学校需要知道该补什么')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        await api.identity.rejectApplication(application.application_id, { reason: reason.trim() })
        toast.success('申请已驳回')
        onDone()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '驳回没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [application.application_id, onDone, reason],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>驳回申请</ModalTitle>
          <ModalDescription>
            驳回后不创建学校。理由会留档,学校可以按理由修改后重新提交。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <DescriptionList
              dense
              items={[
                { term: '学校名称', description: application.school_name },
                { term: '联系人', description: application.contact_name },
              ]}
            />

            <FormField
              label="驳回理由"
              htmlFor="reject-reason"
              required
              error={formError}
              helper="写清缺什么或哪里不符合,便于学校补齐后再来"
            >
              <Textarea
                id="reject-reason"
                value={reason}
                rows={4}
                invalid={Boolean(formError)}
                onChange={(event) => setReason(event.target.value)}
              />
            </FormField>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="danger" loading={working}>
              确认驳回
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}
