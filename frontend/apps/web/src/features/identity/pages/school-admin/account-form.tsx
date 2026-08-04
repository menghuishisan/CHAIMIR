// 账号表单(账号管理页的流程页)。
//
// 新增与编辑共用一个表单,但后端字段不同:创建要手机号/学工号/基础身份/开通方式,
// 编辑只改姓名、归属组织、入学年份与职称(手机号由本人换绑,学工号与身份不可改)。
//
// 归属组织按基础身份分叉:教师挂院系、学生挂班级 —— 后端 validateAccountOrgForProfile
// 分别校验院系与班级存在性,学生还要求入学年份。故这里按身份切换选择器,不给一个笼统的「组织」。

import { useCallback, useId, useMemo, useState } from 'react'
import { Network } from 'lucide-react'
import { BaseIdentity, type Account, type Class, type Department } from '@chaimir/api-client'
import {
  Button,
  Callout,
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
  SegmentedControl,
  Select,
  Skeleton,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { baseIdentityLabel } from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 入学年份的合理范围:上限给到明年,便于提前建下一届。 */
const MIN_ENROLLMENT_YEAR = 1990

export interface AccountFormModalProps {
  /** 传入即为编辑模式;缺省为新建 */
  account?: Account
  onClose: () => void
  onSaved: () => void
}

/**
 * AccountFormModal 承载账号新增与编辑。
 */
export function AccountFormModal({ account, onClose, onSaved }: AccountFormModalProps) {
  const fieldId = useId()
  const editing = account !== undefined

  const [name, setName] = useState(account?.name ?? '')
  const [phone, setPhone] = useState('')
  const [no, setNo] = useState(account?.no ?? '')
  const [baseIdentity, setBaseIdentity] = useState(String(account?.base_identity ?? BaseIdentity.STUDENT))
  const [orgId, setOrgId] = useState('')
  const [enrollmentYear, setEnrollmentYear] = useState(String(new Date().getFullYear()))
  const [title, setTitle] = useState(account?.title ?? '')
  const [useActivation, setUseActivation] = useState(true)
  const [initialPassword, setInitialPassword] = useState('')

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)
  const [activationCode, setActivationCode] = useState<string>()

  const isTeacher = Number(baseIdentity) === BaseIdentity.TEACHER

  // 组织清单一次读齐:教师挂院系、学生挂班级,两份都要有才能在身份切换时立即可选
  const org = useAsyncResource(
    () =>
      Promise.all([api.identity.listDepartments(), api.identity.listClasses()]).then(
        ([departments, classes]) => ({ departments, classes }),
      ),
    [],
    () => false,
  )

  const orgOptions = useMemo(() => {
    if (!org.data) return []
    return isTeacher
      ? org.data.departments.map((item: Department) => ({ value: item.id, label: item.name }))
      : org.data.classes.map((item: Class) => ({ value: item.id, label: item.name }))
  }, [isTeacher, org.data])

  /** validate 按后端 CreateAccountByAdmin 的要求校验必填项。 */
  const validate = useCallback((): boolean => {
    const year = Number(enrollmentYear)
    const next: Record<string, string | null> = {
      name: name.trim() === '' ? '请输入姓名' : null,
      orgId: orgId === '' ? (isTeacher ? '请选择所属院系' : '请选择所属班级') : null,
    }

    if (!editing) {
      next.phone = /^1\d{10}$/.test(phone.trim()) ? null : '请输入 11 位手机号'
      next.no = no.trim() === '' ? '请输入学工号' : null
      if (!useActivation) {
        next.initialPassword = isStrongPassword(initialPassword)
          ? null
          : '初始密码至少 8 位,且要同时包含字母和数字'
      }
    }

    if (!isTeacher) {
      next.enrollmentYear =
        !Number.isInteger(year) || year < MIN_ENROLLMENT_YEAR || year > new Date().getFullYear() + 1
          ? '请输入有效的入学年份'
          : null
    }

    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [editing, enrollmentYear, initialPassword, isTeacher, name, no, orgId, phone, useActivation])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return

      setFormError(undefined)
      setSubmitting(true)
      try {
        if (editing) {
          await api.identity.updateAccount(account.id, {
            name: name.trim(),
            org_id: orgId,
            enrollment_year: isTeacher ? undefined : Number(enrollmentYear),
            title: title.trim() || undefined,
          })
          toast.success('账号资料已更新')
          onSaved()
          return
        }

        const created = await api.identity.createAccount({
          phone: phone.trim(),
          name: name.trim(),
          no: no.trim(),
          base_identity: Number(baseIdentity) as BaseIdentity,
          org_id: orgId,
          enrollment_year: isTeacher ? undefined : Number(enrollmentYear),
          title: title.trim() || undefined,
          initial_password: useActivation ? undefined : initialPassword,
          use_activation: useActivation,
        })
        // 激活码只在创建响应里出现一次,拿到就展示给管理员转交本人,不再二次可取
        if (created.activation_code) {
          setActivationCode(created.activation_code)
          toast.success('账号已开通,请把激活码交给本人')
          return
        }
        toast.success('账号已开通')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, editing ? '资料更新失败,请稍后重试。' : '账号开通失败,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [
      account?.id,
      baseIdentity,
      editing,
      enrollmentYear,
      initialPassword,
      isTeacher,
      name,
      no,
      onSaved,
      orgId,
      phone,
      title,
      useActivation,
      validate,
    ],
  )

  // 激活码展示态:关闭即刷新列表,不让管理员误以为没开通成功
  if (activationCode) {
    return (
      <Modal open onOpenChange={() => onSaved()}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>账号已开通</ModalTitle>
            <ModalDescription>
              把这个激活码交给本人。本人用激活码设置密码后即可登录。关闭后无法再次查看。
            </ModalDescription>
          </ModalHeader>
          <ModalBody className="flex flex-col gap-4">
            <DescriptionList
              dense
              items={[
                { term: '姓名', description: name.trim() },
                { term: '学工号', description: no.trim(), mono: true },
                { term: '激活码', description: activationCode, mono: true },
              ]}
            />
            <Callout tone="warning">
              激活码等同于首次登录凭据,请通过本人可信的渠道转交,不要在群聊里公开。
            </Callout>
          </ModalBody>
          <ModalFooter>
            <Button variant="seal" onClick={() => onSaved()}>
              我已记录,关闭
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    )
  }

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑账号资料' : '新增账号'}</ModalTitle>
          <ModalDescription>
            {editing
              ? '手机号由本人在个人中心换绑;学工号与身份创建后不可修改。'
              : '开通后本人用激活码或初始密码首次登录,首次登录会要求改密。'}
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            {!editing ? (
              <FormField label="身份" required helper="身份决定归属组织与可用功能,创建后不可修改">
                <SegmentedControl
                  aria-label="账号身份"
                  options={[
                    { value: String(BaseIdentity.TEACHER), label: baseIdentityLabel(BaseIdentity.TEACHER) },
                    { value: String(BaseIdentity.STUDENT), label: baseIdentityLabel(BaseIdentity.STUDENT) },
                  ]}
                  value={baseIdentity}
                  onValueChange={(value) => {
                    setBaseIdentity(value)
                    setOrgId('')
                  }}
                />
              </FormField>
            ) : null}

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="姓名" htmlFor={`${fieldId}-name`} required error={errors.name}>
                <Input
                  id={`${fieldId}-name`}
                  value={name}
                  invalid={Boolean(errors.name)}
                  onChange={(event) => setName(event.target.value)}
                />
              </FormField>
              <FormField
                label={isTeacher ? '职称' : '备注'}
                htmlFor={`${fieldId}-title`}
                helper={isTeacher ? '如 讲师、副教授' : '选填'}
              >
                <Input
                  id={`${fieldId}-title`}
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                />
              </FormField>
            </div>

            {!editing ? (
              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  label="手机号"
                  htmlFor={`${fieldId}-phone`}
                  required
                  error={errors.phone}
                  helper="用于登录与找回密码"
                >
                  <Input
                    id={`${fieldId}-phone`}
                    value={phone}
                    inputMode="numeric"
                    autoComplete="off"
                    invalid={Boolean(errors.phone)}
                    onChange={(event) => setPhone(event.target.value)}
                  />
                </FormField>
                <FormField
                  label={isTeacher ? '工号' : '学号'}
                  htmlFor={`${fieldId}-no`}
                  required
                  error={errors.no}
                  helper="校内唯一,创建后不可修改"
                >
                  <Input
                    id={`${fieldId}-no`}
                    value={no}
                    autoComplete="off"
                    invalid={Boolean(errors.no)}
                    onChange={(event) => setNo(event.target.value)}
                  />
                </FormField>
              </div>
            ) : (
              <DescriptionList
                dense
                columns={2}
                items={[
                  { term: '学工号', description: account.no ?? '未设置', mono: true },
                  { term: '手机号', description: account.phone_masked ?? '未绑定', mono: true },
                ]}
              />
            )}

            <ResourceState
              resource={org}
              emptyIcon={Network}
              emptyTitle="还没有组织架构"
              emptyDescription="先在组织架构里建立院系、专业与班级,再开通账号。"
              skeleton={<Skeleton variant="line" lines={2} />}
            >
              {() => (
                <div className="grid gap-4 sm:grid-cols-2">
                  <FormField
                    label={isTeacher ? '所属院系' : '所属班级'}
                    htmlFor={`${fieldId}-org`}
                    required
                    error={errors.orgId}
                  >
                    <Select
                      id={`${fieldId}-org`}
                      options={orgOptions}
                      value={orgId}
                      placeholder={
                        orgOptions.length > 0
                          ? isTeacher
                            ? '选择院系'
                            : '选择班级'
                          : isTeacher
                            ? '暂无院系'
                            : '暂无班级'
                      }
                      disabled={orgOptions.length === 0}
                      onValueChange={setOrgId}
                    />
                  </FormField>
                  {isTeacher ? null : (
                    <FormField
                      label="入学年份"
                      htmlFor={`${fieldId}-year`}
                      required
                      error={errors.enrollmentYear}
                      helper="用于升届与归档"
                    >
                      <Input
                        id={`${fieldId}-year`}
                        type="number"
                        min={MIN_ENROLLMENT_YEAR}
                        value={enrollmentYear}
                        invalid={Boolean(errors.enrollmentYear)}
                        onChange={(event) => setEnrollmentYear(event.target.value)}
                      />
                    </FormField>
                  )}
                </div>
              )}
            </ResourceState>

            {!editing ? (
              <div className="flex flex-col gap-3 rounded-md border border-line bg-surface-sunken p-4">
                <Checkbox
                  checked={useActivation}
                  label="用激活码开通(本人自行设置密码)"
                  onCheckedChange={(checked) => setUseActivation(checked === true)}
                />
                {useActivation ? (
                  <p className="text-sm text-ink-sub">
                    开通后会生成一次性激活码。学校未开启激活码开通时这个方式不可用,请改用初始密码。
                  </p>
                ) : (
                  <FormField
                    label="初始密码"
                    htmlFor={`${fieldId}-password`}
                    required
                    error={errors.initialPassword}
                    helper="至少 8 位,同时包含字母和数字。本人首次登录时会被要求修改"
                  >
                    <Input
                      id={`${fieldId}-password`}
                      type="password"
                      value={initialPassword}
                      autoComplete="new-password"
                      invalid={Boolean(errors.initialPassword)}
                      onChange={(event) => setInitialPassword(event.target.value)}
                    />
                  </FormField>
                )}
              </div>
            ) : null}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={submitting}>
              {editing ? '保存资料' : '开通账号'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** isStrongPassword 与后端 ValidatePassword 同一口径:至少 8 位且含字母与数字。 */
function isStrongPassword(password: string): boolean {
  return password.length >= 8 && /[a-zA-Z]/.test(password) && /\d/.test(password)
}
