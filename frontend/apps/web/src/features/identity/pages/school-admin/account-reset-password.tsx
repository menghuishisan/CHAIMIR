// 重置账号密码(账号管理页的行内动作)。
//
// 管理员重置密码后,可以要求本人下次登录必须改密 —— 默认要求,
// 因为管理员知道这个密码,继续用它等于账号被两个人掌握。

import { useCallback, useState } from 'react'
import type { Account } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Checkbox,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

export interface ResetPasswordModalProps {
  account: Account
  onClose: () => void
  onDone: () => void
}

/**
 * ResetPasswordModal 由管理员为账号设置新密码。
 */
export function ResetPasswordModal({ account, onClose, onDone }: ResetPasswordModalProps) {
  const [password, setPassword] = useState('')
  const [mustChange, setMustChange] = useState(true)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (password.length < 8 || !/[a-zA-Z]/.test(password) || !/\d/.test(password)) {
        setFormError('密码至少 8 位,且要同时包含字母和数字')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        await api.identity.resetAccountPassword(account.id, {
          new_password: password,
          must_change_pwd: mustChange,
        })
        toast.success('密码已重置,请把新密码交给本人')
        onDone()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '重置没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [account.id, mustChange, onDone, password],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>重置密码</ModalTitle>
          <ModalDescription>
            为 {account.name}
            {account.no ? ` · ${account.no}` : ''} 设置新密码。设置后原密码立即失效。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField
              label="新密码"
              htmlFor="reset-password"
              required
              error={formError}
              helper="至少 8 位,同时包含字母和数字"
            >
              <Input
                id="reset-password"
                type="password"
                value={password}
                autoComplete="new-password"
                invalid={Boolean(formError)}
                onChange={(event) => setPassword(event.target.value)}
              />
            </FormField>

            <Checkbox
              checked={mustChange}
              label="要求本人下次登录时修改密码"
              onCheckedChange={(checked) => setMustChange(checked === true)}
            />

            <Callout tone="warning">
              新密码请通过本人可信的渠道转交。你也知道这个密码,建议保持勾选上面的选项。
            </Callout>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={working}>
              重置密码
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}
