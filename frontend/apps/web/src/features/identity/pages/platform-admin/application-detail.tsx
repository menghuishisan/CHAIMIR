// 申请详情(平台深页,/platform-admin/applications/:applicationId)。设计 §7.6:
// 页头为学校名 + 状态;待审核时右侧主按钮「通过并开通学校」、次要「驳回申请」。
// 审核风险提示和历史驳回理由放在申请信息上方(有则显示);主体为申请信息(学校、联系人、电话、邮箱、提交时间)。
// 通过会在一个事务里创建学校、开通首个管理员并签发一次性激活码,不可撤销:开通信息在弹窗里逐项填写;
// 开通结果弹窗只出现一次,要勾选「我已记录」才能关闭,并给「去配置配额」。

import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { CircleCheck, CircleX, Inbox, ShieldCheck } from 'lucide-react'
import { ApplicationStatus, type Tenant, type TenantApplication } from '@chaimir/api-client'
import {
  Button,
  buttonVariants,
  Callout,
  Card,
  CardBody,
  Checkbox,
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
  Skeleton,
  StatusIndicator,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { locationLabel, useAsyncResource, usePageLocation } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { applicationStatusLabel, schoolTypeLabel } from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { isPhoneNumber } from '../../authForm'
import { applicationStatusTone } from '../../statusPresentation'

/** 学校短码规则,与后端 ValidateTenantCode 一致 */
const TENANT_CODE_PATTERN = /^[a-z][a-z0-9-]{1,30}[a-z0-9]$/

/** PlatformApplicationDetailPage 核对一份入驻申请,决定开通或驳回 */
export default function PlatformApplicationDetailPage() {
  const { applicationId = '' } = useParams<{ applicationId: string }>()
  const application = useAsyncResource(
    () => api.identity.getApplication(applicationId),
    [applicationId],
    () => false
  )

  usePageLocation([{ label: locationLabel(application, (data) => data.school_name, '申请详情') }])

  return (
    <PageScaffold>
      <ResourceState
        resource={application}
        emptyIcon={Inbox}
        emptyTitle="没有找到这份申请"
        emptyDescription="申请可能已被移除,回入驻申请重新选择。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="title" width="half" />
            <Skeleton variant="block" />
          </div>
        }
      >
        {(data) => <ApplicationReview application={data} onReviewed={application.reload} />}
      </ResourceState>
    </PageScaffold>
  )
}

interface ApplicationReviewProps {
  application: TenantApplication
  onReviewed: () => void
}

/** ApplicationReview 申请信息与审核动作;已处理的申请只读 */
function ApplicationReview({ application, onReviewed }: ApplicationReviewProps) {
  const [action, setAction] = useState<'approve' | 'reject'>()
  const pending = application.status === ApplicationStatus.PENDING

  return (
    <>
      <PageHeader
        title={application.school_name}
        status={
          <StatusIndicator
            tone={applicationStatusTone(application.status)}
            label={applicationStatusLabel(application.status)}
          />
        }
        meta={[
          { value: schoolTypeLabel(application.school_type) },
          {
            label: '提交于',
            value: (
              <span title={formatDateTime(application.submitted_at)}>
                {formatRelativeOccurrence(application.submitted_at)}
              </span>
            ),
          },
          ...(application.reviewed_at
            ? [{ label: '处理于', value: formatDateTime(application.reviewed_at) }]
            : []),
        ]}
        actions={
          pending ? (
            <>
              <Button variant="outline" leftIcon={CircleX} onClick={() => setAction('reject')}>
                驳回申请
              </Button>
              <Button variant="primary" leftIcon={CircleCheck} onClick={() => setAction('approve')}>
                通过并开通学校
              </Button>
            </>
          ) : application.tenant_id ? (
            <Link
              to={`/platform-admin/schools/${application.tenant_id}`}
              className={buttonVariants({ variant: 'outline' })}
            >
              查看已开通的学校
            </Link>
          ) : null
        }
      />

      <div className="flex max-w-3xl flex-col gap-4">
        {pending ? (
          <Callout tone="warning" title="开通前请核实身份">
            申请里的联系方式由学校自己填写。通过后会立刻创建学校和一个能登录的管理员账号,且不能撤销,请先通过其他渠道确认联系人身份。
          </Callout>
        ) : null}
        {application.reject_reason ? (
          <Callout tone="info" title="驳回理由">
            <span className="whitespace-pre-wrap">{application.reject_reason}</span>
          </Callout>
        ) : null}

        <Card>
          <CardBody>
            <DescriptionList
              columns={2}
              items={[
                { term: '学校名称', description: application.school_name },
                { term: '机构类型', description: schoolTypeLabel(application.school_type) },
                { term: '联系人', description: application.contact_name },
                { term: '联系电话', description: application.contact_phone, mono: true },
                { term: '联系邮箱', description: application.contact_email, mono: true },
                {
                  term: '提交时间',
                  description: formatDateTime(application.submitted_at),
                  mono: true,
                },
              ]}
            />
          </CardBody>
        </Card>
      </div>

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
 * ApproveModal 填写开通信息并通过申请;成功后换成一次性的开通结果。
 * 学校短码与手机号的规则与后端一致,先在这里挡住 —— 提交后才发现写错,学校已经建了一半。
 */
function ApproveModal({ application, onClose, onDone }: ReviewModalProps) {
  const [tenantCode, setTenantCode] = useState('')
  const [adminName, setAdminName] = useState(application.contact_name)
  const [adminPhone, setAdminPhone] = useState(application.contact_phone)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const [result, setResult] = useState<{ tenant: Tenant; code?: string }>()

  const { errors, validate } = useFormValidation(() => ({
    tenantCode: TENANT_CODE_PATTERN.test(tenantCode.trim())
      ? null
      : '用小写字母开头,只含小写字母、数字和连字符,3 到 32 位',
    adminName: adminName.trim() === '' ? '请填写管理员姓名' : null,
    adminPhone: isPhoneNumber(adminPhone.trim()) ? null : '请填写 11 位手机号',
  }))

  /** submit 通过申请:建学校、开首个管理员、签发激活码 */
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!validate()) return
    setFormError(undefined)
    setWorking(true)
    try {
      const approved = await api.identity.approveApplication(application.application_id, {
        tenant_code: tenantCode.trim(),
        admin_name: adminName.trim(),
        admin_phone: adminPhone.trim(),
      })
      setResult({ tenant: approved.tenant, code: approved.activation_code })
      toast.success('学校已开通')
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '开通没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }

  if (result) {
    return (
      <ApprovedResult
        application={application}
        tenant={result.tenant}
        adminName={adminName.trim()}
        adminPhone={adminPhone.trim()}
        activationCode={result.code}
        onDone={onDone}
      />
    )
  }

  return (
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>通过并开通学校</ModalTitle>
          <ModalDescription>
            将创建「{application.school_name}」并开通首个学校管理员。开通后不能撤销。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField
              label="学校短码"
              htmlFor="approve-code"
              required
              error={errors.tenantCode}
              helper="学校在平台里的唯一短码,开通后不能修改,例如 nanjing-tech"
            >
              <Input
                id="approve-code"
                value={tenantCode}
                placeholder="nanjing-tech"
                autoComplete="off"
                invalid={Boolean(errors.tenantCode)}
                onBlur={() => validate(['tenantCode'])}
                onChange={(event) => setTenantCode(event.target.value)}
              />
            </FormField>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="首个管理员姓名"
                htmlFor="approve-admin"
                required
                error={errors.adminName}
                helper="默认是申请联系人,可以改"
              >
                <Input
                  id="approve-admin"
                  value={adminName}
                  invalid={Boolean(errors.adminName)}
                  onBlur={() => validate(['adminName'])}
                  onChange={(event) => setAdminName(event.target.value)}
                />
              </FormField>
              <FormField
                label="管理员手机号"
                htmlFor="approve-phone"
                required
                error={errors.adminPhone}
                helper="这个号码就是管理员的登录账号"
              >
                <Input
                  id="approve-phone"
                  value={adminPhone}
                  inputMode="numeric"
                  invalid={Boolean(errors.adminPhone)}
                  onBlur={() => validate(['adminPhone'])}
                  onChange={(event) => setAdminPhone(event.target.value)}
                />
              </FormField>
            </div>
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" disabled={working} onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" leftIcon={ShieldCheck} loading={working}>
              确认开通
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface ApprovedResultProps {
  application: TenantApplication
  tenant: Tenant
  adminName: string
  adminPhone: string
  activationCode?: string
  onDone: () => void
}

/**
 * ApprovedResult 开通结果:激活码只在这一次出现,关闭后无法再看。
 * 必须勾选「我已记录」才能关闭(Esc 与点遮罩也不关),避免误关后激活码丢失。
 */
function ApprovedResult({
  application,
  tenant,
  adminName,
  adminPhone,
  activationCode,
  onDone,
}: ApprovedResultProps) {
  const navigate = useNavigate()
  const [recorded, setRecorded] = useState(false)

  return (
    <Modal open onOpenChange={(open) => !open && recorded && onDone()}>
      <ModalContent
        size="md"
        onEscapeKeyDown={(event) => !recorded && event.preventDefault()}
        onInteractOutside={(event) => !recorded && event.preventDefault()}
      >
        <ModalHeader>
          <ModalTitle>「{application.school_name}」已开通</ModalTitle>
          <ModalDescription>
            把激活码交给这位管理员,本人用激活码设置密码后即可登录。关闭后无法再次查看。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            items={[
              { term: '学校短码', description: tenant.code, mono: true },
              { term: '管理员', description: adminName },
              { term: '登录手机号', description: adminPhone, mono: true },
              {
                term: '激活码',
                description: activationCode ? (
                  <span className="font-mono text-base font-semibold text-ink">
                    {activationCode}
                  </span>
                ) : (
                  '这次没有签发激活码,请让管理员用手机号找回密码后登录'
                ),
              },
            ]}
          />
          <Callout tone="warning">
            激活码等同于首次登录凭据,请通过可信渠道转交,不要发在群聊里。
          </Callout>
          <Checkbox
            checked={recorded}
            onCheckedChange={(checked) => setRecorded(checked === true)}
            label="我已记录激活码和管理员信息"
          />
        </ModalBody>
        <ModalFooter>
          <Button
            variant="outline"
            disabled={!recorded}
            onClick={() => navigate(`/platform-admin/schools/${tenant.id}`)}
          >
            去配置配额
          </Button>
          <Button variant="primary" disabled={!recorded} onClick={onDone}>
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/** RejectModal 驳回申请并写明理由;理由会留档,学校按理由补齐后可以重新提交 */
function RejectModal({ application, onClose, onDone }: ReviewModalProps) {
  const [reason, setReason] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const { errors, validate } = useFormValidation(() => ({
    reason: reason.trim() === '' ? '请写下驳回理由,学校需要知道该补什么' : null,
  }))

  /** submit 驳回申请 */
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!validate()) return
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
  }

  return (
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="md">
        <ModalHeader>
          <ModalTitle>驳回申请</ModalTitle>
          <ModalDescription>
            驳回后不会创建学校。理由会留档,学校可以按理由修改后重新提交。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField
              label="驳回理由"
              htmlFor="reject-reason"
              required
              error={errors.reason}
              helper="写清缺什么或哪里不符合要求"
            >
              <Textarea
                id="reject-reason"
                value={reason}
                rows={4}
                invalid={Boolean(errors.reason)}
                onBlur={() => validate(['reason'])}
                onChange={(event) => setReason(event.target.value)}
              />
            </FormField>
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" disabled={working} onClick={onClose}>
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
