// TenantStatusModal 调整一所学校的状态与服务到期日(设计 §7.6:学校管理与学校详情共用同一弹窗)。
// 状态三档由后端限定(正常 / 停用 / 已到期);停用与到期会立即让该校师生无法登录、已登录的人下一次操作时被退出。
// 到期日与状态一起提交(同一条接口),所以续期就是「改回正常 + 延长到期日」一次完成;
// 后端把到期日整体替换,留空即不设到期。

import { useState } from 'react'
import { CircleSlash, ShieldCheck } from 'lucide-react'
import { TenantStatus, type Tenant } from '@chaimir/api-client'
import {
  Button,
  Callout,
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
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { tenantStatusLabel } from '../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { TENANT_STATUSES } from '../options'

/** 每一档状态的后果说明 */
const STATUS_CONSEQUENCE: Record<TenantStatus, string> = {
  [TenantStatus.ACTIVE]: '师生可以照常登录与使用。停用期间的数据都还在。',
  [TenantStatus.DISABLED]:
    '会立即影响该校师生登录:全校无法登录,已登录的人下一次操作时会被退出。数据保留,恢复后可继续使用。',
  [TenantStatus.EXPIRED]:
    '会立即影响该校师生登录,效果与停用相同,原因记为服务期结束。续期时改回正常并延长到期日。',
}

/** localDay 把接口时间换成日期控件需要的本地 YYYY-MM-DD */
function localDay(value: string | undefined): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

export interface TenantStatusModalProps {
  tenant: Tenant
  onClose: () => void
  /** 保存成功,带回更新后的学校 */
  onSaved: (tenant: Tenant) => void
}

/** TenantStatusModal 选择状态与到期日,写明后果后保存 */
export function TenantStatusModal({ tenant, onClose, onSaved }: TenantStatusModalProps) {
  const [status, setStatus] = useState(String(tenant.status))
  const [expireDay, setExpireDay] = useState(localDay(tenant.expire_at))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const next = Number(status) as TenantStatus
  const blocking = next !== TenantStatus.ACTIVE
  const unchanged = next === tenant.status && expireDay === localDay(tenant.expire_at)
  // 正常状态配一个已过去的到期日,保存后依然登录不了 —— 先挡住
  const today = localDay(new Date().toISOString())
  const pastExpiry = next === TenantStatus.ACTIVE && expireDay !== '' && expireDay < today

  /** submit 保存状态与到期日;到期日取所选那天的 23:59:59(本地时间) */
  const submit = async () => {
    setFormError(undefined)
    setWorking(true)
    try {
      const saved = await api.identity.updateTenant(tenant.id, {
        status: next,
        expire_at: expireDay ? new Date(`${expireDay}T23:59:59`).toISOString() : undefined,
      })
      toast.success(`已更新「${tenant.display_name || tenant.name}」的状态`)
      onSaved(saved)
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '状态没有更新成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }

  return (
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="md">
        <ModalHeader>
          <ModalTitle>调整状态</ModalTitle>
          <ModalDescription>
            {tenant.display_name || tenant.name},当前{tenantStatusLabel(tenant.status)}。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField label="状态" required>
            <SegmentedControl
              aria-label="学校状态"
              options={TENANT_STATUSES.map((item) => ({
                value: String(item),
                label: tenantStatusLabel(item),
              }))}
              value={status}
              onValueChange={setStatus}
            />
          </FormField>
          <FormField
            label="服务到期日"
            htmlFor="tenant-expire"
            helper="留空表示长期有效。到了这一天的 23:59 之后,该校师生无法登录"
            error={
              pastExpiry
                ? '这个日期已经过去,保存后该校仍然无法登录;续期请选今天以后的日期'
                : undefined
            }
          >
            <Input
              id="tenant-expire"
              type="date"
              value={expireDay}
              invalid={pastExpiry}
              onChange={(event) => setExpireDay(event.target.value)}
              className="sm:w-48"
            />
          </FormField>
          <Callout tone={blocking ? 'warning' : 'info'}>{STATUS_CONSEQUENCE[next]}</Callout>
          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose} disabled={working}>
            取消
          </Button>
          <Button
            variant={blocking ? 'danger' : 'primary'}
            leftIcon={blocking ? CircleSlash : ShieldCheck}
            loading={working}
            disabled={unchanged || pastExpiry}
            onClick={() => void submit()}
          >
            {blocking ? `确认${tenantStatusLabel(next)}` : '保存'}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
